package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
		filepath.Join(rootDir, "objects"),
		filepath.Join(rootDir, "versions"),
		filepath.Join(rootDir, "meta"),
		filepath.Join(rootDir, "links"),
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

// Store stores an object and returns its ID
func (s *BinaryStore) Store(obj core.Object) (string, error) {
	// Generate ID if not provided
	if obj.ID == "" {
		obj.ID = s.generateID(obj.Content)
	}

	// Create object directory
	objDir := filepath.Join(s.rootDir, "objects", obj.ID[:2])
	if err := os.MkdirAll(objDir, 0755); err != nil {
		return "", fmt.Errorf("creating object directory: %w", err)
	}

	// Write content
	objPath := s.getObjectPath(obj.ID)
	if err := os.WriteFile(objPath, obj.Content, 0644); err != nil {
		return "", fmt.Errorf("writing object content: %w", err)
	}

	// Initialize metadata if nil
	if obj.Meta == nil {
		obj.Meta = make(map[string]any)
	}

	// Store metadata
	metaPath := filepath.Join(s.rootDir, "meta", obj.ID+".json")
	meta := map[string]interface{}{
		"type":     obj.Type,
		"version":  obj.Version,
		"created":  obj.Created,
		"modified": obj.Modified,
		"meta":     obj.Meta,
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

	// Read content
	objPath := s.getObjectPath(id)
	content, err := os.ReadFile(objPath)
	if err != nil {
		return obj, fmt.Errorf("reading object content: %w", err)
	}
	obj.Content = content

	// Read metadata
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

	// Handle potentially nil metadata
	if metaMap, ok := meta["meta"].(map[string]interface{}); ok {
		obj.Meta = metaMap
	} else {
		obj.Meta = make(map[string]any)
	}

	return obj, nil
}

// Delete removes an object
func (s *BinaryStore) Delete(id string) error {
	// Remove content
	objPath := s.getObjectPath(id)
	if err := os.Remove(objPath); err != nil {
		return fmt.Errorf("removing object content: %w", err)
	}

	// Remove metadata
	metaPath := filepath.Join(s.rootDir, "meta", id+".json")
	if err := os.Remove(metaPath); err != nil {
		return fmt.Errorf("removing metadata: %w", err)
	}

	// Remove versions
	versionsDir := filepath.Join(s.rootDir, "versions", id)
	if err := os.RemoveAll(versionsDir); err != nil {
		return fmt.Errorf("removing versions: %w", err)
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

// Copy creates a copy of the content
func (s *BinaryStore) Copy(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying content: %w", err)
	}

	return nil
}
