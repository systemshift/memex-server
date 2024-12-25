package test

import (
	"fmt"
	"testing"

	"memex/pkg/sdk"
	"memex/pkg/sdk/types"
)

type mockModule struct {
	id          string
	name        string
	description string
	initCalled  bool
	commands    []types.Command
	lastCommand string
	lastArgs    []string
}

func (m *mockModule) ID() string                { return m.id }
func (m *mockModule) Name() string              { return m.name }
func (m *mockModule) Description() string       { return m.description }
func (m *mockModule) Commands() []types.Command { return m.commands }
func (m *mockModule) Init(repo types.Repository) error {
	m.initCalled = true
	return nil
}
func (m *mockModule) HandleCommand(cmd string, args []string) error {
	m.lastCommand = cmd
	m.lastArgs = args
	return nil
}

func TestModuleManager(t *testing.T) {
	// Create test modules
	mod1 := &mockModule{id: "test1", name: "Test 1"}
	mod2 := &mockModule{id: "test2", name: "Test 2"}

	// Create manager
	mgr := sdk.NewManager()

	// Test registration
	t.Run("registration", func(t *testing.T) {
		// Register valid modules
		if err := mgr.RegisterModule(mod1); err != nil {
			t.Errorf("RegisterModule() error = %v", err)
		}
		if err := mgr.RegisterModule(mod2); err != nil {
			t.Errorf("RegisterModule() error = %v", err)
		}

		// Test duplicate registration
		if err := mgr.RegisterModule(mod1); err == nil {
			t.Error("RegisterModule() should error on duplicate")
		}

		// Test nil module
		if err := mgr.RegisterModule(nil); err == nil {
			t.Error("RegisterModule() should error on nil module")
		}
	})

	// Test module retrieval
	t.Run("retrieval", func(t *testing.T) {
		// Get existing module
		if mod, exists := mgr.GetModule("test1"); !exists {
			t.Error("GetModule() should find test1")
		} else if mod.ID() != "test1" {
			t.Errorf("GetModule() got = %v, want test1", mod.ID())
		}

		// Get non-existent module
		if _, exists := mgr.GetModule("nonexistent"); exists {
			t.Error("GetModule() should not find nonexistent")
		}

		// List modules
		mods := mgr.ListModules()
		if len(mods) != 2 {
			t.Errorf("ListModules() got %v modules, want 2", len(mods))
		}
	})

	// Test command handling
	t.Run("commands", func(t *testing.T) {
		// Handle valid command
		cmd := "test"
		args := []string{"arg1", "arg2"}
		if err := mgr.HandleCommand("test1", cmd, args); err != nil {
			t.Errorf("HandleCommand() error = %v", err)
		}

		// Verify command was handled
		if mod1.lastCommand != cmd {
			t.Errorf("HandleCommand() command = %v, want %v", mod1.lastCommand, cmd)
		}
		if len(mod1.lastArgs) != len(args) {
			t.Errorf("HandleCommand() args = %v, want %v", mod1.lastArgs, args)
		}

		// Handle command for non-existent module
		if err := mgr.HandleCommand("nonexistent", cmd, args); err == nil {
			t.Error("HandleCommand() should error on non-existent module")
		}
	})

	// Test repository integration
	t.Run("repository", func(t *testing.T) {
		// Set repository
		mockRepo := &mockRepository{}
		if err := mgr.SetRepository(mockRepo); err != nil {
			t.Errorf("SetRepository() error = %v", err)
		}

		// Verify modules were initialized
		if !mod1.initCalled {
			t.Error("Module 1 should be initialized")
		}
		if !mod2.initCalled {
			t.Error("Module 2 should be initialized")
		}

		// Test nil repository
		if err := mgr.SetRepository(nil); err == nil {
			t.Error("SetRepository() should error on nil repository")
		}
	})
}

// Mock repository for testing
type mockRepository struct{}

func (r *mockRepository) AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error) {
	return "test-id", nil
}

func (r *mockRepository) GetNode(id string) (*types.Node, error) {
	return nil, fmt.Errorf("not found")
}

func (r *mockRepository) DeleteNode(id string) error {
	return nil
}

func (r *mockRepository) AddLink(source, target, linkType string, meta map[string]interface{}) error {
	return nil
}

func (r *mockRepository) GetLinks(nodeID string) ([]*types.Link, error) {
	return nil, nil
}

func (r *mockRepository) DeleteLink(source, target, linkType string) error {
	return nil
}

func (r *mockRepository) QueryNodes(query types.Query) ([]*types.Node, error) {
	return nil, nil
}

func (r *mockRepository) QueryLinks(query types.Query) ([]*types.Link, error) {
	return nil, nil
}
