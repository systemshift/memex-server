package storage

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
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
	blobs  []IndexEntry // Blob index
	chunks *ChunkStore  // Chunk storage
	mu     sync.RWMutex // Mutex for thread safety
	pos    int64        // Current file position
}

// IndexEntry represents an index entry
type IndexEntry struct {
	ID     [32]byte // Node ID, edge ID, or content hash
	Offset uint64   // File offset to data
	Length uint32   // Length of data
}

// Transaction represents an atomic operation
type Transaction struct {
	store    *MXStore
	startPos int64
	writes   [][]byte
	indexes  []IndexEntry
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
	}

	// Create chunk store
	chunksPath := filepath.Join(filepath.Dir(path), ".chunks")
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
	// Open file
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}

	// Create store
	store := &MXStore{
		path: path,
		file: file,
	}

	// Read header
	if err := store.readHeader(); err != nil {
		file.Close()
		return nil, fmt.Errorf("reading header: %w", err)
	}

	// Open chunk store
	chunksPath := filepath.Join(filepath.Dir(path), ".chunks")
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

// beginTransaction starts a new transaction
func (s *MXStore) beginTransaction() (*Transaction, error) {
	// Get current position
	pos, err := s.file.Seek(0, os.SEEK_CUR)
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
	// Get current position
	pos := tx.store.pos

	// Add write to transaction
	tx.writes = append(tx.writes, data)

	// Update position
	tx.store.pos += int64(len(data))

	return uint64(pos), nil
}

// addIndex adds an index entry to the transaction
func (tx *Transaction) addIndex(entry IndexEntry) {
	tx.indexes = append(tx.indexes, entry)
}

// commit applies the transaction
func (tx *Transaction) commit() error {
	s := tx.store

	// Write all data
	for _, data := range tx.writes {
		if _, err := s.file.Write(data); err != nil {
			return fmt.Errorf("writing data: %w", err)
		}
	}

	// Update indexes
	s.nodes = append(s.nodes, tx.indexes...)

	// Update header
	s.header.NodeCount = uint32(len(s.nodes))
	s.header.Modified = time.Now()

	// Write indexes and header in a single operation
	if err := s.writeIndexesAndHeader(); err != nil {
		return fmt.Errorf("writing indexes and header: %w", err)
	}

	// Sync file
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("syncing file: %w", err)
	}

	return nil
}

// writeIndexesAndHeader writes both indexes and header in a single atomic operation
func (s *MXStore) writeIndexesAndHeader() error {
	// Save current position
	currentPos := s.pos

	// Get current file size for initial offset
	size, err := s.file.Seek(0, os.SEEK_END)
	if err != nil {
		return fmt.Errorf("getting file size: %w", err)
	}

	// Update header with new index offsets
	s.header.NodeIndex = uint64(size)
	s.header.EdgeIndex = uint64(size + int64(binary.Size(s.nodes)))
	s.header.BlobIndex = uint64(size + int64(binary.Size(s.nodes)) + int64(binary.Size(s.edges)))

	// Write header first
	if _, err := s.file.Seek(0, os.SEEK_SET); err != nil {
		return fmt.Errorf("seeking to start: %w", err)
	}
	if err := s.writeHeader(); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	// Write indexes at end of file
	if _, err := s.file.Seek(size, os.SEEK_SET); err != nil {
		return fmt.Errorf("seeking to end: %w", err)
	}
	if err := binary.Write(s.file, binary.LittleEndian, s.nodes); err != nil {
		return fmt.Errorf("writing node index: %w", err)
	}
	if err := binary.Write(s.file, binary.LittleEndian, s.edges); err != nil {
		return fmt.Errorf("writing edge index: %w", err)
	}
	if err := binary.Write(s.file, binary.LittleEndian, s.blobs); err != nil {
		return fmt.Errorf("writing blob index: %w", err)
	}

	// Restore position
	if _, err := s.file.Seek(currentPos, os.SEEK_SET); err != nil {
		return fmt.Errorf("restoring position: %w", err)
	}
	s.pos = currentPos

	return nil
}

// rollback aborts the transaction
func (tx *Transaction) rollback() error {
	// Restore original position
	if _, err := tx.store.file.Seek(tx.startPos, os.SEEK_SET); err != nil {
		return fmt.Errorf("restoring position: %w", err)
	}

	tx.store.pos = tx.startPos
	return nil
}

// seek moves the file pointer
func (s *MXStore) seek(offset int64, whence int) (int64, error) {
	pos, err := s.file.Seek(offset, whence)
	if err != nil {
		return 0, err
	}
	s.pos = pos
	return pos, nil
}
