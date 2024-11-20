package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"memex/internal/memex/core"
)

// AddLink creates a link between nodes
func (s *DAGStore) AddLink(source, target, linkType string, meta map[string]any) error {
	// Create link
	link := core.Link{
		Source: source,
		Target: target,
		Type:   linkType,
		Meta:   meta,
	}

	// Store link
	data, err := json.MarshalIndent(link, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling link: %w", err)
	}

	// Use full IDs in link filename
	linkPath := filepath.Join(s.path, "links", fmt.Sprintf("%s-%s-%s.json", source, target, linkType))
	if err := os.WriteFile(linkPath, data, 0644); err != nil {
		return fmt.Errorf("writing link file: %w", err)
	}

	log.Printf("Created link file: %s", linkPath)

	// Update repository modified time
	if err := s.updateModified(); err != nil {
		return fmt.Errorf("updating modified time: %w", err)
	}

	return nil
}

// GetLinks returns all links for a node
func (s *DAGStore) GetLinks(nodeID string) ([]core.Link, error) {
	var links []core.Link
	linksDir := filepath.Join(s.path, "links")

	entries, err := os.ReadDir(linksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return links, nil
		}
		return nil, fmt.Errorf("reading links directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(linksDir, entry.Name()))
			if err != nil {
				continue
			}

			var link core.Link
			if err := json.Unmarshal(data, &link); err != nil {
				continue
			}

			if link.Source == nodeID || link.Target == nodeID {
				links = append(links, link)
			}
		}
	}

	return links, nil
}

// DeleteLink removes a link
func (s *DAGStore) DeleteLink(source, target string) error {
	linksDir := filepath.Join(s.path, "links")
	entries, err := os.ReadDir(linksDir)
	if err != nil {
		return fmt.Errorf("reading links directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(linksDir, entry.Name()))
			if err != nil {
				continue
			}

			var link core.Link
			if err := json.Unmarshal(data, &link); err != nil {
				continue
			}

			if link.Source == source && link.Target == target {
				if err := os.Remove(filepath.Join(linksDir, entry.Name())); err != nil {
					return fmt.Errorf("removing link file: %w", err)
				}

				// Update repository modified time
				if err := s.updateModified(); err != nil {
					return fmt.Errorf("updating modified time: %w", err)
				}
			}
		}
	}

	return nil
}
