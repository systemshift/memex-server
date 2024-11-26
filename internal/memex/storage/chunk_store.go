package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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

	// Write to temporary file first
	path := filepath.Join(dir, hash[2:])
	tempPath := path + ".tmp"

	if err := os.WriteFile(tempPath, content, 0644); err != nil {
		return "", fmt.Errorf("writing temporary chunk file: %w", err)
	}

	// Verify content was written correctly
	written, err := os.ReadFile(tempPath)
	if err != nil {
		os.Remove(tempPath) // Clean up temp file
		return "", fmt.Errorf("verifying chunk file: %w", err)
	}

	writtenHash := s.hashContent(written)
	if writtenHash != hash {
		os.Remove(tempPath) // Clean up temp file
		return "", fmt.Errorf("chunk content verification failed")
	}

	// Atomically rename temp file to final location
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return "", fmt.Errorf("moving chunk file: %w", err)
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

	// Verify hash exists in refs
	if s.refs[hash] == 0 {
		return nil, fmt.Errorf("chunk not found")
	}

	path := filepath.Join(s.path, hash[:2], hash[2:])
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading chunk file: %w", err)
	}

	// Verify content hash matches
	readHash := s.hashContent(content)
	if readHash != hash {
		return nil, fmt.Errorf("chunk content verification failed")
	}

	return content, nil
}

// Delete removes content by hash
func (s *ChunkStore) Delete(hash string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Verify hash exists
	if s.refs[hash] == 0 {
		return fmt.Errorf("chunk not found")
	}

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

	// Verify content hash before deletion
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading chunk before deletion: %w", err)
	}

	if err == nil {
		readHash := s.hashContent(content)
		if readHash != hash {
			return fmt.Errorf("chunk content verification failed before deletion")
		}
	}

	// Delete the file
	if err := os.Remove(path); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("removing chunk file: %w", err)
		}
	}

	// Remove reference count
	delete(s.refs, hash)

	// Try to remove empty directory
	dir := filepath.Join(s.path, hash[:2])
	if empty, _ := isDirEmpty(dir); empty {
		os.Remove(dir) // Ignore error as directory may contain other files
	}

	return nil
}

// Has checks if content exists
func (s *ChunkStore) Has(hash string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.refs[hash] == 0 {
		return false
	}

	// Verify file exists and content matches
	if len(hash) < 2 {
		return false
	}

	path := filepath.Join(s.path, hash[:2], hash[2:])
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	readHash := s.hashContent(content)
	return readHash == hash
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

		// Skip temporary files
		if filepath.Ext(path) == ".tmp" {
			os.Remove(path) // Clean up any leftover temp files
			return nil
		}

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

			// Write to temp file first
			tempPath := newPath + ".tmp"
			if err := os.WriteFile(tempPath, content, 0644); err != nil {
				return fmt.Errorf("writing temporary chunk file: %w", err)
			}

			// Atomically rename
			if err := os.Rename(tempPath, newPath); err != nil {
				os.Remove(tempPath)
				return fmt.Errorf("moving chunk: %w", err)
			}

			// Remove old file
			os.Remove(path)

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

// isDirEmpty checks if a directory is empty
func isDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}
