package module

import "time"

// Module defines the interface for Memex modules
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

// Command represents a module command
type Command struct {
	Name        string   // Command name (e.g., "add", "status")
	Description string   // Command description
	Usage       string   // Usage example (e.g., "git add <file>")
	Args        []string // Expected arguments
}

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

// Repository defines operations modules can perform
type Repository interface {
	// Node operations
	AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error)
	GetNode(id string) (*Node, error)
	DeleteNode(id string) error
	ListNodes() ([]string, error)
	GetContent(id string) ([]byte, error)

	// Link operations
	AddLink(source, target, linkType string, meta map[string]interface{}) error
	GetLinks(nodeID string) ([]*Link, error)
	DeleteLink(source, target, linkType string) error

	// Query operations
	QueryNodesByModule(moduleID string) ([]*Node, error)
	QueryLinksByModule(moduleID string) ([]*Link, error)
}
