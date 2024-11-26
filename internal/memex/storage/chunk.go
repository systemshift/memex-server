package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	// WindowSize is the size of the rolling hash window
	WindowSize = 64

	// MinChunkSize is the minimum chunk size (32KB)
	MinChunkSize = 32 * 1024

	// MaxChunkSize is the maximum chunk size (256KB)
	MaxChunkSize = 256 * 1024

	// Boundary is the rolling hash boundary value
	// When (hash % Boundary) == 0, we've found a chunk boundary
	Boundary = 32

	// ReadBufferSize is the size of the read buffer (64KB)
	ReadBufferSize = 64 * 1024
)

// RollingHash implements Rabin-Karp rolling hash
type RollingHash struct {
	window []byte
	pos    int
	hash   uint32
}

// NewRollingHash creates a new rolling hash
func NewRollingHash() *RollingHash {
	return &RollingHash{
		window: make([]byte, WindowSize),
		pos:    0,
		hash:   0,
	}
}

// Update updates the rolling hash with a new byte
func (r *RollingHash) Update(b byte) uint32 {
	// Remove old byte's contribution
	if r.pos >= WindowSize {
		r.hash = (r.hash - uint32(r.window[r.pos%WindowSize])) << 1
	}

	// Add new byte
	r.window[r.pos%WindowSize] = b
	r.hash = (r.hash + uint32(b))
	r.pos++

	return r.hash
}

// Reset resets the rolling hash
func (r *RollingHash) Reset() {
	r.pos = 0
	r.hash = 0
}

// Chunk represents a content chunk
type Chunk struct {
	Hash    string
	Content []byte
}

// ChunkContent splits content into chunks using content-defined chunking
func ChunkContent(content []byte) ([]Chunk, error) {
	var chunks []Chunk
	var chunk bytes.Buffer
	hash := NewRollingHash()

	fmt.Printf("Chunking content of size %d bytes\n", len(content))

	// Process content in larger blocks for efficiency
	for i := 0; i < len(content); i++ {
		b := content[i]
		chunk.WriteByte(b)
		h := hash.Update(b)

		// Check if we've found a chunk boundary
		if (h%Boundary == 0 && chunk.Len() >= MinChunkSize) || chunk.Len() >= MaxChunkSize {
			// Create chunk
			chunkContent := chunk.Bytes()
			chunkHash := sha256.Sum256(chunkContent)
			chunks = append(chunks, Chunk{
				Hash:    hex.EncodeToString(chunkHash[:]),
				Content: chunkContent,
			})

			fmt.Printf("Created chunk of size %d bytes\n", chunk.Len())

			// Reset for next chunk
			chunk.Reset()
			hash.Reset()
		}
	}

	// Handle remaining content
	if chunk.Len() > 0 {
		chunkContent := chunk.Bytes()
		chunkHash := sha256.Sum256(chunkContent)
		chunks = append(chunks, Chunk{
			Hash:    hex.EncodeToString(chunkHash[:]),
			Content: chunkContent,
		})
		fmt.Printf("Created final chunk of size %d bytes\n", chunk.Len())
	}

	fmt.Printf("Total chunks created: %d\n", len(chunks))
	return chunks, nil
}

// ReassembleContent reassembles content from chunks
func ReassembleContent(chunks []Chunk) []byte {
	var content bytes.Buffer
	content.Grow(len(chunks) * MinChunkSize) // Pre-allocate estimated size
	for _, chunk := range chunks {
		content.Write(chunk.Content)
	}
	return content.Bytes()
}

// VerifyChunk verifies a chunk's hash
func VerifyChunk(chunk Chunk) bool {
	hash := sha256.Sum256(chunk.Content)
	return hex.EncodeToString(hash[:]) == chunk.Hash
}
