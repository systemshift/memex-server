package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// BinaryVersionStore implements version storage
type BinaryVersionStore struct {
	rootDir string
}

// NewVersionStore creates a new version store
func NewVersionStore(rootDir string) (*BinaryVersionStore, error) {
	versionsDir := filepath.Join(rootDir, "versions")
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		return nil, fmt.Errorf("creating versions directory: %w", err)
	}

	return &BinaryVersionStore{rootDir: rootDir}, nil
}

// getVersionPath returns the path for a version file
func (s *BinaryVersionStore) getVersionPath(id string, version int) string {
	return filepath.Join(s.rootDir, "versions", fmt.Sprintf("%s.%d.json", id, version))
}

// Store stores a version
func (s *BinaryVersionStore) Store(id string, version int, chunks []string) error {
	// Create version data
	data := map[string]interface{}{
		"version": version,
		"chunks":  chunks,
	}

	// Marshal version data
	versionData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling version data: %w", err)
	}

	// Write version file
	versionPath := s.getVersionPath(id, version)
	if err := os.WriteFile(versionPath, versionData, 0644); err != nil {
		return fmt.Errorf("writing version file: %w", err)
	}

	return nil
}

// Load retrieves chunks for a specific version
func (s *BinaryVersionStore) Load(id string, version int) ([]string, error) {
	// Read version file
	versionPath := s.getVersionPath(id, version)
	versionData, err := os.ReadFile(versionPath)
	if err != nil {
		return nil, fmt.Errorf("reading version file: %w", err)
	}

	// Parse version data
	var data map[string]interface{}
	if err := json.Unmarshal(versionData, &data); err != nil {
		return nil, fmt.Errorf("unmarshaling version data: %w", err)
	}

	// Extract chunks
	chunks := make([]string, 0)
	if chunkList, ok := data["chunks"].([]interface{}); ok {
		for _, chunk := range chunkList {
			if hash, ok := chunk.(string); ok {
				chunks = append(chunks, hash)
			}
		}
	}

	return chunks, nil
}

// Delete removes all versions for an object
func (s *BinaryVersionStore) Delete(id string) error {
	// Get all versions
	versions := s.List(id)

	// Delete each version file
	for _, version := range versions {
		versionPath := s.getVersionPath(id, version)
		if err := os.Remove(versionPath); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("removing version file: %w", err)
			}
		}
	}

	return nil
}

// List returns all versions for an object
func (s *BinaryVersionStore) List(id string) []int {
	var versions []int
	versionsDir := filepath.Join(s.rootDir, "versions")

	// Walk through versions directory
	filepath.Walk(versionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			// Parse version number from filename
			var version int
			_, err := fmt.Sscanf(filepath.Base(path), fmt.Sprintf("%s.%%d.json", id), &version)
			if err == nil {
				versions = append(versions, version)
			}
		}
		return nil
	})

	return versions
}
