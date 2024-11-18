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
func (s *BinaryLinkStore) getLinkPath(source, target, linkType string, sourceChunk, targetChunk string) string {
	// Use source-target-type-chunks as filename
	filename := fmt.Sprintf("%s-%s-%s", source, target, linkType)
	if sourceChunk != "" && targetChunk != "" {
		filename += fmt.Sprintf("-chunk-%s-%s", sourceChunk[:8], targetChunk[:8])
	}
	return filepath.Join(s.rootDir, "links", filename+".json")
}

// Store stores a link
func (s *BinaryLinkStore) Store(link core.Link) error {
	linkPath := s.getLinkPath(link.Source, link.Target, link.Type, link.SourceChunk, link.TargetChunk)

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

// Delete removes links involving an object
func (s *BinaryLinkStore) Delete(source, target string) error {
	linksDir := filepath.Join(s.rootDir, "links")

	// Walk through links directory
	return filepath.Walk(linksDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			// Read link file
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			var link core.Link
			if err := json.Unmarshal(data, &link); err != nil {
				return nil
			}

			// Delete if link involves the specified object(s)
			if (source != "" && link.Source == source) ||
				(target != "" && link.Target == target) {
				if err := os.Remove(path); err != nil {
					if !os.IsNotExist(err) {
						return fmt.Errorf("removing link file: %w", err)
					}
				}
			}
		}
		return nil
	})
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
			// Read link file
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			var link core.Link
			if err := json.Unmarshal(data, &link); err != nil {
				return nil
			}

			// Add if this link is from our source
			if link.Source == source {
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
			// Read link file
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			var link core.Link
			if err := json.Unmarshal(data, &link); err != nil {
				return nil
			}

			// Add if this link points to our target
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
			// Read link file
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
			// Read link file
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			var link core.Link
			if err := json.Unmarshal(data, &link); err != nil {
				return nil
			}

			// Add if this link matches our type
			if link.Type == linkType {
				links = append(links, link)
			}
		}
		return nil
	})

	return links
}
