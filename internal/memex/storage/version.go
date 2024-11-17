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

// Version represents a version of an object
type Version struct {
	ID      string   // Object ID
	Number  int      // Version number
	Chunks  []string // Chunk hashes for this version
	Message string   // Version message
}

// getVersionPath returns the path for a version file
func (s *BinaryVersionStore) getVersionPath(id string, version int) string {
	return filepath.Join(s.rootDir, "versions", fmt.Sprintf("%s.%d.json", id, version))
}

// Store stores a version
func (s *BinaryVersionStore) Store(id string, version int, chunks []string) error {
	v := Version{
		ID:      id,
		Number:  version,
		Chunks:  chunks,
		Message: fmt.Sprintf("Version %d", version),
	}

	// Marshal version data
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling version: %w", err)
	}

	// Write version file
	versionPath := s.getVersionPath(id, version)
	if err := os.WriteFile(versionPath, data, 0644); err != nil {
		return fmt.Errorf("writing version file: %w", err)
	}

	return nil
}

// Load retrieves chunk hashes for a specific version
func (s *BinaryVersionStore) Load(id string, version int) ([]string, error) {
	versionPath := s.getVersionPath(id, version)
	data, err := os.ReadFile(versionPath)
	if err != nil {
		return nil, fmt.Errorf("reading version file: %w", err)
	}

	var v Version
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("unmarshaling version: %w", err)
	}

	return v.Chunks, nil
}

// List returns all versions of an object
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
			var v Version
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			if err := json.Unmarshal(data, &v); err != nil {
				return nil
			}
			if v.ID == id {
				versions = append(versions, v.Number)
			}
		}
		return nil
	})

	return versions
}
