package chunk

import (
	"crypto/sha256"
	"fmt"

	"memex/internal/memex/storage/common"
)

// Verifier handles chunk verification
type Verifier struct {
	store *Store
}

// NewVerifier creates a new chunk verifier
func NewVerifier(store *Store) *Verifier {
	return &Verifier{
		store: store,
	}
}

// VerifyChunk verifies a chunk's integrity
func (v *Verifier) VerifyChunk(hash string) error {
	// Get chunk content
	content, err := v.store.Get(hash)
	if err != nil {
		return fmt.Errorf("getting chunk: %w", err)
	}

	// Verify hash
	actualHash := sha256.Sum256(content)
	if fmt.Sprintf("%x", actualHash) != hash {
		return fmt.Errorf("hash mismatch: got %x want %s", actualHash, hash)
	}

	// Verify checksum
	checksum := common.CalculateChecksum(content)
	if !common.ValidateChecksum(content, checksum) {
		return fmt.Errorf("checksum mismatch")
	}

	return nil
}

// VerifyIndex verifies the chunk index integrity
func (v *Verifier) VerifyIndex() error {
	// Get all index entries
	entries := v.store.index

	// Verify each entry
	for _, entry := range entries {
		// Skip deleted entries
		if entry.Flags&FlagDeleted != 0 {
			continue
		}

		// Verify chunk exists
		hash := fmt.Sprintf("%x", entry.ID)
		if err := v.VerifyChunk(hash); err != nil {
			return fmt.Errorf("verifying chunk %s: %w", hash, err)
		}
	}

	return nil
}

// VerifyChunkSize verifies a chunk's size is within limits
func (v *Verifier) VerifyChunkSize(content []byte) error {
	if len(content) > MaxChunkSize {
		return fmt.Errorf("chunk too large: %d bytes (max %d)", len(content), MaxChunkSize)
	}
	return nil
}

// Constants for chunk verification
const (
	MaxChunkSize = 4096 // Maximum chunk size in bytes
)
