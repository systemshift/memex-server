package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Current file format version
const Version = 1

// MXStore implements a graph-oriented file format
type MXStore struct {
	path   string       // Path to .mx file
	file   *os.File     // File handle
	header Header       // File header
	nodes  []IndexEntry // Node index
	edges  []IndexEntry // Edge index
	chunks *ChunkStore  // Chunk storage
	mu     sync.RWMutex // Mutex for thread safety
	logger Logger       // Logger interface
}

// CreateMX creates a new repository
func CreateMX(path string) (*MXStore, error) {
	// Create file
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return nil, fmt.Errorf("creating file: %w", err)
	}

	// Create store
	store := &MXStore{
		path: path,
		file: file,
		header: Header{
			Version:  Version,
			Created:  time.Now(),
			Modified: time.Now(),
		},
		logger: &NoopLogger{},
	}

	// Create chunk store
	chunksPath := getChunksPath(path)
	if err := os.MkdirAll(chunksPath, 0755); err != nil {
		file.Close()
		return nil, fmt.Errorf("creating chunks directory: %w", err)
	}
	store.chunks = NewChunkStore(chunksPath)

	// Write initial header
	if err := store.writeHeader(); err != nil {
		file.Close()
		return nil, fmt.Errorf("writing header: %w", err)
	}

	return store, nil
}

// OpenMX opens an existing repository
func OpenMX(path string) (*MXStore, error) {
	// Get absolute path to repository
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Open file
	file, err := os.OpenFile(absPath, os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}

	// Create store
	store := &MXStore{
		path:   absPath,
		file:   file,
		logger: &NoopLogger{},
	}

	// Read header
	if err := store.readHeader(); err != nil {
		file.Close()
		return nil, fmt.Errorf("reading header: %w", err)
	}

	// Open chunk store
	chunksPath := getChunksPath(absPath)
	store.chunks = NewChunkStore(chunksPath)

	// Read indexes
	if err := store.readIndexes(); err != nil {
		file.Close()
		return nil, fmt.Errorf("reading indexes: %w", err)
	}

	return store, nil
}

// Close closes the repository
func (s *MXStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Write header and indexes before closing
	if err := s.writeHeader(); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	if err := s.writeIndexes(); err != nil {
		return fmt.Errorf("writing indexes: %w", err)
	}

	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("syncing file: %w", err)
	}

	if err := s.file.Close(); err != nil {
		return fmt.Errorf("closing file: %w", err)
	}

	return nil
}

// Path returns the repository path
func (s *MXStore) Path() string {
	return s.path
}

// Nodes returns all nodes in the repository
func (s *MXStore) Nodes() []IndexEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nodes
}

// GetChunk returns the content of a chunk by its hash
func (s *MXStore) GetChunk(hash string) ([]byte, error) {
	return s.chunks.Get(hash)
}

// StoreChunk stores a chunk and returns its hash
func (s *MXStore) StoreChunk(content []byte) (string, error) {
	return s.chunks.Store(content)
}

// ReconstructContent reconstructs the full content from its chunks
func (s *MXStore) ReconstructContent(contentHash string) ([]byte, error) {
	// Find node with this content hash
	for _, entry := range s.nodes {
		if _, err := s.seek(int64(entry.Offset), 0); err != nil {
			continue
		}

		// Read length prefix
		var length uint32
		if err := binary.Read(s.file, binary.LittleEndian, &length); err != nil {
			continue
		}

		// Read node data
		data := make([]byte, length)
		if _, err := s.file.Read(data); err != nil {
			continue
		}

		// Parse node data
		var node struct {
			Meta map[string]interface{} `json:"meta"`
		}
		if err := json.Unmarshal(data, &node); err != nil {
			continue
		}

		if hash, ok := node.Meta["content"].(string); ok && hash == contentHash {
			// Found the node, now get its chunks
			if chunksRaw, ok := node.Meta["chunks"].([]interface{}); ok {
				// Convert chunks to []string
				var chunkList []string
				for _, chunk := range chunksRaw {
					if chunkStr, ok := chunk.(string); ok {
						chunkStr = strings.Trim(chunkStr, `"`)
						chunkList = append(chunkList, chunkStr)
					}
				}

				// Load and concatenate chunks
				var content bytes.Buffer
				for _, chunkHash := range chunkList {
					chunk, err := s.chunks.Get(chunkHash)
					if err != nil {
						return nil, fmt.Errorf("loading chunk %s: %w", chunkHash, err)
					}
					content.Write(chunk)
				}

				// Return reconstructed content
				return content.Bytes(), nil
			}
		}
	}

	return nil, fmt.Errorf("content not found: %s", contentHash)
}

// beginTransaction starts a new transaction
func (s *MXStore) beginTransaction() (*Transaction, error) {
	// Get current position
	pos, err := s.file.Seek(0, os.SEEK_END)
	if err != nil {
		return nil, fmt.Errorf("getting position: %w", err)
	}

	return &Transaction{
		store:    s,
		startPos: pos,
		writes:   make([][]byte, 0),
		indexes:  make([]IndexEntry, 0),
	}, nil
}

// write adds data to the transaction
func (tx *Transaction) write(data []byte) (uint64, error) {
	// Write length prefix
	length := uint32(len(data))
	if err := binary.Write(tx.store.file, binary.LittleEndian, length); err != nil {
		return 0, fmt.Errorf("writing length: %w", err)
	}

	// Write data
	n, err := tx.store.file.Write(data)
	if err != nil {
		return 0, fmt.Errorf("writing data: %w", err)
	}
	if n != len(data) {
		return 0, fmt.Errorf("short write: wrote %d of %d bytes", n, len(data))
	}

	return uint64(tx.startPos), nil
}

// addIndex adds an index entry to the transaction
func (tx *Transaction) addIndex(entry IndexEntry) {
	tx.indexes = append(tx.indexes, entry)
}

// commit applies the transaction
func (tx *Transaction) commit() error {
	s := tx.store

	// Update indexes
	for _, entry := range tx.indexes {
		if tx.isEdge {
			s.edges = append(s.edges, entry)
			s.header.EdgeCount++
		} else {
			s.nodes = append(s.nodes, entry)
			s.header.NodeCount++
		}
	}

	// Update header
	s.header.Modified = time.Now()

	// Update header with new index offsets
	s.header.NodeIndex = uint64(tx.startPos + int64(len(tx.writes)))
	s.header.EdgeIndex = s.header.NodeIndex + uint64(len(s.nodes)*indexEntrySize)

	// Write indexes
	if err := s.writeIndexes(); err != nil {
		return fmt.Errorf("writing indexes: %w", err)
	}

	// Write header at start of file
	if _, err := s.file.Seek(0, os.SEEK_SET); err != nil {
		return fmt.Errorf("seeking to start: %w", err)
	}
	if err := s.writeHeader(); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	// Sync file
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("syncing file: %w", err)
	}

	return nil
}

// rollback aborts the transaction
func (tx *Transaction) rollback() error {
	// Restore original position
	if _, err := tx.store.file.Seek(tx.startPos, os.SEEK_SET); err != nil {
		return fmt.Errorf("restoring position: %w", err)
	}
	return nil
}

// seek moves the file pointer
func (s *MXStore) seek(offset int64, whence int) (int64, error) {
	return s.file.Seek(offset, whence)
}

// getChunksPath returns the path to the chunks directory
func getChunksPath(repoPath string) string {
	// Get absolute path to repository
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return filepath.Join(filepath.Dir(repoPath), filepath.Base(repoPath)+".chunks")
	}

	// Get the repository directory and base name
	dir := filepath.Dir(absPath)
	base := filepath.Base(absPath)

	// Create chunks directory name by appending .chunks to the repository name
	chunksDir := base + ".chunks"

	// Return full path to chunks directory
	return filepath.Join(dir, chunksDir)
}

// writeIndexes writes the node and edge indexes to the file
func (s *MXStore) writeIndexes() error {
	// Write node index
	if _, err := s.file.Seek(int64(s.header.NodeIndex), os.SEEK_SET); err != nil {
		return fmt.Errorf("seeking to node index: %w", err)
	}

	for _, entry := range s.nodes {
		if err := binary.Write(s.file, binary.LittleEndian, entry.ID); err != nil {
			return fmt.Errorf("writing node ID: %w", err)
		}
		if err := binary.Write(s.file, binary.LittleEndian, entry.Offset); err != nil {
			return fmt.Errorf("writing node offset: %w", err)
		}
		if err := binary.Write(s.file, binary.LittleEndian, entry.Length); err != nil {
			return fmt.Errorf("writing node length: %w", err)
		}
	}

	// Write edge index
	if _, err := s.file.Seek(int64(s.header.EdgeIndex), os.SEEK_SET); err != nil {
		return fmt.Errorf("seeking to edge index: %w", err)
	}

	for _, entry := range s.edges {
		if err := binary.Write(s.file, binary.LittleEndian, entry.ID); err != nil {
			return fmt.Errorf("writing edge ID: %w", err)
		}
		if err := binary.Write(s.file, binary.LittleEndian, entry.Offset); err != nil {
			return fmt.Errorf("writing edge offset: %w", err)
		}
		if err := binary.Write(s.file, binary.LittleEndian, entry.Length); err != nil {
			return fmt.Errorf("writing edge length: %w", err)
		}
	}

	return nil
}
