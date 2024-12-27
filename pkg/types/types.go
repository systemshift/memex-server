package types

import (
	"fmt"
	"time"
)

// Node represents a node in the graph
type Node struct {
	ID       string
	Type     string
	Content  []byte
	Meta     map[string]interface{}
	Created  time.Time
	Modified time.Time
}

// Link represents a relationship between nodes
type Link struct {
	Source   string
	Target   string
	Type     string
	Meta     map[string]interface{}
	Created  time.Time
	Modified time.Time
}

// Command represents a module command
type Command struct {
	Name        string   // Command name (e.g., "add", "status")
	Description string   // Command description
	Usage       string   // Usage example (e.g., "git add <file>")
	Args        []string // Expected arguments
}

// Module defines the interface that all memex modules must implement
type Module interface {
	// Identity
	ID() string          // Unique identifier (e.g., "git", "ast")
	Name() string        // Human-readable name
	Description() string // Module description

	// Core functionality
	Init(repo Repository) error                    // Initialize module with repository
	Commands() []Command                           // Available commands
	HandleCommand(cmd string, args []string) error // Execute a command
}

// Repository defines the interface for repository operations
type Repository interface {
	// Node operations
	AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error)
	AddNodeWithID(id string, content []byte, nodeType string, meta map[string]interface{}) error
	GetNode(id string) (*Node, error)
	DeleteNode(id string) error
	ListNodes() ([]string, error)
	GetContent(id string) ([]byte, error)

	// Link operations
	AddLink(source, target, linkType string, meta map[string]interface{}) error
	GetLinks(nodeID string) ([]*Link, error)
	DeleteLink(source, target, linkType string) error

	// Module operations
	ListModules() []Module
	GetModule(id string) (Module, bool)
	RegisterModule(module Module) error

	// Query operations
	QueryNodesByModule(moduleID string) ([]*Node, error)
	QueryLinksByModule(moduleID string) ([]*Link, error)

	// Close closes the repository
	Close() error
}

// Common command names
const (
	CmdID          = "id"
	CmdName        = "name"
	CmdDescription = "description"
	CmdHelp        = "help"
)

// BaseModule provides a basic implementation of Module interface
type BaseModule struct {
	id          string
	name        string
	description string
	repo        Repository
	commands    []Command
}

// NewBaseModule creates a new base module
func NewBaseModule(id, name, description string) *BaseModule {
	return &BaseModule{
		id:          id,
		name:        name,
		description: description,
		commands:    make([]Command, 0),
	}
}

// ID returns the module identifier
func (m *BaseModule) ID() string {
	return m.id
}

// Name returns the module name
func (m *BaseModule) Name() string {
	return m.name
}

// Description returns the module description
func (m *BaseModule) Description() string {
	return m.description
}

// Init initializes the module with a repository
func (m *BaseModule) Init(repo Repository) error {
	m.repo = repo
	return nil
}

// Commands returns the list of available commands
func (m *BaseModule) Commands() []Command {
	baseCommands := []Command{
		{
			Name:        CmdID,
			Description: "Get module ID",
		},
		{
			Name:        CmdName,
			Description: "Get module name",
		},
		{
			Name:        CmdDescription,
			Description: "Get module description",
		},
		{
			Name:        CmdHelp,
			Description: "Get command help",
		},
	}
	return append(baseCommands, m.commands...)
}

// AddCommand adds a command to the module
func (m *BaseModule) AddCommand(cmd Command) {
	m.commands = append(m.commands, cmd)
}

// Helper functions for repository operations
func (m *BaseModule) AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error) {
	if m.repo == nil {
		return "", fmt.Errorf("module not initialized")
	}
	return m.repo.AddNode(content, nodeType, meta)
}

func (m *BaseModule) GetNode(id string) (*Node, error) {
	if m.repo == nil {
		return nil, fmt.Errorf("module not initialized")
	}
	return m.repo.GetNode(id)
}

func (m *BaseModule) AddLink(source, target, linkType string, meta map[string]interface{}) error {
	if m.repo == nil {
		return fmt.Errorf("module not initialized")
	}
	return m.repo.AddLink(source, target, linkType, meta)
}
