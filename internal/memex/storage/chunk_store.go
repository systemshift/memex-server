package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ChunkStore implements content-addressable storage
type ChunkStore struct {
	path  string
	mutex sync.RWMutex
	refs  map[string]int // Reference counts for chunks
}

// NewChunkStore creates a new chunk store
func NewChunkStore(path string) *ChunkStore {
	return &ChunkStore{
		path: path,
		refs: make(map[string]int),
	}
}

// Store stores content and returns its hash
func (s *ChunkStore) Store(content []byte) (string, error) {
	// Calculate content hash
	hash := s.hashContent(content)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if content already exists
	if s.refs[hash] > 0 {
		s.refs[hash]++
		return hash, nil
	}

	// Create chunk directory if needed
	if len(hash) < 2 {
		return "", fmt.Errorf("hash too short")
	}
	dir := filepath.Join(s.path, hash[:2])
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating chunk directory: %w", err)
	}

	// Write content file
	path := filepath.Join(dir, hash[2:])
	if err := os.WriteFile(path, content, 0644); err != nil {
		return "", fmt.Errorf("writing chunk file: %w", err)
	}

	// Initialize reference count
	s.refs[hash] = 1

	return hash, nil
}

// Load retrieves content by hash
func (s *ChunkStore) Load(hash string) ([]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if len(hash) < 2 {
		return nil, fmt.Errorf("hash too short")
	}

	path := filepath.Join(s.path, hash[:2], hash[2:])
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading chunk file: %w", err)
	}

	return content, nil
}

// Delete removes content by hash
func (s *ChunkStore) Delete(hash string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Decrease reference count
	if s.refs[hash] > 1 {
		s.refs[hash]--
		return nil
	}

	// Delete file if no more references
	if len(hash) < 2 {
		return fmt.Errorf("hash too short")
	}

	path := filepath.Join(s.path, hash[:2], hash[2:])
	if err := os.Remove(path); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("removing chunk file: %w", err)
		}
	}

	// Remove reference count
	delete(s.refs, hash)

	return nil
}

// Has checks if content exists
func (s *ChunkStore) Has(hash string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.refs[hash] > 0
}

// Dedupe runs deduplication on all stored chunks
func (s *ChunkStore) Dedupe() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Reset reference counts
	s.refs = make(map[string]int)

	// Walk chunk directories
	err := filepath.Walk(s.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get chunk hash from path
		relPath, err := filepath.Rel(s.path, path)
		if err != nil {
			return fmt.Errorf("getting relative path: %w", err)
		}

		dir := filepath.Dir(relPath)
		base := filepath.Base(relPath)
		hash := dir + base

		// Read chunk content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading chunk: %w", err)
		}

		// Verify hash matches content
		calcHash := s.hashContent(content)
		if calcHash != hash {
			// Move chunk to correct location
			newDir := filepath.Join(s.path, calcHash[:2])
			if err := os.MkdirAll(newDir, 0755); err != nil {
				return fmt.Errorf("creating chunk directory: %w", err)
			}

			newPath := filepath.Join(newDir, calcHash[2:])
			if err := os.Rename(path, newPath); err != nil {
				return fmt.Errorf("moving chunk: %w", err)
			}

			hash = calcHash
		}

		// Update reference count
		s.refs[hash]++

		return nil
	})

	if err != nil {
		return fmt.Errorf("walking chunks: %w", err)
	}

	return nil
}

// hashContent calculates SHA-256 hash of content
func (s *ChunkStore) hashContent(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
