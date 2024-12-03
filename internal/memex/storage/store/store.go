package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"

	"memex/internal/memex/core"
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
	lockMgr sync.Mutex
	path    string
}

// ChunkHeader represents the on-disk format of a chunk
type ChunkHeader struct {
	Magic    uint32   // Magic number for validation
	Size     uint32   // Size of chunk data
	Hash     [32]byte // SHA-256 hash of chunk data
	RefCount uint32   // Number of references to this chunk
	Reserved uint32   // Reserved for future use
	Checksum uint32   // Header checksum
}

const (
	// Magic number for chunk headers
	ChunkMagic = 0x4B4E4843 // "CHNK"
)

// NewStore creates a new chunk store
func NewStore(path string, chunker storage.Chunker, txStore *transaction.ActionStore) (*ChunkStore, error) {
	// Open file
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}

	store := &ChunkStore{
		file:    file,
		chunker: chunker,
		index:   make(map[string]int64),
		txStore: txStore,
		path:    path,
	}

	// Load existing chunks
	if err := store.loadIndex(); err != nil {
		store.Close()
		return nil, fmt.Errorf("loading index: %w", err)
	}

	return store, nil
}

// SetTxStore sets the transaction store
func (s *ChunkStore) SetTxStore(txStore *transaction.ActionStore) {
	s.txStore = txStore
}

// GetFile returns the underlying file for transaction storage
func (s *ChunkStore) GetFile() interface{} {
	return s.file
}

// GetLockManager returns the lock manager for transaction storage
func (s *ChunkStore) GetLockManager() interface{} {
	return &s.lockMgr
}

// Path returns the store path
func (s *ChunkStore) Path() string {
	return s.path
}

// GetNode returns a node by ID
func (s *ChunkStore) GetNode(id string) (*core.Node, error) {
	// Convert ID to byte array
	var hash [32]byte
	_, err := fmt.Sscanf(id, "%x", &hash)
	if err != nil {
		return nil, fmt.Errorf("parsing node ID: %w", err)
	}

	// Get node data
	data, err := s.Get([][]byte{hash[:]})
	if err != nil {
		return nil, fmt.Errorf("getting node data: %w", err)
	}

	// Parse node
	node := &core.Node{
		ID:      id,
		Content: data,
	}
	return node, nil
}

// GetLinks returns links for a node
func (s *ChunkStore) GetLinks(nodeID string) ([]*core.Link, error) {
	// TODO: Implement link retrieval
	return nil, nil
}

// Put stores content and returns chunk addresses
func (s *ChunkStore) Put(content []byte) ([][]byte, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Split content into chunks
	chunks := s.chunker.Split(content)
	addresses := make([][]byte, len(chunks))

	// Record transaction
	if s.txStore != nil {
		if err := s.txStore.RecordAction(transaction.ActionPutContent, map[string]any{
			"size":   len(content),
			"chunks": len(chunks),
		}); err != nil {
			return nil, fmt.Errorf("recording action: %w", err)
		}
	}

	for i, chunk := range chunks {
		// Calculate chunk hash
		hash := sha256.Sum256(chunk)
		hashStr := hex.EncodeToString(hash[:])

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

	var content bytes.Buffer
	for _, addr := range addresses {
		// Get chunk offset
		hashStr := hex.EncodeToString(addr)
		offset, exists := s.index[hashStr]
		if !exists {
			return nil, fmt.Errorf("chunk not found: %x", addr)
		}

		// Read chunk
		chunk, err := s.readChunk(offset)
		if err != nil {
			return nil, fmt.Errorf("reading chunk: %w", err)
		}

		if _, err := content.Write(chunk); err != nil {
			return nil, fmt.Errorf("writing to buffer: %w", err)
		}
	}

	return content.Bytes(), nil
}

// Delete marks chunks as deleted
func (s *ChunkStore) Delete(addresses [][]byte) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Record transaction
	if s.txStore != nil {
		if err := s.txStore.RecordAction(transaction.ActionDeleteContent, map[string]any{
			"addresses": addresses,
		}); err != nil {
			return fmt.Errorf("recording action: %w", err)
		}
	}

	for _, addr := range addresses {
		hashStr := hex.EncodeToString(addr)
		offset, exists := s.index[hashStr]
		if !exists {
			continue
		}

		// Decrement ref count
		if err := s.decrementRefCount(offset, hashStr); err != nil {
			return fmt.Errorf("updating ref count: %w", err)
		}
	}

	return nil
}

// ListChunks returns a list of all chunk addresses
func (s *ChunkStore) ListChunks() ([][]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	chunks := make([][]byte, 0, len(s.index))
	for hashStr := range s.index {
		addr, err := hex.DecodeString(hashStr)
		if err != nil {
			continue
		}
		chunks = append(chunks, addr)
	}

	return chunks, nil
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
		if err := s.readAt(&header, offset); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("reading header: %w", err)
		}

		// Verify magic number
		if header.Magic != ChunkMagic {
			offset += 4 // Skip ahead and try again
			continue
		}

		// Add to index if ref count > 0
		if header.RefCount > 0 {
			hashStr := hex.EncodeToString(header.Hash[:])
			s.index[hashStr] = offset
		}

		// Move to next chunk
		nextOffset := offset + int64(binary.Size(header)+int(header.Size))
		if nextOffset <= offset {
			break // Prevent infinite loop
		}
		offset = nextOffset
	}

	return nil
}

func (s *ChunkStore) writeChunk(data []byte, hash [32]byte) (int64, error) {
	// Get current position
	offset, err := s.file.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("seeking to end: %w", err)
	}

	// Create header
	header := ChunkHeader{
		Magic:    ChunkMagic,
		Size:     uint32(len(data)),
		Hash:     hash,
		RefCount: 1,
	}

	// Calculate header checksum
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, header); err != nil {
		return 0, fmt.Errorf("calculating checksum: %w", err)
	}
	header.Checksum = crc32(buf.Bytes())

	// Write header
	if err := s.writeAt(&header, offset); err != nil {
		return 0, fmt.Errorf("writing header: %w", err)
	}

	// Write data
	dataOffset := offset + int64(binary.Size(header))
	if _, err := s.file.WriteAt(data, dataOffset); err != nil {
		return 0, fmt.Errorf("writing data: %w", err)
	}

	// Sync to disk
	if err := s.file.Sync(); err != nil {
		return 0, fmt.Errorf("syncing to disk: %w", err)
	}

	return offset, nil
}

func (s *ChunkStore) readChunk(offset int64) ([]byte, error) {
	// Read header
	header := ChunkHeader{}
	if err := s.readAt(&header, offset); err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	// Verify magic number
	if header.Magic != ChunkMagic {
		return nil, fmt.Errorf("invalid chunk magic")
	}

	// Verify header checksum
	var buf bytes.Buffer
	origChecksum := header.Checksum
	header.Checksum = 0
	if err := binary.Write(&buf, binary.LittleEndian, header); err != nil {
		return nil, fmt.Errorf("calculating checksum: %w", err)
	}
	if crc32(buf.Bytes()) != origChecksum {
		return nil, fmt.Errorf("header checksum mismatch")
	}

	// Read data
	data := make([]byte, header.Size)
	dataOffset := offset + int64(binary.Size(header))
	if _, err := s.file.ReadAt(data, dataOffset); err != nil {
		return nil, fmt.Errorf("reading data: %w", err)
	}

	return data, nil
}

func (s *ChunkStore) incrementRefCount(offset int64) error {
	// Read header
	header := ChunkHeader{}
	if err := s.readAt(&header, offset); err != nil {
		return fmt.Errorf("reading header: %w", err)
	}

	// Verify magic number
	if header.Magic != ChunkMagic {
		return fmt.Errorf("invalid chunk magic")
	}

	// Update ref count
	header.RefCount++

	// Update checksum
	var buf bytes.Buffer
	header.Checksum = 0
	if err := binary.Write(&buf, binary.LittleEndian, header); err != nil {
		return fmt.Errorf("calculating checksum: %w", err)
	}
	header.Checksum = crc32(buf.Bytes())

	// Write header
	if err := s.writeAt(&header, offset); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	// Sync to disk
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("syncing to disk: %w", err)
	}

	return nil
}

func (s *ChunkStore) decrementRefCount(offset int64, hashStr string) error {
	// Read header
	header := ChunkHeader{}
	if err := s.readAt(&header, offset); err != nil {
		return fmt.Errorf("reading header: %w", err)
	}

	// Verify magic number
	if header.Magic != ChunkMagic {
		return fmt.Errorf("invalid chunk magic")
	}

	// Update ref count
	if header.RefCount > 0 {
		header.RefCount--
	}

	// Update checksum
	var buf bytes.Buffer
	header.Checksum = 0
	if err := binary.Write(&buf, binary.LittleEndian, header); err != nil {
		return fmt.Errorf("calculating checksum: %w", err)
	}
	header.Checksum = crc32(buf.Bytes())

	// Write header
	if err := s.writeAt(&header, offset); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	// Sync to disk
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("syncing to disk: %w", err)
	}

	// Remove from index if ref count is 0
	if header.RefCount == 0 {
		delete(s.index, hashStr)
	}

	return nil
}

func (s *ChunkStore) readAt(data interface{}, offset int64) error {
	// Create buffer for atomic read
	buf := make([]byte, binary.Size(data))
	if _, err := s.file.ReadAt(buf, offset); err != nil {
		return err
	}

	// Read from buffer
	return binary.Read(bytes.NewReader(buf), binary.LittleEndian, data)
}

func (s *ChunkStore) writeAt(data interface{}, offset int64) error {
	// Create buffer for atomic write
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, data); err != nil {
		return err
	}

	// Write buffer to file
	if _, err := s.file.WriteAt(buf.Bytes(), offset); err != nil {
		return err
	}

	return nil
}

// CRC32 calculates a checksum for chunk headers
func crc32(data []byte) uint32 {
	var checksum uint32
	for _, b := range data {
		checksum = (checksum << 8) ^ uint32(b)
	}
	return checksum
}
