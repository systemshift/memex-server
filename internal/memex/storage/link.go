package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"memex/internal/memex/core"
)

// BinaryLinkStore implements link storage
type BinaryLinkStore struct {
	rootDir string
}

// NewLinkStore creates a new link store
func NewLinkStore(rootDir string) (*BinaryLinkStore, error) {
	linksDir := filepath.Join(rootDir, "links")
	if err := os.MkdirAll(linksDir, 0755); err != nil {
		return nil, fmt.Errorf("creating links directory: %w", err)
	}

	return &BinaryLinkStore{rootDir: rootDir}, nil
}

// getLinkPath returns the path for a link file
func (s *BinaryLinkStore) getLinkPath(source, target string) string {
	// Use source-target as filename
	return filepath.Join(s.rootDir, "links", fmt.Sprintf("%s-%s.json", source, target))
}

// Store stores a link
func (s *BinaryLinkStore) Store(link core.Link) error {
	linkPath := s.getLinkPath(link.Source, link.Target)

	// Marshal link data
	data, err := json.MarshalIndent(link, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling link: %w", err)
	}

	// Write link file
	if err := os.WriteFile(linkPath, data, 0644); err != nil {
		return fmt.Errorf("writing link file: %w", err)
	}

	return nil
}

// Delete removes a link
func (s *BinaryLinkStore) Delete(source, target string) error {
	linkPath := s.getLinkPath(source, target)
	if err := os.Remove(linkPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("removing link file: %w", err)
		}
	}
	return nil
}

// GetBySource returns all links from a source
func (s *BinaryLinkStore) GetBySource(source string) []core.Link {
	var links []core.Link
	linksDir := filepath.Join(s.rootDir, "links")

	// Walk through links directory
	filepath.Walk(linksDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			// Check if file starts with source ID
			filename := filepath.Base(path)
			if len(filename) > len(source) && filename[:len(source)] == source {
				// Read and parse link
				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				var link core.Link
				if err := json.Unmarshal(data, &link); err != nil {
					return nil
				}
				links = append(links, link)
			}
		}
		return nil
	})

	return links
}

// GetByTarget returns all links to a target
func (s *BinaryLinkStore) GetByTarget(target string) []core.Link {
	var links []core.Link
	linksDir := filepath.Join(s.rootDir, "links")

	// Walk through links directory
	filepath.Walk(linksDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			// Read and parse link
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			var link core.Link
			if err := json.Unmarshal(data, &link); err != nil {
				return nil
			}
			// Check if this link points to our target
			if link.Target == target {
				links = append(links, link)
			}
		}
		return nil
	})

	return links
}

// GetAll returns all links
func (s *BinaryLinkStore) GetAll() []core.Link {
	var links []core.Link
	linksDir := filepath.Join(s.rootDir, "links")

	// Walk through links directory
	filepath.Walk(linksDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			// Read and parse link
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			var link core.Link
			if err := json.Unmarshal(data, &link); err != nil {
				return nil
			}
			links = append(links, link)
		}
		return nil
	})

	return links
}

// FindByType returns all links of a specific type
func (s *BinaryLinkStore) FindByType(linkType string) []core.Link {
	var links []core.Link
	linksDir := filepath.Join(s.rootDir, "links")

	// Walk through links directory
	filepath.Walk(linksDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			// Read and parse link
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			var link core.Link
			if err := json.Unmarshal(data, &link); err != nil {
				return nil
			}
			// Check if this link matches our type
			if link.Type == linkType {
				links = append(links, link)
			}
		}
		return nil
	})

	return links
}

// GetLinked returns all objects linked to the given object (in either direction)
func (s *BinaryLinkStore) GetLinked(id string) []string {
	var linked []string
	seen := make(map[string]bool)

	// Get outgoing links
	outgoing := s.GetBySource(id)
	for _, link := range outgoing {
		if !seen[link.Target] {
			linked = append(linked, link.Target)
			seen[link.Target] = true
		}
	}

	// Get incoming links
	incoming := s.GetByTarget(id)
	for _, link := range incoming {
		if !seen[link.Source] {
			linked = append(linked, link.Source)
			seen[link.Source] = true
		}
	}

	return linked
}
