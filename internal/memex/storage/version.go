package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// VersionInfo stores metadata about a version
type VersionInfo struct {
	Version  int       `json:"version"`
	Created  time.Time `json:"created"`
	Message  string    `json:"message"`
	Previous string    `json:"previous"` // ID of previous version
}

// BinaryVersionStore implements version tracking
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

// Store stores a version of an object
func (s *BinaryVersionStore) Store(id string, version int, content []byte, message string) error {
	// Create version directory
	versionDir := filepath.Join(s.rootDir, "versions", id)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return fmt.Errorf("creating version directory: %w", err)
	}

	// Store content
	contentPath := filepath.Join(versionDir, fmt.Sprintf("v%d", version))
	if err := os.WriteFile(contentPath, content, 0644); err != nil {
		return fmt.Errorf("writing version content: %w", err)
	}

	// Get previous version info
	var previousID string
	if version > 1 {
		prevVersions, err := s.List(id)
		if err != nil {
			return fmt.Errorf("getting previous versions: %w", err)
		}
		if len(prevVersions) > 0 {
			previousID = prevVersions[len(prevVersions)-1]
		}
	}

	// Store version info
	info := VersionInfo{
		Version:  version,
		Created:  time.Now(),
		Message:  message,
		Previous: previousID,
	}

	infoPath := filepath.Join(versionDir, fmt.Sprintf("v%d.json", version))
	infoData, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling version info: %w", err)
	}

	if err := os.WriteFile(infoPath, infoData, 0644); err != nil {
		return fmt.Errorf("writing version info: %w", err)
	}

	return nil
}

// Load retrieves a specific version
func (s *BinaryVersionStore) Load(id string, version int) ([]byte, error) {
	contentPath := filepath.Join(s.rootDir, "versions", id, fmt.Sprintf("v%d", version))
	content, err := os.ReadFile(contentPath)
	if err != nil {
		return nil, fmt.Errorf("reading version content: %w", err)
	}

	return content, nil
}

// GetInfo retrieves version information
func (s *BinaryVersionStore) GetInfo(id string, version int) (VersionInfo, error) {
	var info VersionInfo

	infoPath := filepath.Join(s.rootDir, "versions", id, fmt.Sprintf("v%d.json", version))
	infoData, err := os.ReadFile(infoPath)
	if err != nil {
		return info, fmt.Errorf("reading version info: %w", err)
	}

	if err := json.Unmarshal(infoData, &info); err != nil {
		return info, fmt.Errorf("unmarshaling version info: %w", err)
	}

	return info, nil
}

// List returns all versions of an object
func (s *BinaryVersionStore) List(id string) ([]string, error) {
	var versions []string
	versionDir := filepath.Join(s.rootDir, "versions", id)

	// Check if directory exists
	if _, err := os.Stat(versionDir); os.IsNotExist(err) {
		return versions, nil
	}

	// Walk through version directory
	err := filepath.Walk(versionDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Only look at content files (not .json metadata)
		if !info.IsDir() && filepath.Ext(path) == "" {
			versions = append(versions, filepath.Base(path))
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking version directory: %w", err)
	}

	// Sort versions
	sort.Strings(versions)
	return versions, nil
}

// GetHistory returns the version history chain
func (s *BinaryVersionStore) GetHistory(id string) ([]VersionInfo, error) {
	var history []VersionInfo

	versions, err := s.List(id)
	if err != nil {
		return nil, fmt.Errorf("listing versions: %w", err)
	}

	for _, v := range versions {
		// Extract version number from filename (v1, v2, etc)
		var version int
		fmt.Sscanf(v, "v%d", &version)

		info, err := s.GetInfo(id, version)
		if err != nil {
			return nil, fmt.Errorf("getting version info: %w", err)
		}

		history = append(history, info)
	}

	return history, nil
}

// Delete removes all versions of an object
func (s *BinaryVersionStore) Delete(id string) error {
	versionDir := filepath.Join(s.rootDir, "versions", id)
	if err := os.RemoveAll(versionDir); err != nil {
		return fmt.Errorf("removing version directory: %w", err)
	}
	return nil
}
