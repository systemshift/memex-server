package store

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"sync"

	"memex/internal/memex/storage"
	"memex/internal/memex/transaction"
)

// ChunkStore implements basic content-addressed storage
type ChunkStore struct {
	file    *os.File
	chunker storage.Chunker
	index   map[string]int64 // Maps chunk hash to file offset
	mutex   sync.RWMutex
	txStore *transaction.ActionStore
}

// ChunkHeader represents the on-disk format of a chunk
type ChunkHeader struct {
	Size     uint32   // Size of chunk data
	Hash     [32]byte // SHA-256 hash of chunk data
	RefCount uint32   // Number of references to this chunk
}

// NewStore creates a new chunk store
func NewStore(path string, chunker storage.Chunker, txStore *transaction.ActionStore) (*ChunkStore, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}

	store := &ChunkStore{
		file:    file,
		chunker: chunker,
		index:   make(map[string]int64),
		txStore: txStore,
	}

	// Load existing chunks
	if err := store.loadIndex(); err != nil {
		file.Close()
		return nil, fmt.Errorf("loading index: %w", err)
	}

	return store, nil
}

// Put stores content and returns chunk addresses
func (s *ChunkStore) Put(content []byte) ([][]byte, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Split content into chunks
	chunks := s.chunker.Split(content)
	addresses := make([][]byte, len(chunks))

	// Record action
	if err := s.txStore.RecordAction(transaction.ActionPutContent, map[string]any{
		"size":   len(content),
		"chunks": len(chunks),
	}); err != nil {
		return nil, fmt.Errorf("recording action: %w", err)
	}

	for i, chunk := range chunks {
		// Calculate chunk hash
		hash := sha256.Sum256(chunk)
		hashStr := string(hash[:])

		// Check if chunk exists
		if offset, exists := s.index[hashStr]; exists {
			// Update ref count
			if err := s.incrementRefCount(offset); err != nil {
				return nil, fmt.Errorf("updating ref count: %w", err)
			}
			addresses[i] = hash[:]
			continue
		}

		// Write new chunk
		offset, err := s.writeChunk(chunk, hash)
		if err != nil {
			return nil, fmt.Errorf("writing chunk: %w", err)
		}

		s.index[hashStr] = offset
		addresses[i] = hash[:]
	}

	return addresses, nil
}

// Get retrieves content from chunk addresses
func (s *ChunkStore) Get(addresses [][]byte) ([]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var content []byte
	for _, addr := range addresses {
		// Get chunk offset
		hashStr := string(addr)
		offset, exists := s.index[hashStr]
		if !exists {
			return nil, fmt.Errorf("chunk not found: %x", addr)
		}

		// Read chunk
		chunk, err := s.readChunk(offset)
		if err != nil {
			return nil, fmt.Errorf("reading chunk: %w", err)
		}

		content = append(content, chunk...)
	}

	return content, nil
}

// Delete marks chunks as deleted
func (s *ChunkStore) Delete(addresses [][]byte) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Record action
	if err := s.txStore.RecordAction(transaction.ActionDeleteContent, map[string]any{
		"addresses": addresses,
	}); err != nil {
		return fmt.Errorf("recording action: %w", err)
	}

	for _, addr := range addresses {
		hashStr := string(addr)
		offset, exists := s.index[hashStr]
		if !exists {
			continue
		}

		// Decrement ref count
		if err := s.decrementRefCount(offset); err != nil {
			return fmt.Errorf("updating ref count: %w", err)
		}
	}

	return nil
}

// Close releases resources
func (s *ChunkStore) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.file.Close()
}

// Internal methods

func (s *ChunkStore) loadIndex() error {
	// Get file size
	info, err := s.file.Stat()
	if err != nil {
		return fmt.Errorf("getting file info: %w", err)
	}

	// Read all chunks
	var offset int64
	for offset < info.Size() {
		// Read header
		header := ChunkHeader{}
		if err := binary.Read(s.file, binary.LittleEndian, &header); err != nil {
			return fmt.Errorf("reading header: %w", err)
		}

		// Skip chunk data
		if _, err := s.file.Seek(int64(header.Size), os.SEEK_CUR); err != nil {
			return fmt.Errorf("seeking past chunk: %w", err)
		}

		// Add to index if ref count > 0
		if header.RefCount > 0 {
			s.index[string(header.Hash[:])] = offset
		}

		offset += int64(binary.Size(header) + int(header.Size))
	}

	return nil
}

func (s *ChunkStore) writeChunk(data []byte, hash [32]byte) (int64, error) {
	// Get current position
	offset, err := s.file.Seek(0, os.SEEK_END)
	if err != nil {
		return 0, fmt.Errorf("seeking to end: %w", err)
	}

	// Write header
	header := ChunkHeader{
		Size:     uint32(len(data)),
		Hash:     hash,
		RefCount: 1,
	}
	if err := binary.Write(s.file, binary.LittleEndian, header); err != nil {
		return 0, fmt.Errorf("writing header: %w", err)
	}

	// Write data
	if _, err := s.file.Write(data); err != nil {
		return 0, fmt.Errorf("writing data: %w", err)
	}

	return offset, nil
}

func (s *ChunkStore) readChunk(offset int64) ([]byte, error) {
	// Read header
	if _, err := s.file.Seek(offset, os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("seeking to chunk: %w", err)
	}

	header := ChunkHeader{}
	if err := binary.Read(s.file, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	// Read data
	data := make([]byte, header.Size)
	if _, err := s.file.Read(data); err != nil {
		return nil, fmt.Errorf("reading data: %w", err)
	}

	return data, nil
}

func (s *ChunkStore) incrementRefCount(offset int64) error {
	header := ChunkHeader{}

	// Read current header
	if _, err := s.file.Seek(offset, os.SEEK_SET); err != nil {
		return fmt.Errorf("seeking to chunk: %w", err)
	}
	if err := binary.Read(s.file, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("reading header: %w", err)
	}

	// Update ref count
	header.RefCount++

	// Write updated header
	if _, err := s.file.Seek(offset, os.SEEK_SET); err != nil {
		return fmt.Errorf("seeking to chunk: %w", err)
	}
	if err := binary.Write(s.file, binary.LittleEndian, header); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	return nil
}

func (s *ChunkStore) decrementRefCount(offset int64) error {
	header := ChunkHeader{}

	// Read current header
	if _, err := s.file.Seek(offset, os.SEEK_SET); err != nil {
		return fmt.Errorf("seeking to chunk: %w", err)
	}
	if err := binary.Read(s.file, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("reading header: %w", err)
	}

	// Update ref count
	if header.RefCount > 0 {
		header.RefCount--
	}

	// Write updated header
	if _, err := s.file.Seek(offset, os.SEEK_SET); err != nil {
		return fmt.Errorf("seeking to chunk: %w", err)
	}
	if err := binary.Write(s.file, binary.LittleEndian, header); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	// Remove from index if ref count is 0
	if header.RefCount == 0 {
		delete(s.index, string(header.Hash[:]))
	}

	return nil
}
