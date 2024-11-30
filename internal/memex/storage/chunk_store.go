package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
)

// ChunkStore manages content chunks
type ChunkStore struct {
	store *MXStore // Parent store for file access
}

// ChunkEntry represents a chunk in the store
type ChunkEntry struct {
	Hash   [32]byte // SHA-256 hash of content
	Offset uint64   // File offset to chunk data
	Length uint32   // Length of chunk data
}

// NewChunkStore creates a new chunk store
func NewChunkStore(store *MXStore) *ChunkStore {
	return &ChunkStore{
		store: store,
	}
}

// Store adds a chunk to the store
func (s *ChunkStore) Store(content []byte) (string, error) {
	// Calculate hash
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])

	// Check if chunk already exists
	if _, err := s.Get(hashStr); err == nil {
		// Chunk already exists
		return hashStr, nil
	}

	// Begin transaction
	tx, err := s.store.beginTransaction()
	if err != nil {
		return "", fmt.Errorf("beginning transaction: %w", err)
	}

	// Write chunk data
	offset, err := tx.write(content)
	if err != nil {
		tx.rollback()
		return "", fmt.Errorf("writing chunk: %w", err)
	}

	// Create chunk entry
	entry := ChunkEntry{
		Hash:   hash,
		Offset: offset,
		Length: uint32(len(content)),
	}

	// Add to index
	tx.addIndex(IndexEntry{
		ID:     entry.Hash,
		Offset: entry.Offset,
		Length: entry.Length,
	})

	// Commit transaction
	if err := tx.commit(); err != nil {
		return "", fmt.Errorf("committing transaction: %w", err)
	}

	return hashStr, nil
}

// Get retrieves a chunk by its hash
func (s *ChunkStore) Get(hash string) ([]byte, error) {
	// Decode hash
	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return nil, fmt.Errorf("invalid hash: %w", err)
	}

	// Find chunk in index
	var entry ChunkEntry
	copy(entry.Hash[:], hashBytes)

	// Find chunk entry
	for _, idx := range s.store.nodes {
		if bytes.Equal(idx.ID[:], entry.Hash[:]) {
			entry.Offset = idx.Offset
			entry.Length = idx.Length
			break
		}
	}

	if entry.Length == 0 {
		return nil, fmt.Errorf("chunk not found: %s", hash)
	}

	// Seek to chunk data
	if _, err := s.store.seek(int64(entry.Offset), io.SeekStart); err != nil {
		return nil, fmt.Errorf("seeking to chunk: %w", err)
	}

	// Read length prefix
	var length uint32
	if err := binary.Read(s.store.file, binary.LittleEndian, &length); err != nil {
		return nil, fmt.Errorf("reading length prefix: %w", err)
	}

	// Read chunk data
	data := make([]byte, length)
	if _, err := io.ReadFull(s.store.file, data); err != nil {
		return nil, fmt.Errorf("reading chunk data: %w", err)
	}

	return data, nil
}

// Delete removes a chunk from the store
func (s *ChunkStore) Delete(hash string) error {
	// Chunks are never actually deleted, they remain in the file
	// This is fine because:
	// 1. They might be referenced by other nodes
	// 2. Content addressing means same content = same hash
	// 3. Space can be reclaimed during compaction
	return nil
}

// ChunkContent splits content into chunks
func ChunkContent(content []byte) ([]Chunk, error) {
	// Use fixed-size chunking
	chunkSize := 4096 // 4KB chunks
	if len(content) < chunkSize {
		chunkSize = 1024 // Use 1KB chunks for small files
	}

	fmt.Printf("Chunking content of size %d bytes\n", len(content))

	var chunks []Chunk
	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}

		chunk := content[i:end]
		hash := sha256.Sum256(chunk)
		chunks = append(chunks, Chunk{
			Content: chunk,
			Hash:    hex.EncodeToString(hash[:]),
		})
	}

	fmt.Printf("Created %d fixed-size chunks\n", len(chunks))
	return chunks, nil
}
