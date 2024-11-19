package memex

import (
	"fmt"
	"path/filepath"

	"memex/internal/memex/storage"
)

// Memex represents a content-addressable storage system
type Memex struct {
	repo *storage.DAGStore
}

// Open creates or opens a Memex instance at the specified path
func Open(path string) (*Memex, error) {
	repoPath := filepath.Join(path, ".memex")
	repo, err := storage.NewDAGStore(repoPath)
	if err != nil {
		return nil, fmt.Errorf("initializing repository: %w", err)
	}

	return &Memex{repo: repo}, nil
}

// Add adds new content to the repository
func (m *Memex) Add(content []byte, contentType string, meta map[string]any) (string, error) {
	id, err := m.repo.AddNode(content, contentType, meta)
	if err != nil {
		return "", err
	}
	return id, nil
}

// Get retrieves an object by ID
func (m *Memex) Get(id string) (Object, error) {
	node, err := m.repo.GetNode(id)
	if err != nil {
		return Object{}, err
	}

	// Get current version content
	version, err := m.repo.GetVersion(id, node.Current)
	if err != nil {
		return Object{}, err
	}

	// Reconstruct content from chunks
	var content []byte
	for _, hash := range version.Chunks {
		chunk, err := m.repo.GetChunk(hash)
		if err != nil {
			return Object{}, err
		}
		content = append(content, chunk...)
	}

	obj := convertNode(node)
	obj.Content = content
	return obj, nil
}

// Delete removes an object
func (m *Memex) Delete(id string) error {
	return m.repo.DeleteNode(id)
}

// Update updates an object's content
func (m *Memex) Update(id string, content []byte) error {
	node, err := m.repo.GetNode(id)
	if err != nil {
		return err
	}
	return m.repo.UpdateNode(id, content, node.Meta)
}

// Link creates a link between objects
func (m *Memex) Link(source, target, linkType string, meta map[string]any) error {
	return m.repo.AddLink(source, target, linkType, meta)
}

// GetLinks returns all links for an object
func (m *Memex) GetLinks(id string) ([]Link, error) {
	coreLinks, err := m.repo.GetLinks(id)
	if err != nil {
		return nil, err
	}
	var links []Link
	for _, link := range coreLinks {
		links = append(links, convertLink(link))
	}
	return links, nil
}

// List returns all object IDs
func (m *Memex) List() []string {
	root, err := m.repo.GetRoot()
	if err != nil {
		return []string{}
	}
	return root.Nodes
}

// FindByType returns all objects of a specific type
func (m *Memex) FindByType(contentType string) []Object {
	nodes, err := m.repo.FindByType(contentType)
	if err != nil {
		return []Object{}
	}

	var objects []Object
	for _, node := range nodes {
		obj := convertNode(node)
		objects = append(objects, obj)
	}
	return objects
}

// Search finds objects matching criteria
func (m *Memex) Search(query map[string]any) []Object {
	nodes, err := m.repo.Search(query)
	if err != nil {
		return []Object{}
	}

	var objects []Object
	for _, node := range nodes {
		obj := convertNode(node)
		objects = append(objects, obj)
	}
	return objects
}
