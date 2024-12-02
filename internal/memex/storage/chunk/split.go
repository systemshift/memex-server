package chunk

import (
	"crypto/sha256"
	"fmt"

	"memex/internal/memex/storage/common"
)

// Split splits content into chunks
func Split(content []byte) ([]ChunkData, error) {
	// Use fixed-size chunking
	chunkSize := MaxChunkSize // 4KB chunks
	if len(content) < chunkSize {
		chunkSize = len(content) // Use content size for small files
	}

	var chunks []ChunkData
	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
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
	return chunks, nil
}
