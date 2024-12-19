package test

import (
	"memex/internal/memex/core"
	"memex/pkg/sdk/types"
)

// TestModule is a mock module implementation for testing.
type TestModule struct {
	id          string
	repo        types.Repository
	lastCommand string
}

// NewTestModule creates a new test module with the given repository.
func NewTestModule(repo types.Repository) *TestModule {
	return &TestModule{
		repo: repo,
	}
}

// ID returns the module's identifier.
func (m *TestModule) ID() string {
	return m.id
}

// Name returns the module's name.
func (m *TestModule) Name() string {
	return "Test Module"
}

// SetID sets the module's identifier.
func (m *TestModule) SetID(id string) {
	m.id = id
}

// Description returns the module's description.
func (m *TestModule) Description() string {
	return "A test module for testing purposes"
}

// ValidateLinkType validates a link type.
func (m *TestModule) ValidateLinkType(linkType string) bool {
	// For testing purposes, accept any link type
	return true
}

// ValidateNodeType validates a node type.
func (m *TestModule) ValidateNodeType(nodeType string) bool {
	// For testing purposes, accept any node type
	return true
}

// ValidateMetadata validates metadata for nodes and links.
func (m *TestModule) ValidateMetadata(meta map[string]interface{}) error {
	// For testing purposes, accept any metadata
	return nil
}

// Commands returns the module's available commands.
func (m *TestModule) Commands() []core.ModuleCommand {
	return []core.ModuleCommand{
		{
			Name:        "add",
			Description: "Add something",
			Usage:       "add <arg>",
			Args:        []string{"<arg>"},
		},
		{
			Name:        "remove",
			Description: "Remove something",
			Usage:       "remove <arg>",
			Args:        []string{"<arg>"},
		},
		{
			Name:        "list",
			Description: "List things",
			Usage:       "list",
			Args:        []string{},
		},
	}
}

// GetCommands returns the module's available command names.
func (m *TestModule) GetCommands() []string {
	return []string{"add", "remove", "list"}
}

// HandleCommand processes a command with the given arguments.
func (m *TestModule) HandleCommand(command string, args []string) error {
	m.lastCommand = command
	return nil
}

// GetLastCommand returns the last command that was handled.
func (m *TestModule) GetLastCommand() string {
	return m.lastCommand
}

// Repository returns the module's repository.
func (m *TestModule) Repository() types.Repository {
	return m.repo
}
