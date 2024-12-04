package core

import (
	"time"
)

// Node represents a node in the graph
type Node struct {
	ID       string                 // Node identifier
	Type     string                 // Node type
	Content  []byte                 // Node content
	Meta     map[string]interface{} // Node metadata
	Created  time.Time              // Creation timestamp
	Modified time.Time              // Last modification timestamp
}

// Link represents a relationship between nodes
type Link struct {
	Source   string                 // Source node ID
	Target   string                 // Target node ID
	Type     string                 // Link type
	Meta     map[string]interface{} // Link metadata
	Created  time.Time              // Creation timestamp
	Modified time.Time              // Last modification timestamp
}

// Repository represents a content repository
type Repository interface {
	// Node operations
	AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error)
	AddNodeWithID(id string, content []byte, nodeType string, meta map[string]interface{}) error
	GetNode(id string) (*Node, error)
	DeleteNode(id string) error
	ListNodes() ([]string, error)

	// Link operations
	AddLink(source, target, linkType string, meta map[string]interface{}) error
	GetLinks(nodeID string) ([]*Link, error)
	DeleteLink(source, target, linkType string) error

	// Content operations
	GetContent(id string) ([]byte, error)

	// Repository operations
	Close() error
}
