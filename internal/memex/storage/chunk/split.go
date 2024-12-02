package chunk

import (
	"crypto/sha256"
	"fmt"

	"memex/internal/memex/storage/common"
)

// Split splits content into chunks
func Split(content []byte) ([]ChunkData, error) {
	if len(content) == 0 {
		return nil, fmt.Errorf("content cannot be empty")
	}

	// Calculate number of chunks needed
	numChunks := (len(content) + MaxChunkSize - 1) / MaxChunkSize
	chunks := make([]ChunkData, 0, numChunks)

	// Split content into fixed-size chunks
	for i := 0; i < len(content); i += MaxChunkSize {
		end := i + MaxChunkSize
		if end > len(content) {
			end = len(content)
		}

		chunkContent := content[i:end]
		hash := sha256.Sum256(chunkContent)
		checksum := common.CalculateChecksum(chunkContent)

		chunks = append(chunks, ChunkData{
			Content:  chunkContent,
			Hash:     hash,
			Length:   uint32(len(chunkContent)),
			Checksum: checksum,
		})
	}

	fmt.Printf("Chunking content of size %d bytes\n", len(content))
	fmt.Printf("Created %d fixed-size chunks\n", len(chunks))

	// Verify total size
	totalSize := 0
	for _, chunk := range chunks {
		totalSize += int(chunk.Length)
	}
	if totalSize != len(content) {
		return nil, fmt.Errorf("chunk size mismatch: got %d want %d", totalSize, len(content))
	}

	return chunks, nil
}

// Join joins chunks back into the original content
func Join(chunks []ChunkData) []byte {
	// Calculate total size
	totalSize := 0
	for _, chunk := range chunks {
		totalSize += int(chunk.Length)
	}

	// Allocate buffer
	content := make([]byte, 0, totalSize)

	// Join chunks
	for _, chunk := range chunks {
		content = append(content, chunk.Content...)
	}

	return content
}

// Verify verifies a chunk's hash and checksum
func Verify(chunk ChunkData) bool {
	hash := sha256.Sum256(chunk.Content)
	checksum := common.CalculateChecksum(chunk.Content)
	return hash == chunk.Hash && checksum == chunk.Checksum
}
