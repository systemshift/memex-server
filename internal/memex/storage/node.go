package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"memex/internal/memex/core"
)

// NodeData represents serialized node data
type NodeData struct {
	ID       [32]byte
	Type     [32]byte
	Created  int64 // Unix timestamp
	Modified int64 // Unix timestamp
	MetaLen  uint32
	Meta     []byte // JSON-encoded metadata
}

const (
	maxMetaLen = 1024 * 1024 // 1MB max metadata size
)

// AddNode adds a node to the store
func (s *MXStore) AddNode(content []byte, nodeType string, meta map[string]any) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Split content into chunks
	chunks, err := ChunkContent(content)
	if err != nil {
		return "", fmt.Errorf("chunking content: %w", err)
	}

	// Store chunks and collect hashes
	var chunkHashes []string
	for _, chunk := range chunks {
		// Store chunk in chunk store
		hash, err := s.chunks.Store(chunk.Content)
		if err != nil {
			return "", fmt.Errorf("storing chunk: %w", err)
		}
		chunkHashes = append(chunkHashes, hash)
	}

	// Calculate full content hash
	hash := sha256.Sum256(content)
	contentHash := hex.EncodeToString(hash[:])

	// Add content info to metadata
	if meta == nil {
		meta = make(map[string]any)
	}
	meta["content"] = contentHash
	meta["chunks"] = chunkHashes

	// Generate node ID
	id := generateID()

	// Serialize metadata
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("serializing metadata: %w", err)
	}

	if len(metaJSON) > maxMetaLen {
		return "", fmt.Errorf("metadata too large: %d > %d bytes", len(metaJSON), maxMetaLen)
	}

	// Create node data
	now := time.Now().Unix()
	var nodeData NodeData
	copy(nodeData.ID[:], id)

	// Ensure nodeType fits in fixed buffer
	if len(nodeType) > 32 {
		nodeType = nodeType[:32]
	}
	copy(nodeData.Type[:], []byte(nodeType))

	nodeData.Created = now
	nodeData.Modified = now
	nodeData.MetaLen = uint32(len(metaJSON))
	nodeData.Meta = metaJSON

	// Begin transaction
	tx, err := s.beginTransaction()
	if err != nil {
		return "", fmt.Errorf("beginning transaction: %w", err)
	}

	// Write node data
	offset, err := s.writeNode(nodeData)
	if err != nil {
		tx.rollback()
		return "", fmt.Errorf("writing node: %w", err)
	}

	// Add to index
	var idBytes [32]byte
	copy(idBytes[:], id)
	entry := IndexEntry{
		ID:     idBytes,
		Offset: offset,
		Length: uint32(80 + len(metaJSON)), // Fixed size + metadata
	}
	tx.addIndex(entry)

	nodeIDStr := hex.EncodeToString(idBytes[:])

	// Commit transaction
	if err := tx.commit(); err != nil {
		return "", fmt.Errorf("committing transaction: %w", err)
	}

	// Create similarity links after transaction is committed
	go s.createSimilarityLinks(nodeIDStr, chunkHashes)

	return nodeIDStr, nil
}

// GetNode retrieves a node by ID
func (s *MXStore) GetNode(id string) (core.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Convert ID to bytes
	idBytes, err := hex.DecodeString(id)
	if err != nil {
		return core.Node{}, fmt.Errorf("invalid ID: %w", err)
	}

	// Find node in index
	var entry IndexEntry
	for _, e := range s.nodes {
		if bytes.HasPrefix(e.ID[:], idBytes) {
			entry = e
			break
		}
	}
	if entry.ID == [32]byte{} {
		return core.Node{}, fmt.Errorf("node not found")
	}

	// Seek to node data
	if _, err := s.seek(int64(entry.Offset), io.SeekStart); err != nil {
		return core.Node{}, fmt.Errorf("seeking to node: %w", err)
	}

	// Read node data
	var nodeData NodeData

	// Read ID
	if _, err := io.ReadFull(s.file, nodeData.ID[:]); err != nil {
		return core.Node{}, fmt.Errorf("reading ID: %w", err)
	}

	// Read Type
	if _, err := io.ReadFull(s.file, nodeData.Type[:]); err != nil {
		return core.Node{}, fmt.Errorf("reading type: %w", err)
	}
	nodeType := string(bytes.TrimRight(nodeData.Type[:], "\x00"))

	// Read timestamps
	if err := binary.Read(s.file, binary.LittleEndian, &nodeData.Created); err != nil {
		return core.Node{}, fmt.Errorf("reading created time: %w", err)
	}
	if err := binary.Read(s.file, binary.LittleEndian, &nodeData.Modified); err != nil {
		return core.Node{}, fmt.Errorf("reading modified time: %w", err)
	}

	// Read metadata length
	if err := binary.Read(s.file, binary.LittleEndian, &nodeData.MetaLen); err != nil {
		return core.Node{}, fmt.Errorf("reading metadata length: %w", err)
	}

	// Validate metadata length
	if nodeData.MetaLen > maxMetaLen {
		return core.Node{}, fmt.Errorf("invalid metadata length: %d", nodeData.MetaLen)
	}

	// Read metadata
	nodeData.Meta = make([]byte, nodeData.MetaLen)
	if _, err := io.ReadFull(s.file, nodeData.Meta); err != nil {
		return core.Node{}, fmt.Errorf("reading metadata: %w", err)
	}

	// Validate metadata is valid JSON
	if !json.Valid(nodeData.Meta) {
		return core.Node{}, fmt.Errorf("invalid metadata JSON")
	}

	// Parse metadata
	var meta map[string]any
	if err := json.Unmarshal(nodeData.Meta, &meta); err != nil {
		return core.Node{}, fmt.Errorf("parsing metadata: %w", err)
	}

	// Convert chunks to []string
	if chunksRaw, ok := meta["chunks"]; ok {
		var chunkList []string
		switch chunks := chunksRaw.(type) {
		case []interface{}:
			for _, chunk := range chunks {
				if chunkStr, ok := chunk.(string); ok {
					chunkStr = strings.Trim(chunkStr, `"`)
					chunkList = append(chunkList, chunkStr)
				}
			}
			if len(chunkList) > 0 {
				meta["chunks"] = chunkList
			}
		case []string:
			for i, chunk := range chunks {
				chunks[i] = strings.Trim(chunk, `"`)
			}
			chunkList = chunks
			meta["chunks"] = chunkList
		case string:
			chunks = strings.Trim(chunks, `"`)
			chunkList = strings.Fields(chunks)
			if len(chunkList) > 0 {
				meta["chunks"] = chunkList
			}
		}
	}

	// Load content if available
	var versions []core.Version
	if contentHash, ok := meta["content"].(string); ok {
		// Get chunks from metadata
		var chunkList []string
		if chunks, ok := meta["chunks"].([]string); ok {
			chunkList = chunks
		} else {
			chunkList = []string{contentHash} // Fallback for old nodes
		}

		// Create version for content
		version := core.Version{
			Hash:      contentHash,
			Chunks:    chunkList,
			Created:   time.Unix(nodeData.Created, 0),
			Meta:      make(map[string]any),
			Available: true,
		}
		versions = append(versions, version)
	}

	// Convert to core.Node
	node := core.Node{
		ID:       hex.EncodeToString(nodeData.ID[:]),
		Type:     nodeType,
		Meta:     meta,
		Created:  time.Unix(nodeData.Created, 0),
		Modified: time.Unix(nodeData.Modified, 0),
		Versions: versions,
	}
	if len(versions) > 0 {
		node.Current = versions[0].Hash
	}

	return node, nil
}

// DeleteNode removes a node
func (s *MXStore) DeleteNode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Convert ID to bytes
	idBytes, err := hex.DecodeString(id)
	if err != nil {
		return fmt.Errorf("invalid ID: %w", err)
	}

	// Find node in index
	var nodeIndex int
	var entry IndexEntry
	for i, e := range s.nodes {
		if bytes.HasPrefix(e.ID[:], idBytes) {
			entry = e
			nodeIndex = i
			break
		}
	}
	if entry.ID == [32]byte{} {
		return fmt.Errorf("node not found")
	}

	// Begin transaction
	tx, err := s.beginTransaction()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	// Read node data
	if _, err := s.seek(int64(entry.Offset), io.SeekStart); err != nil {
		tx.rollback()
		return fmt.Errorf("seeking to node: %w", err)
	}

	var nodeData NodeData
	// Read ID
	if _, err := io.ReadFull(s.file, nodeData.ID[:]); err != nil {
		tx.rollback()
		return fmt.Errorf("reading ID: %w", err)
	}

	// Read Type
	if _, err := io.ReadFull(s.file, nodeData.Type[:]); err != nil {
		tx.rollback()
		return fmt.Errorf("reading type: %w", err)
	}

	// Read timestamps
	if err := binary.Read(s.file, binary.LittleEndian, &nodeData.Created); err != nil {
		tx.rollback()
		return fmt.Errorf("reading created time: %w", err)
	}
	if err := binary.Read(s.file, binary.LittleEndian, &nodeData.Modified); err != nil {
		tx.rollback()
		return fmt.Errorf("reading modified time: %w", err)
	}

	// Read metadata length
	if err := binary.Read(s.file, binary.LittleEndian, &nodeData.MetaLen); err != nil {
		tx.rollback()
		return fmt.Errorf("reading metadata length: %w", err)
	}

	// Validate metadata length
	if nodeData.MetaLen > maxMetaLen {
		tx.rollback()
		return fmt.Errorf("invalid metadata length: %d", nodeData.MetaLen)
	}

	// Read metadata
	nodeData.Meta = make([]byte, nodeData.MetaLen)
	if _, err := io.ReadFull(s.file, nodeData.Meta); err != nil {
		tx.rollback()
		return fmt.Errorf("reading metadata: %w", err)
	}

	// Parse metadata to get chunks
	var meta map[string]any
	if err := json.Unmarshal(nodeData.Meta, &meta); err != nil {
		tx.rollback()
		return fmt.Errorf("parsing metadata: %w", err)
	}

	// Remove chunks if available
	if chunksRaw, ok := meta["chunks"]; ok {
		var chunkList []string
		switch chunks := chunksRaw.(type) {
		case []interface{}:
			for _, chunk := range chunks {
				if chunkStr, ok := chunk.(string); ok {
					chunkStr = strings.Trim(chunkStr, `"`)
					chunkList = append(chunkList, chunkStr)
				}
			}
		case []string:
			for i, chunk := range chunks {
				chunks[i] = strings.Trim(chunk, `"`)
			}
			chunkList = chunks
		case string:
			chunks = strings.Trim(chunks, `"`)
			chunkList = strings.Fields(chunks)
		}

		for _, chunk := range chunkList {
			if err := s.chunks.Delete(chunk); err != nil {
				tx.rollback()
				return fmt.Errorf("deleting chunk: %w", err)
			}
		}
	}

	// Remove any edges connected to this node
	var newEdges []IndexEntry
	for _, e := range s.edges {
		// Read edge data to check if it's connected to this node
		if _, err := s.seek(int64(e.Offset), io.SeekStart); err != nil {
			tx.rollback()
			return fmt.Errorf("seeking to edge: %w", err)
		}

		var edgeData EdgeData
		// Read Source
		if _, err := io.ReadFull(s.file, edgeData.Source[:]); err != nil {
			tx.rollback()
			return fmt.Errorf("reading edge source: %w", err)
		}
		// Read Target
		if _, err := io.ReadFull(s.file, edgeData.Target[:]); err != nil {
			tx.rollback()
			return fmt.Errorf("reading edge target: %w", err)
		}

		// Keep edge if it's not connected to the node being deleted
		if !bytes.Equal(edgeData.Source[:], nodeData.ID[:]) && !bytes.Equal(edgeData.Target[:], nodeData.ID[:]) {
			newEdges = append(newEdges, e)
		}
	}
	s.edges = newEdges
	s.header.EdgeCount = uint32(len(s.edges))

	// Remove node from index
	s.nodes = append(s.nodes[:nodeIndex], s.nodes[nodeIndex+1:]...)
	s.header.NodeCount--
	s.header.Modified = time.Now()

	// Commit transaction
	if err := tx.commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// createSimilarityLinks creates links between nodes that share chunks
func (s *MXStore) createSimilarityLinks(nodeID string, chunks []string) error {
	// Get all nodes
	for _, entry := range s.nodes {
		otherID := hex.EncodeToString(entry.ID[:])
		if otherID == nodeID {
			continue // Skip self
		}

		// Get other node's chunks
		node, err := s.GetNode(otherID)
		if err != nil {
			continue
		}

		// Get chunks from metadata
		var otherChunks []string
		if chunksRaw, ok := node.Meta["chunks"]; ok {
			switch chunks := chunksRaw.(type) {
			case []interface{}:
				for _, chunk := range chunks {
					if chunkStr, ok := chunk.(string); ok {
						chunkStr = strings.Trim(chunkStr, `"`)
						otherChunks = append(otherChunks, chunkStr)
					}
				}
			case []string:
				for i, chunk := range chunks {
					chunks[i] = strings.Trim(chunk, `"`)
				}
				otherChunks = chunks
			case string:
				chunks = strings.Trim(chunks, `"`)
				otherChunks = strings.Fields(chunks)
			}
		}

		if len(otherChunks) == 0 {
			continue
		}

		// Calculate similarity
		sharedChunks := 0
		for _, chunk := range chunks {
			for _, otherChunk := range otherChunks {
				if chunk == otherChunk {
					sharedChunks++
					break
				}
			}
		}

		// Create link if similarity threshold met
		if sharedChunks > 0 {
			similarity := float64(sharedChunks) / float64(len(chunks))
			if similarity >= 0.3 { // At least 30% similar
				meta := map[string]any{
					"similarity": similarity,
					"shared":     sharedChunks,
				}
				if err := s.AddLink(nodeID, otherID, "similar", meta); err != nil {
					return fmt.Errorf("adding similarity link: %w", err)
				}
			}
		}
	}

	return nil
}

// generateID generates a unique ID
func generateID() []byte {
	id := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return id[:]
}

// writeNode writes node data to the file
func (s *MXStore) writeNode(node NodeData) (uint64, error) {
	// Get current file size
	offset, err := s.seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("seeking to end: %w", err)
	}

	// Write ID
	if _, err := s.file.Write(node.ID[:]); err != nil {
		return 0, fmt.Errorf("writing ID: %w", err)
	}

	// Write Type
	if _, err := s.file.Write(node.Type[:]); err != nil {
		return 0, fmt.Errorf("writing type: %w", err)
	}

	// Write timestamps
	if err := binary.Write(s.file, binary.LittleEndian, node.Created); err != nil {
		return 0, fmt.Errorf("writing created time: %w", err)
	}
	if err := binary.Write(s.file, binary.LittleEndian, node.Modified); err != nil {
		return 0, fmt.Errorf("writing modified time: %w", err)
	}

	// Write metadata length
	if err := binary.Write(s.file, binary.LittleEndian, node.MetaLen); err != nil {
		return 0, fmt.Errorf("writing metadata length: %w", err)
	}

	// Write metadata
	if _, err := s.file.Write(node.Meta); err != nil {
		return 0, fmt.Errorf("writing metadata: %w", err)
	}

	return uint64(offset), nil
}
