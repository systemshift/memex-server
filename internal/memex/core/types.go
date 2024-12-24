package core

import "time"

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
