package memex

import (
	"fmt"
	"os"

	"memex/internal/memex/storage"
)

// Memex represents a memex instance
type Memex struct {
	repo *storage.MXStore
}

// Open opens a memex repository at the given path
func Open(path string) (*Memex, error) {
	// If repository exists, open it
	if _, err := os.Stat(path); err == nil {
		repo, err := storage.OpenMX(path)
		if err != nil {
			return nil, fmt.Errorf("opening repository: %w", err)
		}
		return &Memex{repo: repo}, nil
	}

	// Create new repository
	repo, err := storage.CreateMX(path)
	if err != nil {
		return nil, fmt.Errorf("creating repository: %w", err)
	}

	return &Memex{repo: repo}, nil
}

// GetRepository returns the underlying repository
func (m *Memex) GetRepository() (*storage.MXStore, error) {
	if m.repo == nil {
		return nil, fmt.Errorf("repository not initialized")
	}
	return m.repo, nil
}

// Add adds content to the repository
func (m *Memex) Add(content []byte, nodeType string, meta map[string]any) (string, error) {
	// Store content as chunks
	if meta == nil {
		meta = make(map[string]any)
	}
	return m.repo.AddNode(content, nodeType, meta)
}

// Get retrieves an object by ID
func (m *Memex) Get(id string) (Node, error) {
	node, err := m.repo.GetNode(id)
	if err != nil {
		return Node{}, fmt.Errorf("getting node: %w", err)
	}

	// Reconstruct content from chunks if available
	if contentHash, ok := node.Meta["content"].(string); ok {
		content, err := m.repo.ReconstructContent(contentHash)
		if err != nil {
			return Node{}, fmt.Errorf("reconstructing content: %w", err)
		}
		return Node{
			ID:       node.ID,
			Type:     node.Type,
			Meta:     node.Meta,
			Content:  content,
			Created:  node.Created,
			Modified: node.Modified,
		}, nil
	}

	return Node{
		ID:       node.ID,
		Type:     node.Type,
		Meta:     node.Meta,
		Created:  node.Created,
		Modified: node.Modified,
	}, nil
}

// Update updates an object's content and metadata
func (m *Memex) Update(id string, content []byte) error {
	// Get existing object to preserve metadata
	node, err := m.repo.GetNode(id)
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	// Store new content
	meta := make(map[string]any)
	for k, v := range node.Meta {
		if k != "content" && k != "chunks" { // Don't copy old content hash or chunks
			meta[k] = v
		}
	}

	// Add new node
	newID, err := m.repo.AddNode(content, node.Type, meta)
	if err != nil {
		return fmt.Errorf("adding new node: %w", err)
	}

	// Get all links to update
	links, err := m.repo.GetLinks(id)
	if err != nil {
		return fmt.Errorf("getting links: %w", err)
	}

	// Delete old node
	if err := m.repo.DeleteNode(id); err != nil {
		return fmt.Errorf("deleting old node: %w", err)
	}

	// Recreate links with new ID
	for _, link := range links {
		err := m.repo.AddLink(newID, link.Target, link.Type, link.Meta)
		if err != nil {
			return fmt.Errorf("recreating link: %w", err)
		}
	}

	return nil
}

// Delete removes an object
func (m *Memex) Delete(id string) error {
	return m.repo.DeleteNode(id)
}

// Link creates a link between objects
func (m *Memex) Link(source, target, linkType string, meta map[string]any) error {
	return m.repo.AddLink(source, target, linkType, meta)
}

// GetLinks returns all links for an object
func (m *Memex) GetLinks(id string) ([]Link, error) {
	links, err := m.repo.GetLinks(id)
	if err != nil {
		return nil, err
	}

	result := make([]Link, len(links))
	for i, link := range links {
		result[i] = Link{
			Target: link.Target,
			Type:   link.Type,
			Meta:   link.Meta,
		}
	}

	return result, nil
}
