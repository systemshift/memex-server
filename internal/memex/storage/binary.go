package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

// resolvePartialID attempts to find the full ID from a partial ID
func (s *BinaryStore) resolvePartialID(id string) (string, error) {
	if len(id) >= 64 {
		return id, nil
	}

	prefix := id[:2]
	rest := id[2:]

	// Check objects directory for full ID
	dirPath := filepath.Join(s.rootDir, "objects", prefix)
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("reading objects directory: %w", err)
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), rest) {
			return prefix + file.Name(), nil
		}
	}

	return "", fmt.Errorf("no matching object found for ID: %s", id)
}

// getObjectPath returns the path for an object ID
func (s *BinaryStore) getObjectPath(id string) string {
	// Use first 2 chars as directory name, rest as filename
	prefix := id[:2]
	filename := id[2:]

	if len(filename) < 64 {
		// If we only have a short ID, try to load the full ID from metadata
		dirPath := filepath.Join(s.rootDir, "objects", prefix)
		files, err := os.ReadDir(dirPath)
		if err == nil {
			for _, file := range files {
				if strings.HasPrefix(file.Name(), filename) {
					filename = file.Name()
					break
				}
			}
		}
	}

	return filepath.Join(s.rootDir, "objects", prefix, filename)
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

	// Try to resolve full ID
	fullID, err := s.resolvePartialID(id)
	if err != nil {
		return obj, fmt.Errorf("resolving ID: %w", err)
	}
	obj.ID = fullID

	// Read content
	objPath := s.getObjectPath(fullID)
	content, err := os.ReadFile(objPath)
	if err != nil {
		return obj, fmt.Errorf("reading object content: %w", err)
	}
	obj.Content = content

	// Read metadata
	metaPath := filepath.Join(s.rootDir, "meta", fullID+".json")
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
	// Try to resolve full ID
	fullID, err := s.resolvePartialID(id)
	if err != nil {
		return fmt.Errorf("resolving ID: %w", err)
	}

	// Remove content
	objPath := s.getObjectPath(fullID)
	if err := os.Remove(objPath); err != nil {
		return fmt.Errorf("removing object content: %w", err)
	}

	// Remove metadata
	metaPath := filepath.Join(s.rootDir, "meta", fullID+".json")
	if err := os.Remove(metaPath); err != nil {
		return fmt.Errorf("removing metadata: %w", err)
	}

	// Remove versions
	versionsDir := filepath.Join(s.rootDir, "versions", fullID)
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
