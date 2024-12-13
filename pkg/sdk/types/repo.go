package types

// Node represents a node in the graph
type Node struct {
	ID      string
	Type    string
	Content []byte
	Meta    map[string]interface{}
}

// Link represents a relationship between nodes
type Link struct {
	Source string
	Target string
	Type   string
	Meta   map[string]interface{}
}

// Query represents a search query for nodes or links
type Query struct {
	ModuleID string                 // Filter by module ID
	Type     string                 // Filter by type
	Meta     map[string]interface{} // Filter by metadata
}

// Repository provides access to memex operations
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
