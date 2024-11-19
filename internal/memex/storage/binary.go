package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"memex/internal/memex/core"
)

// BinaryStore implements object storage using binary files
type BinaryStore struct {
	rootDir string
}

// NewBinaryStore creates a new binary storage
func NewBinaryStore(rootDir string) (*BinaryStore, error) {
	// Create required directories
	dirs := []string{
		filepath.Join(rootDir, "chunks"),
		filepath.Join(rootDir, "meta"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	return &BinaryStore{rootDir: rootDir}, nil
}

// getChunkPath returns the path for a chunk hash
func (s *BinaryStore) getChunkPath(hash string) string {
	// Use first 2 chars as directory name
	dir := filepath.Join(s.rootDir, "chunks", hash[:2])
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ""
	}
	return filepath.Join(dir, hash[2:])
}

// Store stores a node and returns its ID
func (s *BinaryStore) Store(node core.Node) (string, error) {
	// Generate ID if not provided
	if node.ID == "" {
		if len(node.Versions) > 0 && len(node.Versions[0].Chunks) > 0 {
			// Generate ID from first version's chunks
			hasher := sha256.New()
			for _, chunk := range node.Versions[0].Chunks {
				hasher.Write([]byte(chunk))
			}
			node.ID = hex.EncodeToString(hasher.Sum(nil)[:8])
		} else {
			return "", fmt.Errorf("node must have at least one version with chunks")
		}
	}

	// Store metadata
	metaPath := filepath.Join(s.rootDir, "meta", node.ID+".json")
	data, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return "", fmt.Errorf("writing metadata: %w", err)
	}

	return node.ID, nil
}

// Load retrieves a node by ID
func (s *BinaryStore) Load(id string) (core.Node, error) {
	// Read metadata
	metaPath := filepath.Join(s.rootDir, "meta", id+".json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return core.Node{}, fmt.Errorf("reading metadata: %w", err)
	}

	var node core.Node
	if err := json.Unmarshal(data, &node); err != nil {
		return core.Node{}, fmt.Errorf("parsing metadata: %w", err)
	}

	return node, nil
}

// StoreChunk stores a content chunk
func (s *BinaryStore) StoreChunk(hash string, content []byte) error {
	// Create chunk directory
	chunkPath := s.getChunkPath(hash)
	if chunkPath == "" {
		return fmt.Errorf("error creating chunk directory")
	}

	// Write chunk
	if err := os.WriteFile(chunkPath, content, 0644); err != nil {
		return fmt.Errorf("writing chunk: %w", err)
	}

	return nil
}

// LoadChunk retrieves a content chunk
func (s *BinaryStore) LoadChunk(hash string) ([]byte, error) {
	chunkPath := s.getChunkPath(hash)
	if chunkPath == "" {
		return nil, fmt.Errorf("error getting chunk path")
	}

	content, err := os.ReadFile(chunkPath)
	if err != nil {
		return nil, fmt.Errorf("reading chunk: %w", err)
	}
	return content, nil
}

// DeleteChunk removes a chunk
func (s *BinaryStore) DeleteChunk(hash string) error {
	chunkPath := s.getChunkPath(hash)
	if chunkPath == "" {
		return fmt.Errorf("error getting chunk path")
	}

	if err := os.Remove(chunkPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("removing chunk: %w", err)
		}
	}
	return nil
}

// Delete removes a node
func (s *BinaryStore) Delete(id string) error {
	// Remove metadata
	metaPath := filepath.Join(s.rootDir, "meta", id+".json")
	if err := os.Remove(metaPath); err != nil {
		return fmt.Errorf("removing metadata: %w", err)
	}

	return nil
}

// List returns all node IDs
func (s *BinaryStore) List() []string {
	var ids []string
	metaDir := filepath.Join(s.rootDir, "meta")

	// Walk through meta directory
	filepath.Walk(metaDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			// Remove .json extension to get ID
			id := filepath.Base(path[:len(path)-5])
			ids = append(ids, id)
		}
		return nil
	})

	return ids
}
