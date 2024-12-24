package test

import (
	"memex/internal/memex/core"
)

// TestModule is a mock module implementation for testing.
type TestModule struct {
	id          string
	repo        core.Repository
	lastCommand string
}

// NewTestModule creates a new test module with the given repository.
func NewTestModule() *TestModule {
	return &TestModule{
		id: "test",
	}
}

func (m *TestModule) ID() string          { return m.id }
func (m *TestModule) Name() string        { return "Test Module" }
func (m *TestModule) Description() string { return "A test module for testing purposes" }

func (m *TestModule) Init(repo core.Repository) error {
	m.repo = repo
	return nil
}

func (m *TestModule) Commands() []core.Command {
	return []core.Command{
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

func (m *TestModule) HandleCommand(cmd string, args []string) error {
	m.lastCommand = cmd
	return nil
}

// GetLastCommand returns the last command that was handled (for testing).
func (m *TestModule) GetLastCommand() string {
	return m.lastCommand
}

// SetID sets the module's identifier (for testing).
func (m *TestModule) SetID(id string) {
	m.id = id
}
