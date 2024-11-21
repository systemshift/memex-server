package storage

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

// storeBlob stores content and returns its hash
func (s *MXStore) storeBlob(content []byte) (string, error) {
	// Calculate content hash
	hash := sha256.Sum256(content)

	// Check if content already exists
	hashStr := hex.EncodeToString(hash[:])
	for _, entry := range s.blobs {
		if string(entry.ID[:]) == string(hash[:]) {
			return hashStr, nil
		}
	}

	// Write content
	offset, err := s.file.Seek(0, io.SeekEnd)
	if err != nil {
		return "", fmt.Errorf("seeking to end: %w", err)
	}

	if err := binary.Write(s.file, binary.LittleEndian, uint32(len(content))); err != nil {
		return "", fmt.Errorf("writing content length: %w", err)
	}

	if _, err := s.file.Write(content); err != nil {
		return "", fmt.Errorf("writing content: %w", err)
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
		return "", fmt.Errorf("updating header: %w", err)
	}

	return hashStr, nil
}

// LoadBlob loads content by hash
func (s *MXStore) LoadBlob(hash string) ([]byte, error) {
	// Convert hash to bytes
	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return nil, fmt.Errorf("invalid hash: %w", err)
	}

	// Find blob in index
	var entry IndexEntry
	for _, e := range s.blobs {
		if string(e.ID[:]) == string(hashBytes) {
			entry = e
			break
		}
	}
	if entry.ID == [32]byte{} {
		return nil, fmt.Errorf("blob not found")
	}

	// Seek to content
	if _, err := s.file.Seek(int64(entry.Offset), io.SeekStart); err != nil {
		return nil, fmt.Errorf("seeking to blob: %w", err)
	}

	// Read content length
	var length uint32
	if err := binary.Read(s.file, binary.LittleEndian, &length); err != nil {
		return nil, fmt.Errorf("reading blob length: %w", err)
	}

	// Read content
	content := make([]byte, length)
	if _, err := io.ReadFull(s.file, content); err != nil {
		return nil, fmt.Errorf("reading blob content: %w", err)
	}

	return content, nil
}
