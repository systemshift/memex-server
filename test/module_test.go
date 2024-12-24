package test

import (
	"testing"

	"memex/internal/memex/core"
)

// MockModule implements core.Module for testing
type MockModule struct {
	id          string
	name        string
	description string
	repo        core.Repository
}

func (m *MockModule) ID() string                                    { return m.id }
func (m *MockModule) Name() string                                  { return m.name }
func (m *MockModule) Description() string                           { return m.description }
func (m *MockModule) Init(repo core.Repository) error               { m.repo = repo; return nil }
func (m *MockModule) Commands() []core.Command                      { return nil }
func (m *MockModule) HandleCommand(cmd string, args []string) error { return nil }

func TestModuleRegistration(t *testing.T) {
	repo := NewMockSDKRepository()

	// Create test module
	module := &MockModule{
		id:          "test-module",
		name:        "Test Module",
		description: "A test module",
	}

	// Test registration
	if err := repo.RegisterModule(module); err != nil {
		t.Errorf("Failed to register module: %v", err)
	}

	// Test retrieval
	if mod, exists := repo.GetModule("test-module"); !exists {
		t.Error("Module not found after registration")
	} else if mod.ID() != module.ID() {
		t.Errorf("Got wrong module ID, expected %s, got %s", module.ID(), mod.ID())
	}

	// Test listing
	modules := repo.ListModules()
	if len(modules) != 1 {
		t.Errorf("Expected 1 module, got %d", len(modules))
	}
}

func TestModuleNodeOperations(t *testing.T) {
	repo := NewMockSDKRepository()
	module := &MockModule{id: "test-module"}
	repo.RegisterModule(module)

	// Add node with module metadata
	content := []byte("test content")
	meta := map[string]interface{}{
		"module": "test-module",
		"key":    "value",
	}

	nodeID, err := repo.AddNode(content, "test.doc", meta)
	if err != nil {
		t.Errorf("Failed to add node: %v", err)
	}

	// Query nodes by module (extra method in MockSDKRepository)
	nodes, err := repo.QueryNodesByModule("test-module")
	if err != nil {
		t.Errorf("Failed to query nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(nodes))
	}
	if len(nodes) > 0 && nodes[0].ID != nodeID {
		t.Errorf("Got wrong node ID, expected %s, got %s", nodeID, nodes[0].ID)
	}
}

func TestModuleLinkOperations(t *testing.T) {
	repo := NewMockSDKRepository()
	module := &MockModule{id: "test-module"}
	repo.RegisterModule(module)

	// Create two nodes
	sourceID, _ := repo.AddNode([]byte("source"), "test.doc", nil)
	targetID, _ := repo.AddNode([]byte("target"), "test.doc", nil)

	// Add link with module metadata
	linkMeta := map[string]interface{}{
		"module": "test-module",
		"key":    "value",
	}
	err := repo.AddLink(sourceID, targetID, "test-link", linkMeta)
	if err != nil {
		t.Errorf("Failed to add link: %v", err)
	}

	// Query links by module (extra method in MockSDKRepository)
	links, err := repo.QueryLinksByModule("test-module")
	if err != nil {
		t.Errorf("Failed to query links: %v", err)
	}
	if len(links) != 1 {
		t.Errorf("Expected 1 link, got %d", len(links))
	}
	if len(links) > 0 {
		if links[0].Source != sourceID {
			t.Errorf("Got wrong source ID, expected %s, got %s", sourceID, links[0].Source)
		}
		if links[0].Target != targetID {
			t.Errorf("Got wrong target ID, expected %s, got %s", targetID, links[0].Target)
		}
	}
}
