package test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"memex/internal/memex/core"
)

// MockRepository implements core.Repository for testing
type MockRepository struct {
	nodes   map[string]*core.Node
	links   map[string][]*core.Link
	modules *core.ModuleRegistry
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		nodes:   make(map[string]*core.Node),
		links:   make(map[string][]*core.Link),
		modules: core.NewModuleRegistry(),
	}
}

// Module operations

func (r *MockRepository) RegisterModule(module core.Module) error {
	return r.modules.RegisterModule(module)
}

func (r *MockRepository) GetModule(id string) (core.Module, bool) {
	return r.modules.GetModule(id)
}

func (r *MockRepository) ListModules() []core.Module {
	return r.modules.ListModules()
}

func (r *MockRepository) QueryNodesByModule(moduleID string) ([]*core.Node, error) {
	nodes := []*core.Node{}
	for _, node := range r.nodes {
		if modID, ok := node.Meta["module"].(string); ok && modID == moduleID {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func (r *MockRepository) QueryLinksByModule(moduleID string) ([]*core.Link, error) {
	links := []*core.Link{}
	for _, nodeLinks := range r.links {
		for _, link := range nodeLinks {
			if modID, ok := link.Meta["module"].(string); ok && modID == moduleID {
				links = append(links, link)
			}
		}
	}
	return links, nil
}

func (r *MockRepository) AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error) {
	// Generate deterministic ID from content
	hash := sha256.Sum256(content)
	id := hex.EncodeToString(hash[:])

	now := time.Now().UTC()
	node := &core.Node{
		ID:       id,
		Type:     nodeType,
		Content:  content,
		Meta:     meta,
		Created:  now,
		Modified: now,
	}
	if node.Meta == nil {
		node.Meta = make(map[string]interface{})
	}
	// Add chunks to metadata
	node.Meta["chunks"] = []string{id} // Use node ID as chunk ID for simplicity
	r.nodes[id] = node
	return id, nil
}

func (r *MockRepository) AddNodeWithID(id string, content []byte, nodeType string, meta map[string]interface{}) error {
	// Store node with given ID
	now := time.Now().UTC()
	node := &core.Node{
		ID:       id,
		Type:     nodeType,
		Content:  content,
		Meta:     meta,
		Created:  now,
		Modified: now,
	}
	if node.Meta == nil {
		node.Meta = make(map[string]interface{})
	}
	// Add chunks to metadata
	node.Meta["chunks"] = []string{id} // Use node ID as chunk ID for simplicity
	r.nodes[id] = node
	return nil
}

func (r *MockRepository) GetNode(id string) (*core.Node, error) {
	// Try both the raw ID and hex-decoded ID
	if node, ok := r.nodes[id]; ok {
		return node, nil
	}
	// If the ID is a hex string, try decoding it
	if len(id) == 64 { // Length of a hex-encoded SHA-256 hash
		if hashBytes, err := hex.DecodeString(id); err == nil {
			if node, ok := r.nodes[string(hashBytes)]; ok {
				return node, nil
			}
		}
	}
	return nil, fmt.Errorf("not found")
}

func (r *MockRepository) ListNodes() ([]string, error) {
	ids := make([]string, 0, len(r.nodes))
	for id := range r.nodes {
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *MockRepository) DeleteNode(id string) error {
	// Try deleting with raw ID first
	if _, ok := r.nodes[id]; ok {
		delete(r.nodes, id)
		delete(r.links, id)
		return nil
	}

	// If not found and ID is hex string, try decoding it
	if len(id) == 64 { // Length of a hex-encoded SHA-256 hash
		if hashBytes, err := hex.DecodeString(id); err == nil {
			rawID := string(hashBytes)
			if _, ok := r.nodes[rawID]; ok {
				delete(r.nodes, rawID)
				delete(r.links, rawID)
				return nil
			}
		}
	}

	return fmt.Errorf("not found")
}

func (r *MockRepository) AddLink(source, target, linkType string, meta map[string]interface{}) error {
	// Verify nodes exist
	if _, ok := r.nodes[source]; !ok {
		return fmt.Errorf("source node not found")
	}
	if _, ok := r.nodes[target]; !ok {
		return fmt.Errorf("target node not found")
	}

	now := time.Now().UTC()
	link := &core.Link{
		Source:   source,
		Target:   target,
		Type:     linkType,
		Meta:     meta,
		Created:  now,
		Modified: now,
	}
	if link.Meta == nil {
		link.Meta = make(map[string]interface{})
	}
	r.links[source] = append(r.links[source], link)
	return nil
}

func (r *MockRepository) GetLinks(nodeID string) ([]*core.Link, error) {
	if links, ok := r.links[nodeID]; ok {
		return links, nil
	}
	return nil, nil
}

func (r *MockRepository) DeleteLink(source, target, linkType string) error {
	links := r.links[source]
	for i, link := range links {
		if link.Target == target && link.Type == linkType {
			r.links[source] = append(links[:i], links[i+1:]...)
			break
		}
	}
	return nil
}

func (r *MockRepository) GetContent(id string) ([]byte, error) {
	if node, ok := r.nodes[id]; ok {
		return node.Content, nil
	}
	return nil, fmt.Errorf("not found")
}

func (r *MockRepository) Close() error {
	return nil
}
