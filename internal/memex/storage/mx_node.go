package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
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

// writeNode writes node data to the file
func (s *MXStore) writeNode(node NodeData) (uint64, error) {
	offset, err := s.file.Seek(0, io.SeekEnd)
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

// AddNode adds a node to the store
func (s *MXStore) AddNode(content []byte, nodeType string, meta map[string]any) (string, error) {
	// Store content first
	blobHash, err := s.storeBlob(content)
	if err != nil {
		return "", fmt.Errorf("storing content: %w", err)
	}

	// Add content hash to metadata
	if meta == nil {
		meta = make(map[string]any)
	}
	meta["content"] = blobHash

	// Generate node ID
	id := generateID()

	// Serialize metadata
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("serializing metadata: %w", err)
	}

	// Create node data
	now := time.Now().Unix()
	var nodeData NodeData
	copy(nodeData.ID[:], id)
	copy(nodeData.Type[:], []byte(nodeType))
	nodeData.Created = now
	nodeData.Modified = now
	nodeData.MetaLen = uint32(len(metaJSON))
	nodeData.Meta = metaJSON

	// Write node data
	offset, err := s.writeNode(nodeData)
	if err != nil {
		return "", fmt.Errorf("writing node: %w", err)
	}

	// Add to index
	var idBytes [32]byte
	copy(idBytes[:], id)
	s.nodes = append(s.nodes, IndexEntry{
		ID:     idBytes,
		Offset: offset,
		Length: uint32(80 + len(metaJSON)), // Fixed size + metadata
	})

	// Update header
	s.header.NodeCount++
	s.header.Modified = time.Now()
	if err := s.writeHeader(); err != nil {
		return "", fmt.Errorf("updating header: %w", err)
	}

	return hex.EncodeToString(idBytes[:]), nil
}

// GetNode retrieves a node by ID
func (s *MXStore) GetNode(id string) (core.Node, error) {
	fmt.Fprintf(os.Stderr, "Getting node: %s\n", id)

	// Convert ID to bytes
	idBytes, err := hex.DecodeString(id)
	if err != nil {
		return core.Node{}, fmt.Errorf("invalid ID: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Looking for ID bytes: %x\n", idBytes)

	// Pad ID to 32 bytes
	var fullID [32]byte
	copy(fullID[:], idBytes)

	// Find node in index
	var entry IndexEntry
	for _, e := range s.nodes {
		fmt.Fprintf(os.Stderr, "Checking index entry: %x at offset %d\n", e.ID, e.Offset)
		// Compare just the prefix of the ID that was provided
		if bytes.HasPrefix(e.ID[:], idBytes) {
			entry = e
			break
		}
	}
	if entry.ID == [32]byte{} {
		return core.Node{}, fmt.Errorf("node not found in index")
	}

	fmt.Fprintf(os.Stderr, "Found node at offset: %d\n", entry.Offset)

	// Seek to node data
	if _, err := s.file.Seek(int64(entry.Offset), io.SeekStart); err != nil {
		return core.Node{}, fmt.Errorf("seeking to node: %w", err)
	}

	// Read node data
	var nodeData NodeData

	// Read ID
	if _, err := io.ReadFull(s.file, nodeData.ID[:]); err != nil {
		return core.Node{}, fmt.Errorf("reading ID: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Read node ID: %x\n", nodeData.ID)

	// Read Type
	if _, err := io.ReadFull(s.file, nodeData.Type[:]); err != nil {
		return core.Node{}, fmt.Errorf("reading type: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Read node type: %s\n", bytes.TrimRight(nodeData.Type[:], "\x00"))

	// Read timestamps
	if err := binary.Read(s.file, binary.LittleEndian, &nodeData.Created); err != nil {
		return core.Node{}, fmt.Errorf("reading created time: %w", err)
	}
	if err := binary.Read(s.file, binary.LittleEndian, &nodeData.Modified); err != nil {
		return core.Node{}, fmt.Errorf("reading modified time: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Read timestamps: created=%d modified=%d\n", nodeData.Created, nodeData.Modified)

	// Read metadata length
	if err := binary.Read(s.file, binary.LittleEndian, &nodeData.MetaLen); err != nil {
		return core.Node{}, fmt.Errorf("reading metadata length: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Read metadata length: %d\n", nodeData.MetaLen)

	// Read metadata
	nodeData.Meta = make([]byte, nodeData.MetaLen)
	if _, err := io.ReadFull(s.file, nodeData.Meta); err != nil {
		return core.Node{}, fmt.Errorf("reading metadata: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Read metadata: %s\n", nodeData.Meta)

	// Parse metadata
	var meta map[string]any
	if err := json.Unmarshal(nodeData.Meta, &meta); err != nil {
		return core.Node{}, fmt.Errorf("parsing metadata: %w", err)
	}

	// Load content if available
	var versions []core.Version
	if contentHash, ok := meta["content"].(string); ok {
		// Create version for content
		version := core.Version{
			Hash:      contentHash,
			Chunks:    []string{contentHash}, // Single chunk for now
			Created:   time.Unix(nodeData.Created, 0),
			Meta:      make(map[string]any),
			Available: true,
		}
		versions = append(versions, version)
	}

	// Convert to core.Node
	return core.Node{
		ID:       hex.EncodeToString(nodeData.ID[:]),
		Type:     string(bytes.TrimRight(nodeData.Type[:], "\x00")),
		Meta:     meta,
		Created:  time.Unix(nodeData.Created, 0),
		Modified: time.Unix(nodeData.Modified, 0),
		Versions: versions,
		Current:  versions[0].Hash,
	}, nil
}

// DeleteNode removes a node
func (s *MXStore) DeleteNode(id string) error {
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

	// Read node data to get content hash
	if _, err := s.file.Seek(int64(entry.Offset), io.SeekStart); err != nil {
		return fmt.Errorf("seeking to node: %w", err)
	}

	var nodeData NodeData
	if err := binary.Read(s.file, binary.LittleEndian, &nodeData); err != nil {
		return fmt.Errorf("reading node: %w", err)
	}

	// Parse metadata to get content hash
	var meta map[string]any
	if err := json.Unmarshal(nodeData.Meta, &meta); err != nil {
		return fmt.Errorf("parsing metadata: %w", err)
	}

	// Remove content if available
	if contentHash, ok := meta["content"].(string); ok {
		// Convert hash to bytes
		hashBytes, err := hex.DecodeString(contentHash)
		if err != nil {
			return fmt.Errorf("invalid content hash: %w", err)
		}

		// Find blob in index
		var blobIndex int
		var blobEntry IndexEntry
		for i, e := range s.blobs {
			if bytes.HasPrefix(e.ID[:], hashBytes) {
				blobEntry = e
				blobIndex = i
				break
			}
		}

		if blobEntry.ID != [32]byte{} {
			// Remove blob from index
			s.blobs = append(s.blobs[:blobIndex], s.blobs[blobIndex+1:]...)
			s.header.BlobCount--
		}
	}

	// Remove node from index
	s.nodes = append(s.nodes[:nodeIndex], s.nodes[nodeIndex+1:]...)
	s.header.NodeCount--
	s.header.Modified = time.Now()

	// Update header
	if err := s.writeHeader(); err != nil {
		return fmt.Errorf("updating header: %w", err)
	}

	return nil
}

// generateID generates a unique ID
func generateID() []byte {
	id := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return id[:]
}
