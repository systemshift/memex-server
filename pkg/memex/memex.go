package memex

import (
	"fmt"
	"os"
	"path/filepath"

	"memex/internal/memex/storage"
)

// Memex represents a memex instance
type Memex struct {
	repo *storage.DAGStore
}

// Open opens a memex repository at the given path
func Open(path string) (*Memex, error) {
	// If repository exists, open it
	if _, err := os.Stat(path); err == nil {
		repo, err := storage.OpenRepository(path)
		if err != nil {
			return nil, fmt.Errorf("opening repository: %w", err)
		}
		return &Memex{repo: repo}, nil
	}

	// Create new repository
	name := filepath.Base(path)
	if filepath.Ext(name) == ".mx" {
		name = name[:len(name)-3]
	}

	repo, err := storage.CreateRepository(path, name)
	if err != nil {
		return nil, fmt.Errorf("creating repository: %w", err)
	}

	return &Memex{repo: repo}, nil
}

// GetRepository returns the underlying repository
func (m *Memex) GetRepository() (*storage.DAGStore, error) {
	if m.repo == nil {
		return nil, fmt.Errorf("repository not initialized")
	}
	return m.repo, nil
}

// Add adds content to the repository
func (m *Memex) Add(content []byte, nodeType string, meta map[string]any) (string, error) {
	return m.repo.AddNode(content, nodeType, meta)
}

// Get retrieves an object by ID
func (m *Memex) Get(id string) (Node, error) {
	node, err := m.repo.GetNode(id)
	if err != nil {
		return Node{}, fmt.Errorf("getting node: %w", err)
	}

	// Get current version content
	version, err := m.repo.GetVersion(id, node.Current)
	if err != nil {
		return Node{}, fmt.Errorf("getting version: %w", err)
	}

	// Reconstruct content
	var content []byte
	for _, hash := range version.Chunks {
		chunk, err := m.repo.GetChunk(hash)
		if err != nil {
			return Node{}, fmt.Errorf("getting chunk: %w", err)
		}
		content = append(content, chunk...)
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

// Update updates an object's content and metadata
func (m *Memex) Update(id string, content []byte) error {
	// Get existing object to preserve metadata
	node, err := m.repo.GetNode(id)
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	// Update with new content but keep metadata
	return m.repo.UpdateNode(id, content, node.Meta)
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

	// Convert core.Link to Link
	result := make([]Link, len(links))
	for i, link := range links {
		result[i] = Link{
			Source: link.Source,
			Target: link.Target,
			Type:   link.Type,
			Meta:   link.Meta,
		}
	}

	return result, nil
}

// Search finds objects matching criteria
func (m *Memex) Search(query map[string]any) ([]Node, error) {
	nodes, err := m.repo.Search(query)
	if err != nil {
		return nil, err
	}

	// Convert core.Node to Node (without content)
	result := make([]Node, len(nodes))
	for i, node := range nodes {
		result[i] = Node{
			ID:       node.ID,
			Type:     node.Type,
			Meta:     node.Meta,
			Created:  node.Created,
			Modified: node.Modified,
		}
	}

	return result, nil
}

// FindByType returns all objects of a specific type
func (m *Memex) FindByType(nodeType string) ([]Node, error) {
	nodes, err := m.repo.FindByType(nodeType)
	if err != nil {
		return nil, err
	}

	// Convert core.Node to Node (without content)
	result := make([]Node, len(nodes))
	for i, node := range nodes {
		result[i] = Node{
			ID:       node.ID,
			Type:     node.Type,
			Meta:     node.Meta,
			Created:  node.Created,
			Modified: node.Modified,
		}
	}

	return result, nil
}

// List returns all objects
func (m *Memex) List() []string {
	root, err := m.repo.GetRoot()
	if err != nil {
		return []string{}
	}
	return root.Nodes
}
