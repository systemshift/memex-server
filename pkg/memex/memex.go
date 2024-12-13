package memex

import (
	"fmt"

	"memex/internal/memex/core"
	"memex/internal/memex/repository"
)

// Module represents a module that can be installed and used
type Module interface {
	ID() string
	Name() string
	Description() string
	Run(args []string) error
}

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
			Target: link.Target,
			Type:   link.Type,
			Meta:   link.Meta,
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

// Module operations

// RegisterModule registers a new module
func (m *Memex) RegisterModule(module core.Module) error {
	return m.repo.RegisterModule(module)
}

// GetModule returns a module by ID
func (m *Memex) GetModule(id string) (core.Module, bool) {
	return m.repo.GetModule(id)
}

// ListModules returns all registered modules
func (m *Memex) ListModules() []core.Module {
	return m.repo.ListModules()
}

// QueryNodesByModule returns all nodes created by a module
func (m *Memex) QueryNodesByModule(moduleID string) ([]*Node, error) {
	nodes, err := m.repo.QueryNodesByModule(moduleID)
	if err != nil {
		return nil, err
	}

	result := make([]*Node, len(nodes))
	for i, node := range nodes {
		result[i] = &Node{
			ID:       node.ID,
			Type:     node.Type,
			Content:  node.Content,
			Meta:     node.Meta,
			Created:  node.Created,
			Modified: node.Modified,
		}
	}
	return result, nil
}

// QueryLinksByModule returns all links created by a module
func (m *Memex) QueryLinksByModule(moduleID string) ([]*Link, error) {
	links, err := m.repo.QueryLinksByModule(moduleID)
	if err != nil {
		return nil, err
	}

	result := make([]*Link, len(links))
	for i, link := range links {
		result[i] = &Link{
			Target: link.Target,
			Type:   link.Type,
			Meta:   link.Meta,
		}
	}
	return result, nil
}
