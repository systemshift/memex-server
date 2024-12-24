package types

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

// Command represents a module command
type Command struct {
	Name        string   // Command name (e.g., "add", "status")
	Description string   // Command description
	Usage       string   // Usage example (e.g., "git add <file>")
	Args        []string // Expected arguments
}

// Command types
const (
	CmdID          = "id"          // Get module ID
	CmdName        = "name"        // Get module name
	CmdDescription = "description" // Get module description
	CmdHelp        = "help"        // Get command help
)

// Status represents a command execution status
type Status int

const (
	StatusSuccess Status = iota
	StatusError
	StatusUnsupported
)

// Response represents a command execution response
type Response struct {
	Status Status      // Execution status
	Data   interface{} // Response data
	Error  string      // Error message if any
	Meta   interface{} // Additional metadata
}

// Handler handles module commands
type Handler interface {
	Handle(cmd Command) Response
}

// Repository defines the interface for interacting with a memex repository
type Repository interface {
	// Node operations
	AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error)
	GetNode(id string) (*Node, error)
	DeleteNode(id string) error

	// Link operations
	AddLink(source, target, linkType string, meta map[string]interface{}) error
	GetLinks(nodeID string) ([]*Link, error)
	DeleteLink(source, target, linkType string) error

	// Query operations
	QueryNodes(query Query) ([]*Node, error)
	QueryLinks(query Query) ([]*Link, error)
}

// Node represents a node in the repository
type Node struct {
	ID      string                 // Node identifier
	Type    string                 // Node type
	Content []byte                 // Node content
	Meta    map[string]interface{} // Node metadata
}

// Link represents a link between nodes
type Link struct {
	Source string                 // Source node ID
	Target string                 // Target node ID
	Type   string                 // Link type
	Meta   map[string]interface{} // Link metadata
}

// Query represents a repository query
type Query struct {
	ModuleID string                 // Module ID to filter by
	Type     string                 // Node/Link type to filter by
	Meta     map[string]interface{} // Metadata to filter by
}

// QueryResult represents a query result
type QueryResult struct {
	Nodes []*Node // Matching nodes
	Links []*Link // Matching links
}
