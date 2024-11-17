package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
		filepath.Join(rootDir, "objects"),
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

// generateID creates a unique ID for content
func (s *BinaryStore) generateID(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// getObjectPath returns the path for an object ID
func (s *BinaryStore) getObjectPath(id string) string {
	// Use first 2 chars as directory name
	return filepath.Join(s.rootDir, "objects", id[:2], id[2:])
}

// getChunkPath returns the path for a chunk hash
func (s *BinaryStore) getChunkPath(hash string) string {
	// Use first 2 chars as directory name
	return filepath.Join(s.rootDir, "chunks", hash[:2], hash[2:])
}

// Store stores an object and returns its ID
func (s *BinaryStore) Store(obj core.Object) (string, error) {
	// Generate ID if not provided
	if obj.ID == "" {
		if len(obj.Content) > 0 {
			obj.ID = s.generateID(obj.Content)
		} else if len(obj.Chunks) > 0 {
			// Generate ID from concatenated chunk hashes
			hasher := sha256.New()
			for _, chunk := range obj.Chunks {
				hasher.Write([]byte(chunk))
			}
			obj.ID = hex.EncodeToString(hasher.Sum(nil))
		} else {
			return "", fmt.Errorf("object must have either Content or Chunks")
		}
	}

	// Create object directory
	objDir := filepath.Join(s.rootDir, "objects", obj.ID[:2])
	if err := os.MkdirAll(objDir, 0755); err != nil {
		return "", fmt.Errorf("creating object directory: %w", err)
	}

	// Write content if present
	if len(obj.Content) > 0 {
		objPath := s.getObjectPath(obj.ID)
		if err := os.WriteFile(objPath, obj.Content, 0644); err != nil {
			return "", fmt.Errorf("writing object content: %w", err)
		}
	}

	// Store metadata
	metaPath := filepath.Join(s.rootDir, "meta", obj.ID+".json")
	meta := map[string]interface{}{
		"type":     obj.Type,
		"version":  obj.Version,
		"created":  obj.Created.Format(time.RFC3339),
		"modified": obj.Modified.Format(time.RFC3339),
		"meta":     obj.Meta,
		"chunks":   obj.Chunks,
	}

	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return "", fmt.Errorf("writing metadata: %w", err)
	}

	return obj.ID, nil
}

// Load retrieves an object by ID
func (s *BinaryStore) Load(id string) (core.Object, error) {
	var obj core.Object
	obj.ID = id

	// Read metadata first
	metaPath := filepath.Join(s.rootDir, "meta", id+".json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return obj, fmt.Errorf("reading metadata: %w", err)
	}

	var meta map[string]interface{}
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return obj, fmt.Errorf("unmarshaling metadata: %w", err)
	}

	obj.Type = meta["type"].(string)
	obj.Version = int(meta["version"].(float64))

	// Parse timestamps
	if created, ok := meta["created"].(string); ok {
		obj.Created, _ = time.Parse(time.RFC3339, created)
	}
	if modified, ok := meta["modified"].(string); ok {
		obj.Modified, _ = time.Parse(time.RFC3339, modified)
	}

	// Handle metadata
	if metaMap, ok := meta["meta"].(map[string]interface{}); ok {
		obj.Meta = make(map[string]any)
		for k, v := range metaMap {
			// Convert interface{} arrays to []string if needed
			if arr, ok := v.([]interface{}); ok {
				strArr := make([]string, len(arr))
				for i, item := range arr {
					strArr[i] = fmt.Sprint(item)
				}
				obj.Meta[k] = strArr
			} else {
				obj.Meta[k] = v
			}
		}
	} else {
		obj.Meta = make(map[string]any)
	}

	// Handle chunks
	if chunks, ok := meta["chunks"].([]interface{}); ok {
		obj.Chunks = make([]string, len(chunks))
		for i, chunk := range chunks {
			obj.Chunks[i] = chunk.(string)
		}
	}

	// Read content if no chunks are present
	if len(obj.Chunks) == 0 {
		objPath := s.getObjectPath(id)
		content, err := os.ReadFile(objPath)
		if err != nil {
			return obj, fmt.Errorf("reading object content: %w", err)
		}
		obj.Content = content
	}

	return obj, nil
}

// StoreChunk stores a content chunk
func (s *BinaryStore) StoreChunk(hash string, content []byte) error {
	// Create chunk directory
	chunkDir := filepath.Join(s.rootDir, "chunks", hash[:2])
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return fmt.Errorf("creating chunk directory: %w", err)
	}

	// Write chunk
	chunkPath := s.getChunkPath(hash)
	if err := os.WriteFile(chunkPath, content, 0644); err != nil {
		return fmt.Errorf("writing chunk: %w", err)
	}

	return nil
}

// LoadChunk retrieves a content chunk
func (s *BinaryStore) LoadChunk(hash string) ([]byte, error) {
	chunkPath := s.getChunkPath(hash)
	content, err := os.ReadFile(chunkPath)
	if err != nil {
		return nil, fmt.Errorf("reading chunk: %w", err)
	}
	return content, nil
}

// Delete removes an object
func (s *BinaryStore) Delete(id string) error {
	// Remove content
	objPath := s.getObjectPath(id)
	if err := os.Remove(objPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("removing object content: %w", err)
		}
	}

	// Remove metadata
	metaPath := filepath.Join(s.rootDir, "meta", id+".json")
	if err := os.Remove(metaPath); err != nil {
		return fmt.Errorf("removing metadata: %w", err)
	}

	return nil
}

// List returns all object IDs
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
