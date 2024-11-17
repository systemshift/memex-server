package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

const (
	// WindowSize is the size of the rolling hash window
	WindowSize = 64

	// MinChunkSize is the minimum chunk size
	MinChunkSize = 2048

	// MaxChunkSize is the maximum chunk size
	MaxChunkSize = 8192

	// Boundary is the rolling hash boundary value
	// When (hash % Boundary) == 0, we've found a chunk boundary
	Boundary = 4096
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

	reader := bytes.NewReader(content)
	buf := make([]byte, 1)

	for {
		n, err := reader.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading content: %w", err)
		}
		if n == 0 {
			continue
		}

		chunk.Write(buf)
		h := hash.Update(buf[0])

		// Check if we've found a chunk boundary
		if (h%Boundary == 0 && chunk.Len() >= MinChunkSize) || chunk.Len() >= MaxChunkSize {
			// Create chunk
			chunkContent := chunk.Bytes()
			chunkHash := sha256.Sum256(chunkContent)
			chunks = append(chunks, Chunk{
				Hash:    hex.EncodeToString(chunkHash[:]),
				Content: chunkContent,
			})

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
	}

	return chunks, nil
}

// ReassembleContent reassembles content from chunks
func ReassembleContent(chunks []Chunk) []byte {
	var content bytes.Buffer
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
