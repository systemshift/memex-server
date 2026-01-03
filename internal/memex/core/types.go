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

	// Version tracking
	VersionID  string // Unique per version (e.g., "person:alice:v3")
	Version    int    // Sequential version number (1, 2, 3...)
	IsCurrent  bool   // Only one version is current
	ChangeNote string // Why this version was created
	ChangedBy  string // Who made the change
}

// VersionInfo provides metadata about a specific version
type VersionInfo struct {
	Version    int       `json:"version"`
	VersionID  string    `json:"version_id"`
	Modified   time.Time `json:"modified"`
	ChangeNote string    `json:"change_note,omitempty"`
	ChangedBy  string    `json:"changed_by,omitempty"`
	IsCurrent  bool      `json:"is_current"`
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
