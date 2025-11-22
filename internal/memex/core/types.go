package core

import (
	"time"
)

// Node represents a node in the graph
type Node struct {
	ID        string
	Type      string
	Content   []byte
	Meta      map[string]interface{}
	Created   time.Time
	Modified  time.Time
	Deleted   bool      // Tombstone flag
	DeletedAt time.Time // When node was deleted
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

	// Close closes the repository
	Close() error
}
