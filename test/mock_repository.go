package test

import (
	"fmt"

	"memex/internal/memex/core"
)

// MockRepository implements core.Repository for testing
type MockRepository struct {
	nodes          map[string]*core.Node
	links          map[string][]*core.Link
	modules        map[string]core.Module
	enabledModules map[string]bool
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		nodes:          make(map[string]*core.Node),
		links:          make(map[string][]*core.Link),
		modules:        make(map[string]core.Module),
		enabledModules: make(map[string]bool),
	}
}

// Module operations

func (r *MockRepository) RegisterModule(module core.Module) error {
	if _, exists := r.modules[module.ID()]; exists {
		return fmt.Errorf("module already registered: %s", module.ID())
	}
	r.modules[module.ID()] = module
	r.enabledModules[module.ID()] = true // Enable by default
	return nil
}

func (r *MockRepository) UnregisterModule(moduleID string) error {
	if _, exists := r.modules[moduleID]; !exists {
		return fmt.Errorf("module not found: %s", moduleID)
	}
	delete(r.modules, moduleID)
	delete(r.enabledModules, moduleID)
	return nil
}

func (r *MockRepository) GetModule(id string) (core.Module, bool) {
	module, exists := r.modules[id]
	return module, exists
}

func (r *MockRepository) ListModules() []core.Module {
	modules := make([]core.Module, 0, len(r.modules))
	for _, module := range r.modules {
		modules = append(modules, module)
	}
	return modules
}

func (r *MockRepository) EnableModule(moduleID string) error {
	if _, exists := r.modules[moduleID]; !exists {
		return fmt.Errorf("module not found: %s", moduleID)
	}
	r.enabledModules[moduleID] = true
	return nil
}

func (r *MockRepository) DisableModule(moduleID string) error {
	if _, exists := r.modules[moduleID]; !exists {
		return fmt.Errorf("module not found: %s", moduleID)
	}
	r.enabledModules[moduleID] = false
	return nil
}

func (r *MockRepository) IsModuleEnabled(moduleID string) bool {
	enabled, exists := r.enabledModules[moduleID]
	return exists && enabled
}

// Node operations

func (r *MockRepository) AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error) {
	// Generate simple ID from content
	id := fmt.Sprintf("node-%d", len(r.nodes)+1)

	if err := r.AddNodeWithID(id, content, nodeType, meta); err != nil {
		return "", err
	}

	return id, nil
}

func (r *MockRepository) AddNodeWithID(id string, content []byte, nodeType string, meta map[string]interface{}) error {
	if _, exists := r.nodes[id]; exists {
		return fmt.Errorf("node already exists: %s", id)
	}

	node := &core.Node{
		ID:      id,
		Type:    nodeType,
		Content: content,
		Meta:    meta,
	}

	r.nodes[id] = node
	return nil
}

func (r *MockRepository) GetNode(id string) (*core.Node, error) {
	node, ok := r.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", id)
	}
	return node, nil
}

func (r *MockRepository) GetContent(id string) ([]byte, error) {
	node, err := r.GetNode(id)
	if err != nil {
		return nil, err
	}
	return node.Content, nil
}

func (r *MockRepository) DeleteNode(id string) error {
	if _, ok := r.nodes[id]; !ok {
		return fmt.Errorf("node not found: %s", id)
	}
	delete(r.nodes, id)
	delete(r.links, id)
	return nil
}

func (r *MockRepository) ListNodes() ([]string, error) {
	nodes := make([]string, 0, len(r.nodes))
	for id := range r.nodes {
		nodes = append(nodes, id)
	}
	return nodes, nil
}

// Link operations

func (r *MockRepository) AddLink(source, target, linkType string, meta map[string]interface{}) error {
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

func (r *MockRepository) GetLinks(nodeID string) ([]*core.Link, error) {
	return r.links[nodeID], nil
}

func (r *MockRepository) DeleteLink(source, target, linkType string) error {
	links := r.links[source]
	for i, link := range links {
		if link.Target == target && link.Type == linkType {
			r.links[source] = append(links[:i], links[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("link not found")
}

// Query operations

func (r *MockRepository) QueryNodesByModule(moduleID string) ([]*core.Node, error) {
	var nodes []*core.Node
	for _, node := range r.nodes {
		if modID, ok := node.Meta["module"].(string); ok && modID == moduleID {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func (r *MockRepository) QueryLinksByModule(moduleID string) ([]*core.Link, error) {
	var links []*core.Link
	for _, nodeLinks := range r.links {
		for _, link := range nodeLinks {
			if modID, ok := link.Meta["module"].(string); ok && modID == moduleID {
				links = append(links, link)
			}
		}
	}
	return links, nil
}

// Close does nothing for mock repository
func (r *MockRepository) Close() error {
	return nil
}
