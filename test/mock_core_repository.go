// MockCoreRepository implements core.Repository for migration tests, storing nodes & links in memory.
// The migration code expects:
// - AddNode to return (string, error)
// - AddNodeWithID to return error
// - GetLinks to return []*core.Link
// - ListNodes to return ([]string, error)

package test

import (
	"fmt"

	"github.com/systemshift/memex/internal/memex/core"
)

// MockCoreRepository is a test double implementing core.Repository for migration tests.
type MockCoreRepository struct {
	nodes map[string]*core.Node
	links map[string][]*core.Link
}

// NewMockCoreRepository builds a fresh in-memory repository that satisfies core.Repository.
func NewMockCoreRepository() *MockCoreRepository {
	return &MockCoreRepository{
		nodes: make(map[string]*core.Node),
		links: make(map[string][]*core.Link),
	}
}

// AddNode returns (string, error) as expected by core.Repository.
func (r *MockCoreRepository) AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error) {
	id := fmt.Sprintf("node-%d", len(r.nodes)+1)
	if _, exists := r.nodes[id]; exists {
		return "", fmt.Errorf("node already exists: %s", id)
	}
	r.nodes[id] = &core.Node{
		ID:      id,
		Type:    nodeType,
		Content: content,
		Meta:    meta,
	}
	return id, nil
}

// AddNodeWithID returns only error as expected by core.Repository.
func (r *MockCoreRepository) AddNodeWithID(id string, content []byte, nodeType string, meta map[string]interface{}) error {
	if _, exists := r.nodes[id]; exists {
		return fmt.Errorf("node already exists: %s", id)
	}
	r.nodes[id] = &core.Node{
		ID:      id,
		Type:    nodeType,
		Content: content,
		Meta:    meta,
	}
	return nil
}

// GetNode fetches a node by ID.
func (r *MockCoreRepository) GetNode(id string) (*core.Node, error) {
	n, ok := r.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", id)
	}
	return n, nil
}

// ListNodes returns all node IDs in the repository.
func (r *MockCoreRepository) ListNodes() ([]string, error) {
	var ids []string
	for id := range r.nodes {
		ids = append(ids, id)
	}
	return ids, nil
}

// DeleteNode removes a node and its links.
func (r *MockCoreRepository) DeleteNode(id string) error {
	if _, exists := r.nodes[id]; !exists {
		return fmt.Errorf("node not found: %s", id)
	}
	delete(r.nodes, id)
	delete(r.links, id)
	return nil
}

// AddLink creates a link between two nodes.
func (r *MockCoreRepository) AddLink(source, target, linkType string, meta map[string]interface{}) error {
	if _, ok := r.nodes[source]; !ok {
		return fmt.Errorf("source node not found: %s", source)
	}
	if _, ok := r.nodes[target]; !ok {
		return fmt.Errorf("target node not found: %s", target)
	}
	newLink := &core.Link{
		Source: source,
		Target: target,
		Type:   linkType,
		Meta:   meta,
	}
	r.links[source] = append(r.links[source], newLink)
	return nil
}

// DeleteLink removes a link between two nodes.
func (r *MockCoreRepository) DeleteLink(source, target, linkType string) error {
	existing := r.links[source]
	for i, l := range existing {
		if l.Target == target && l.Type == linkType {
			r.links[source] = append(existing[:i], existing[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("link not found")
}

// GetLinks returns all links for a given node ID.
func (r *MockCoreRepository) GetLinks(nodeID string) ([]*core.Link, error) {
	return r.links[nodeID], nil
}

// QueryLinks retrieves all links for a given node.
func (r *MockCoreRepository) QueryLinks(nodeID string) ([]*core.Link, error) {
	return r.links[nodeID], nil
}

// GetContent fetches the node's content by ID.
func (r *MockCoreRepository) GetContent(id string) ([]byte, error) {
	n, ok := r.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", id)
	}
	return n.Content, nil
}

// Close is a no-op for this mock.
func (r *MockCoreRepository) Close() error {
	return nil
}
