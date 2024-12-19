// This version of MockCoreRepository is carefully aligned with what the migration test code expects
// from the core.Repository interface. Based on the error messages, we see references to:
//
//   1) AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error)
//   2) AddNodeWithID(id string, content []byte, nodeType string, meta map[string]interface{}) (string, error)
//   3) DeleteNode(id string) error
//
// The migration tests apparently do things like:
//   id, err := repo.AddNode(...)
//   id2, err := repo.AddNodeWithID(...)
//   err := repo.DeleteNode(id)
//
// This file now provides those methods, returning the correct type signatures so the migration test
// can compile without interface mismatches. It also includes the other methods (GetNode, AddLink, etc.)
// used by the migration code.

package test

import (
	"fmt"

	"memex/internal/memex/core"
)

// MockCoreRepository is a test double implementing core.Repository for migration tests.
// It matches the signature the test code expects, with AddNode/AddNodeWithID returning (string, error),
// as well as a DeleteNode method returning error.
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

// AddNode returns (string, error). The migration tests do:
//
//	id, err := repo.AddNode(...)
func (r *MockCoreRepository) AddNode(
	content []byte,
	nodeType string,
	meta map[string]interface{},
) (string, error) {
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

// AddNodeWithID returns (string, error). The migration tests do:
//
//	id, err := repo.AddNodeWithID(...)
func (r *MockCoreRepository) AddNodeWithID(
	id string,
	content []byte,
	nodeType string,
	meta map[string]interface{},
) (string, error) {
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

// DeleteNode appears to be called by migration code with:
//
//	err := repo.DeleteNode(id)
func (r *MockCoreRepository) DeleteNode(id string) error {
	if _, exists := r.nodes[id]; !exists {
		return fmt.Errorf("node not found: %s", id)
	}
	// remove the node
	delete(r.nodes, id)
	// remove all links from this node
	delete(r.links, id)
	return nil
}

// GetNode fetches a node by ID, returning (*core.Node, error).
func (r *MockCoreRepository) GetNode(id string) (*core.Node, error) {
	n, ok := r.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", id)
	}
	return n, nil
}

// AddLink creates a link between two nodes.
func (r *MockCoreRepository) AddLink(
	source, target, linkType string,
	meta map[string]interface{},
) error {
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
func (r *MockCoreRepository) DeleteLink(
	source, target, linkType string,
) error {
	existing := r.links[source]
	for i, l := range existing {
		if l.Target == target && l.Type == linkType {
			r.links[source] = append(existing[:i], existing[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("link not found")
}

// QueryLinks retrieves all links for a given node.
func (r *MockCoreRepository) QueryLinks(nodeID string) ([]*core.Link, error) {
	return r.links[nodeID], nil
}

// QueryNodesByModule filters nodes by "module" field in metadata.
func (r *MockCoreRepository) QueryNodesByModule(moduleID string) ([]*core.Node, error) {
	var results []*core.Node
	for _, nd := range r.nodes {
		if nd.Meta != nil {
			if modID, ok := nd.Meta["module"].(string); ok && modID == moduleID {
				results = append(results, nd)
			}
		}
	}
	return results, nil
}

// QueryLinksByModule filters links by "module" field in metadata.
func (r *MockCoreRepository) QueryLinksByModule(moduleID string) ([]*core.Link, error) {
	var results []*core.Link
	for _, bucket := range r.links {
		for _, l := range bucket {
			if l.Meta != nil {
				if modID, ok := l.Meta["module"].(string); ok && modID == moduleID {
					results = append(results, l)
				}
			}
		}
	}
	return results, nil
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
