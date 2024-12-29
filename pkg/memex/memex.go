package memex

import (
	"fmt"

	"github.com/systemshift/memex/internal/memex/core"
	"github.com/systemshift/memex/internal/memex/repository"
)

// Memex represents a memex instance
type Memex struct {
	repo core.Repository
}

// Open opens an existing repository
func Open(path string) (*Memex, error) {
	repo, err := repository.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening repository: %w", err)
	}
	return &Memex{repo: repo}, nil
}

// Create creates a new repository
func Create(path string) (*Memex, error) {
	repo, err := repository.Create(path)
	if err != nil {
		return nil, fmt.Errorf("creating repository: %w", err)
	}
	return &Memex{repo: repo}, nil
}

// Close closes the repository
func (m *Memex) Close() error {
	return m.repo.Close()
}

// Add adds content to the repository
func (m *Memex) Add(content []byte, nodeType string, meta map[string]interface{}) (string, error) {
	return m.repo.AddNode(content, nodeType, meta)
}

// AddWithID adds content with a specific ID
func (m *Memex) AddWithID(id string, content []byte, nodeType string, meta map[string]interface{}) error {
	return m.repo.AddNodeWithID(id, content, nodeType, meta)
}

// Get retrieves a node by ID
func (m *Memex) Get(id string) (*Node, error) {
	node, err := m.repo.GetNode(id)
	if err != nil {
		return nil, fmt.Errorf("getting node: %w", err)
	}
	return &Node{
		ID:       node.ID,
		Type:     node.Type,
		Content:  node.Content,
		Meta:     node.Meta,
		Created:  node.Created,
		Modified: node.Modified,
	}, nil
}

// Delete removes a node
func (m *Memex) Delete(id string) error {
	return m.repo.DeleteNode(id)
}

// Link creates a link between nodes
func (m *Memex) Link(source, target, linkType string, meta map[string]interface{}) error {
	return m.repo.AddLink(source, target, linkType, meta)
}

// GetLinks returns all links for a node
func (m *Memex) GetLinks(id string) ([]*Link, error) {
	links, err := m.repo.GetLinks(id)
	if err != nil {
		return nil, fmt.Errorf("getting links: %w", err)
	}

	result := make([]*Link, len(links))
	for i, link := range links {
		result[i] = &Link{
			Source:   link.Source,
			Target:   link.Target,
			Type:     link.Type,
			Meta:     link.Meta,
			Created:  link.Created,
			Modified: link.Modified,
		}
	}
	return result, nil
}

// DeleteLink removes a link
func (m *Memex) DeleteLink(source, target, linkType string) error {
	return m.repo.DeleteLink(source, target, linkType)
}

// ListNodes returns a list of all node IDs
func (m *Memex) ListNodes() ([]string, error) {
	return m.repo.ListNodes()
}

// GetContent retrieves raw content by ID
func (m *Memex) GetContent(id string) ([]byte, error) {
	return m.repo.GetContent(id)
}
