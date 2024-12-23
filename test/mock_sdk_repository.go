package test

import (
	"fmt"

	"memex/internal/memex/core"
	"memex/pkg/sdk/types"
)

// MockSDKRepository is a test double that implements types.Repository using
// in-memory maps for nodes and links. It intentionally ignores any core-specific
// logic not needed by the SDK.
type MockSDKRepository struct {
	nodes   map[string]*core.Node
	links   map[string][]*core.Link
	modules map[string]core.Module
}

// NewMockSDKRepository constructs a fresh mock repository that satisfies types.Repository.
func NewMockSDKRepository() *MockSDKRepository {
	return &MockSDKRepository{
		nodes:   make(map[string]*core.Node),
		links:   make(map[string][]*core.Link),
		modules: make(map[string]core.Module),
	}
}

// AddNode stores a new node in memory and returns its generated ID.
func (r *MockSDKRepository) AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error) {
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

// GetNode retrieves a node by ID and returns it as a types.Node.
func (r *MockSDKRepository) GetNode(id string) (*types.Node, error) {
	n, ok := r.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", id)
	}
	return &types.Node{
		ID:      n.ID,
		Type:    n.Type,
		Content: n.Content,
		Meta:    n.Meta,
	}, nil
}

// DeleteNode removes the node and corresponding links from memory.
func (r *MockSDKRepository) DeleteNode(id string) error {
	if _, exists := r.nodes[id]; !exists {
		return fmt.Errorf("node not found: %s", id)
	}
	delete(r.nodes, id)
	delete(r.links, id)
	return nil
}

// AddLink creates a link from source to target.
func (r *MockSDKRepository) AddLink(source, target, linkType string, meta map[string]interface{}) error {
	if _, ok := r.nodes[source]; !ok {
		return fmt.Errorf("source node not found: %s", source)
	}
	if _, ok := r.nodes[target]; !ok {
		return fmt.Errorf("target node not found: %s", target)
	}
	link := &core.Link{
		Source: source,
		Target: target,
		Type:   linkType,
		Meta:   meta,
	}
	r.links[source] = append(r.links[source], link)
	return nil
}

// GetLinks returns a list of links originating from the given node ID.
func (r *MockSDKRepository) GetLinks(nodeID string) ([]*types.Link, error) {
	coreLinks := r.links[nodeID]
	out := make([]*types.Link, len(coreLinks))
	for i, c := range coreLinks {
		out[i] = &types.Link{
			Source: c.Source,
			Target: c.Target,
			Type:   c.Type,
			Meta:   c.Meta,
		}
	}
	return out, nil
}

// DeleteLink removes a single link from source to target, if it exists.
func (r *MockSDKRepository) DeleteLink(source, target, linkType string) error {
	lst := r.links[source]
	for i, l := range lst {
		if l.Target == target && l.Type == linkType {
			r.links[source] = append(lst[:i], lst[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("link not found")
}

// QueryNodes handles a types.Query, returning matching nodes.
func (r *MockSDKRepository) QueryNodes(q types.Query) ([]*types.Node, error) {
	var results []*types.Node
	for _, n := range r.nodes {
		// In this mock, we'll just return all nodes
		// A real implementation would filter based on the query
		results = append(results, &types.Node{
			ID:      n.ID,
			Type:    n.Type,
			Content: n.Content,
			Meta:    n.Meta,
		})
	}
	return results, nil
}

// QueryLinks handles a types.Query, returning matching links.
func (r *MockSDKRepository) QueryLinks(q types.Query) ([]*types.Link, error) {
	var all []*types.Link
	for _, group := range r.links {
		for _, cl := range group {
			all = append(all, &types.Link{
				Source: cl.Source,
				Target: cl.Target,
				Type:   cl.Type,
				Meta:   cl.Meta,
			})
		}
	}
	return all, nil
}

// QueryNodesByModule filters nodes by "module" field in metadata.
func (r *MockSDKRepository) QueryNodesByModule(moduleID string) ([]*core.Node, error) {
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
func (r *MockSDKRepository) QueryLinksByModule(moduleID string) ([]*core.Link, error) {
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

// Close is a no-op for this in-memory mock.
func (r *MockSDKRepository) Close() error {
	return nil
}

// Extra module management methods that some tests might expect

func (r *MockSDKRepository) RegisterModule(mod core.Module) error {
	if _, exists := r.modules[mod.ID()]; exists {
		return fmt.Errorf("module already registered: %s", mod.ID())
	}
	r.modules[mod.ID()] = mod
	return nil
}

func (r *MockSDKRepository) UnregisterModule(moduleID string) error {
	if _, exists := r.modules[moduleID]; !exists {
		return fmt.Errorf("module not found: %s", moduleID)
	}
	delete(r.modules, moduleID)
	return nil
}

func (r *MockSDKRepository) GetModule(id string) (core.Module, bool) {
	m, ok := r.modules[id]
	return m, ok
}

func (r *MockSDKRepository) ListModules() []core.Module {
	var out []core.Module
	for _, mod := range r.modules {
		out = append(out, mod)
	}
	return out
}
