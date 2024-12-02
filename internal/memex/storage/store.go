package storage

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"memex/internal/memex/core"
	"memex/internal/memex/storage/chunk"
	"memex/internal/memex/storage/common"
)

// MXStore represents a Memex repository
type MXStore struct {
	path       string
	file       *common.File
	locks      *common.LockManager
	header     Header
	nodes      []IndexEntry
	edges      []IndexEntry
	chunkIndex []IndexEntry
	mutex      sync.RWMutex
}

// CreateMX creates a new repository at the given path
func CreateMX(path string) (*MXStore, error) {
	// Create file
	file, err := common.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("creating file: %w", err)
	}

	// Create store
	store := &MXStore{
		path:       path,
		file:       file,
		locks:      common.NewLockManager(),
		nodes:      make([]IndexEntry, 0),
		edges:      make([]IndexEntry, 0),
		chunkIndex: make([]IndexEntry, 0),
	}

	// Initialize header
	store.header = Header{
		Version:    Version,
		Created:    time.Now(),
		Modified:   time.Now(),
		NodeCount:  0,
		EdgeCount:  0,
		ChunkCount: 0,
	}

	// Write initial header
	if err := store.writeHeader(); err != nil {
		file.Close()
		return nil, fmt.Errorf("writing header: %w", err)
	}

	return store, nil
}

// OpenMX opens an existing repository
func OpenMX(path string) (*MXStore, error) {
	// Open file
	file, err := common.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}

	// Create store
	store := &MXStore{
		path:  path,
		file:  file,
		locks: common.NewLockManager(),
	}

	// Read header
	if err := store.readHeader(); err != nil {
		file.Close()
		return nil, fmt.Errorf("reading header: %w", err)
	}

	// Read indexes
	if err := store.readIndexes(); err != nil {
		file.Close()
		return nil, fmt.Errorf("reading indexes: %w", err)
	}

	return store, nil
}

// GetFile returns the store's file handle
func (s *MXStore) GetFile() *common.File {
	return s.file
}

// GetLockManager returns the store's lock manager
func (s *MXStore) GetLockManager() *common.LockManager {
	return s.locks
}

// Path returns the repository path
func (s *MXStore) Path() string {
	return s.path
}

// GetNode retrieves a node by ID
func (s *MXStore) GetNode(id string) (*core.Node, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Find node in index
	for _, entry := range s.nodes {
		if fmt.Sprintf("%x", entry.ID[:]) == id {
			// Read node data
			data, err := s.readData(entry)
			if err != nil {
				return nil, fmt.Errorf("reading node data: %w", err)
			}

			// Parse node
			node := &core.Node{}
			if err := json.Unmarshal(data, node); err != nil {
				return nil, fmt.Errorf("parsing node: %w", err)
			}

			// Set node ID
			node.ID = fmt.Sprintf("%x", entry.ID)
			return node, nil
		}
	}

	return nil, fmt.Errorf("node not found: %s", id)
}

// AddNode adds a node to the store
func (s *MXStore) AddNode(content []byte, nodeType string, meta map[string]any) (string, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Create chunks
	chunks, err := chunk.Split(content)
	if err != nil {
		return "", fmt.Errorf("chunking content: %w", err)
	}

	// Store chunks
	var chunkHashes []string
	for _, c := range chunks {
		hash, err := s.StoreChunk(c.Content)
		if err != nil {
			return "", fmt.Errorf("storing chunk: %w", err)
		}
		chunkHashes = append(chunkHashes, hash)
	}

	// Create node
	node := &core.Node{
		Type:     nodeType,
		Created:  time.Now(),
		Modified: time.Now(),
		Meta:     meta,
	}

	// Add content hash and chunks to metadata
	contentHash := chunks[0].Hash // Use first chunk hash as content identifier
	if node.Meta == nil {
		node.Meta = make(map[string]any)
	}
	node.Meta["content"] = contentHash
	node.Meta["chunks"] = chunkHashes

	// Marshal node
	data, err := json.Marshal(node)
	if err != nil {
		return "", fmt.Errorf("marshaling node: %w", err)
	}

	// Write node data
	offset, err := s.writeData(data)
	if err != nil {
		return "", fmt.Errorf("writing node data: %w", err)
	}

	// Create index entry
	entry := IndexEntry{
		ID:     sha256.Sum256(data),
		Offset: offset,
		Length: uint32(len(data)),
	}

	// Add to index
	s.nodes = append(s.nodes, entry)
	s.header.NodeCount++

	return fmt.Sprintf("%x", entry.ID), nil
}

// DeleteNode removes a node from the store
func (s *MXStore) DeleteNode(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Find and remove node
	found := false
	newNodes := make([]IndexEntry, 0, len(s.nodes))
	for _, entry := range s.nodes {
		if fmt.Sprintf("%x", entry.ID[:]) == id {
			found = true
			continue
		}
		newNodes = append(newNodes, entry)
	}

	if !found {
		return fmt.Errorf("node not found: %s", id)
	}

	// Update nodes list
	s.nodes = newNodes
	s.header.NodeCount--

	return nil
}

// GetLinks retrieves links for a node
func (s *MXStore) GetLinks(nodeID string) ([]*core.Link, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var links []*core.Link

	// Find all edges involving this node
	for _, entry := range s.edges {
		// Read edge data
		data, err := s.readData(entry)
		if err != nil {
			continue
		}

		// Parse link
		link := &core.Link{}
		if err := json.Unmarshal(data, link); err != nil {
			continue
		}

		// Check if this edge involves our node
		if link.Source == nodeID || link.Target == nodeID {
			links = append(links, link)
		}
	}

	return links, nil
}

// AddLink creates a link between two nodes
func (s *MXStore) AddLink(sourceID, targetID, linkType string, meta map[string]any) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Create link
	link := &core.Link{
		Source: sourceID,
		Target: targetID,
		Type:   linkType,
		Meta:   meta,
	}

	// Marshal link
	data, err := json.Marshal(link)
	if err != nil {
		return fmt.Errorf("marshaling link: %w", err)
	}

	// Write link data
	offset, err := s.writeData(data)
	if err != nil {
		return fmt.Errorf("writing link data: %w", err)
	}

	// Create index entry
	entry := IndexEntry{
		ID:     sha256.Sum256(data),
		Offset: offset,
		Length: uint32(len(data)),
	}

	// Add to index
	s.edges = append(s.edges, entry)
	s.header.EdgeCount++

	return nil
}

// DeleteLink removes a link between two nodes
func (s *MXStore) DeleteLink(sourceID, targetID, linkType string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Find and remove edge
	found := false
	newEdges := make([]IndexEntry, 0, len(s.edges))
	for _, entry := range s.edges {
		// Read edge data
		data, err := s.readData(entry)
		if err != nil {
			continue
		}

		// Parse link
		link := &core.Link{}
		if err := json.Unmarshal(data, link); err != nil {
			continue
		}

		// Check if this is the edge we want to delete
		if link.Source == sourceID && link.Target == targetID && link.Type == linkType {
			found = true
			continue
		}

		newEdges = append(newEdges, entry)
	}

	if !found {
		return fmt.Errorf("link not found: %s -[%s]-> %s", sourceID, linkType, targetID)
	}

	// Update edges list
	s.edges = newEdges
	s.header.EdgeCount--

	return nil
}

// GetChunk retrieves a chunk by its hash
func (s *MXStore) GetChunk(hash string) ([]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Find chunk in index
	for _, entry := range s.chunkIndex {
		if fmt.Sprintf("%x", entry.ID[:]) == hash {
			// Read chunk data
			data, err := s.readData(entry)
			if err != nil {
				return nil, fmt.Errorf("reading chunk data: %w", err)
			}

			return data, nil
		}
	}

	return nil, fmt.Errorf("chunk not found: %s", hash)
}

// StoreChunk stores a chunk and returns its hash
func (s *MXStore) StoreChunk(content []byte) (string, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Calculate hash
	hash := sha256.Sum256(content)
	hashStr := fmt.Sprintf("%x", hash)

	// Check if chunk already exists
	for _, entry := range s.chunkIndex {
		if fmt.Sprintf("%x", entry.ID[:]) == hashStr {
			return hashStr, nil
		}
	}

	// Write chunk data
	offset, err := s.writeData(content)
	if err != nil {
		return "", fmt.Errorf("writing chunk data: %w", err)
	}

	// Create index entry
	entry := IndexEntry{
		ID:     hash,
		Offset: offset,
		Length: uint32(len(content)),
	}

	// Add to index
	s.chunkIndex = append(s.chunkIndex, entry)
	s.header.ChunkCount++

	return hashStr, nil
}

// ReconstructContent reconstructs content from chunks
func (s *MXStore) ReconstructContent(contentHash string) ([]byte, error) {
	// Get content chunk
	content, err := s.GetChunk(contentHash)
	if err != nil {
		return nil, fmt.Errorf("loading chunk %s: %w", contentHash, err)
	}

	return content, nil
}

// Nodes returns all nodes in the store
func (s *MXStore) Nodes() []*core.Node {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	nodes := make([]*core.Node, 0, len(s.nodes))
	for _, entry := range s.nodes {
		// Read node data
		data, err := s.readData(entry)
		if err != nil {
			continue
		}

		// Parse node
		node := &core.Node{}
		if err := json.Unmarshal(data, node); err != nil {
			continue
		}

		// Set node ID
		node.ID = fmt.Sprintf("%x", entry.ID)
		nodes = append(nodes, node)
	}
	return nodes
}

// NodeEntries returns all node index entries (for internal use)
func (s *MXStore) NodeEntries() []IndexEntry {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Return a copy to prevent modification
	entries := make([]IndexEntry, len(s.nodes))
	copy(entries, s.nodes)
	return entries
}

// Close closes the repository
func (s *MXStore) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Write header and indexes
	if err := s.writeHeader(); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	if err := s.writeIndexes(); err != nil {
		return fmt.Errorf("writing indexes: %w", err)
	}

	// Close file
	return s.file.Close()
}

// Internal methods

func (s *MXStore) readData(entry IndexEntry) ([]byte, error) {
	// Read data
	data := make([]byte, entry.Length)
	if _, err := s.file.ReadAt(data, int64(entry.Offset)); err != nil {
		return nil, fmt.Errorf("reading data: %w", err)
	}

	return data, nil
}

func (s *MXStore) writeData(data []byte) (uint64, error) {
	// Get current position
	pos, err := s.file.Seek(0, os.SEEK_END)
	if err != nil {
		return 0, fmt.Errorf("seeking to end: %w", err)
	}

	// Write data
	if _, err := s.file.WriteAt(data, pos); err != nil {
		return 0, fmt.Errorf("writing data: %w", err)
	}

	return uint64(pos), nil
}

// readHeader reads the file header
func (s *MXStore) readHeader() error {
	return s.locks.WithHeaderRLock(func() error {
		// Read magic number
		magic, err := s.file.ReadUint64()
		if err != nil {
			return fmt.Errorf("reading magic number: %w", err)
		}
		if magic != common.FileMagic {
			return fmt.Errorf("invalid magic number")
		}

		// Read header data
		var data HeaderData
		if err := binary.Read(s.file, binary.LittleEndian, &data); err != nil {
			return fmt.Errorf("reading header data: %w", err)
		}

		// Convert to header
		s.header.FromData(data)
		return nil
	})
}

// writeHeader writes the file header
func (s *MXStore) writeHeader() error {
	return s.locks.WithHeaderLock(func() error {
		// Seek to start
		if _, err := s.file.Seek(0, 0); err != nil {
			return fmt.Errorf("seeking to start: %w", err)
		}

		// Write magic number
		if err := s.file.WriteUint64(common.FileMagic); err != nil {
			return fmt.Errorf("writing magic number: %w", err)
		}

		// Write header data
		data := s.header.ToData()
		if err := binary.Write(s.file, binary.LittleEndian, data); err != nil {
			return fmt.Errorf("writing header data: %w", err)
		}

		return nil
	})
}

// readIndexes reads the node and edge indexes
func (s *MXStore) readIndexes() error {
	return s.locks.WithIndexRLock(func() error {
		// Read node index
		s.nodes = make([]IndexEntry, s.header.NodeCount)
		if _, err := s.file.Seek(int64(s.header.NodeIndex), 0); err != nil {
			return fmt.Errorf("seeking to node index: %w", err)
		}
		if err := binary.Read(s.file, binary.LittleEndian, &s.nodes); err != nil {
			return fmt.Errorf("reading node index: %w", err)
		}

		// Read edge index
		s.edges = make([]IndexEntry, s.header.EdgeCount)
		if _, err := s.file.Seek(int64(s.header.EdgeIndex), 0); err != nil {
			return fmt.Errorf("seeking to edge index: %w", err)
		}
		if err := binary.Read(s.file, binary.LittleEndian, &s.edges); err != nil {
			return fmt.Errorf("reading edge index: %w", err)
		}

		// Read chunk index
		s.chunkIndex = make([]IndexEntry, s.header.ChunkCount)
		if _, err := s.file.Seek(int64(s.header.ChunkIndex), 0); err != nil {
			return fmt.Errorf("seeking to chunk index: %w", err)
		}
		if err := binary.Read(s.file, binary.LittleEndian, &s.chunkIndex); err != nil {
			return fmt.Errorf("reading chunk index: %w", err)
		}

		return nil
	})
}

// writeIndexes writes the node and edge indexes
func (s *MXStore) writeIndexes() error {
	return s.locks.WithIndexLock(func() error {
		// Write node index
		if _, err := s.file.Seek(int64(s.header.NodeIndex), 0); err != nil {
			return fmt.Errorf("seeking to node index: %w", err)
		}
		if err := binary.Write(s.file, binary.LittleEndian, s.nodes); err != nil {
			return fmt.Errorf("writing node index: %w", err)
		}

		// Write edge index
		if _, err := s.file.Seek(int64(s.header.EdgeIndex), 0); err != nil {
			return fmt.Errorf("seeking to edge index: %w", err)
		}
		if err := binary.Write(s.file, binary.LittleEndian, s.edges); err != nil {
			return fmt.Errorf("writing edge index: %w", err)
		}

		// Write chunk index
		if _, err := s.file.Seek(int64(s.header.ChunkIndex), 0); err != nil {
			return fmt.Errorf("seeking to chunk index: %w", err)
		}
		if err := binary.Write(s.file, binary.LittleEndian, s.chunkIndex); err != nil {
			return fmt.Errorf("writing chunk index: %w", err)
		}

		return nil
	})
}
