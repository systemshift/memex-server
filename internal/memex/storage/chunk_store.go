package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// ChunkStore implements content-addressable storage for chunks
type ChunkStore struct {
	rootDir string
}

// NewChunkStore creates a new chunk store
func NewChunkStore(rootDir string) *ChunkStore {
	return &ChunkStore{rootDir: rootDir}
}

// Store stores a chunk and returns its hash
func (s *ChunkStore) Store(content []byte) (string, error) {
	// Calculate hash
	hash := hashBytes(content)

	// Create directory using first two chars of hash
	dir := filepath.Join(s.rootDir, hash[:2])
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating chunk directory: %w", err)
	}

	// Store chunk using rest of hash
	chunkPath := filepath.Join(dir, hash[2:])
	if err := os.WriteFile(chunkPath, content, 0644); err != nil {
		return "", fmt.Errorf("writing chunk file: %w", err)
	}

	return hash, nil
}

// Load retrieves a chunk by its hash
func (s *ChunkStore) Load(hash string) ([]byte, error) {
	if len(hash) < 3 {
		return nil, fmt.Errorf("invalid hash length")
	}

	chunkPath := filepath.Join(s.rootDir, hash[:2], hash[2:])
	content, err := os.ReadFile(chunkPath)
	if err != nil {
		return nil, fmt.Errorf("reading chunk file: %w", err)
	}

	return content, nil
}

// Delete removes a chunk
func (s *ChunkStore) Delete(hash string) error {
	if len(hash) < 3 {
		return fmt.Errorf("invalid hash length")
	}

	chunkPath := filepath.Join(s.rootDir, hash[:2], hash[2:])
	if err := os.Remove(chunkPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("removing chunk file: %w", err)
		}
	}
	return nil
}

// Has checks if a chunk exists
func (s *ChunkStore) Has(hash string) bool {
	if len(hash) < 3 {
		return false
	}

	chunkPath := filepath.Join(s.rootDir, hash[:2], hash[2:])
	_, err := os.Stat(chunkPath)
	return err == nil
}

// Dedupe removes duplicate chunks by comparing content hashes
func (s *ChunkStore) Dedupe() error {
	// Map to track unique content hashes
	seen := make(map[string]bool)

	// Walk through all chunks
	err := filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Read chunk content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading chunk %s: %w", path, err)
		}

		// Calculate hash
		hash := hashBytes(content)

		// If we've seen this content before, remove this chunk
		if seen[hash] {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("removing duplicate chunk %s: %w", path, err)
			}
		} else {
			seen[hash] = true
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("walking chunk directory: %w", err)
	}

	return nil
}

// hashBytes creates a hash from bytes
func hashBytes(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
