package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

// HasBlob checks if a blob exists
func (s *MXStore) HasBlob(hash string) bool {
	// Convert hash string to bytes
	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return false
	}

	// Check if hash exists in blob index
	for _, entry := range s.blobs {
		if entry.ID == [32]byte(hashBytes) {
			return true
		}
	}

	return false
}

// StoreBlob stores content and returns its hash
func (s *MXStore) StoreBlob(content []byte) error {
	// Calculate hash
	hash := sha256.Sum256(content)

	// Check if already exists
	for _, entry := range s.blobs {
		if entry.ID == hash {
			return nil // Already stored
		}
	}

	// Write blob data
	offset, err := s.file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seeking to end: %w", err)
	}

	// Write content
	if _, err := s.file.Write(content); err != nil {
		return fmt.Errorf("writing content: %w", err)
	}

	// Add to index
	s.blobs = append(s.blobs, IndexEntry{
		ID:     hash,
		Offset: uint64(offset),
		Length: uint32(len(content)),
	})

	// Update header
	s.header.BlobCount++
	s.header.Modified = time.Now()
	if err := s.writeHeader(); err != nil {
		return fmt.Errorf("updating header: %w", err)
	}

	return nil
}

// LoadBlob loads content by hash
func (s *MXStore) LoadBlob(hash string) ([]byte, error) {
	// Convert hash string to bytes
	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return nil, fmt.Errorf("invalid hash: %w", err)
	}

	// Find blob in index
	var entry IndexEntry
	for _, e := range s.blobs {
		if e.ID == [32]byte(hashBytes) {
			entry = e
			break
		}
	}
	if entry.ID == [32]byte{} {
		return nil, fmt.Errorf("blob not found")
	}

	// Seek to blob data
	if _, err := s.file.Seek(int64(entry.Offset), io.SeekStart); err != nil {
		return nil, fmt.Errorf("seeking to blob: %w", err)
	}

	// Read content
	content := make([]byte, entry.Length)
	if _, err := io.ReadFull(s.file, content); err != nil {
		return nil, fmt.Errorf("reading blob: %w", err)
	}

	return content, nil
}
