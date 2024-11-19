package core

import (
	"time"
)

// Node represents a node in the DAG
type Node struct {
	ID       string         // Stable identifier
	Type     string         // Type of node (file, directory, etc)
	Meta     map[string]any // Metadata
	Created  time.Time      // Creation timestamp
	Modified time.Time      // Last modified timestamp
	Versions []Version      // Version history
	Links    []Link         // Links to other nodes
	Current  string         // Hash of current content version
}

// Version represents a specific version of content
type Version struct {
	Hash      string         // Content hash
	Chunks    []string       // Chunk hashes that make up this version
	Created   time.Time      // When this version was created
	Meta      map[string]any // Version-specific metadata
	Available bool           // Whether content is available or pruned
}

// Link represents a relationship between nodes
type Link struct {
	Source      string         // Source node ID
	Target      string         // Target node ID
	Type        string         // Type of relationship
	Meta        map[string]any // Link metadata
	SourceChunk string         // Optional: specific chunk in source
	TargetChunk string         // Optional: specific chunk in target
}

// Root represents the root of the DAG
type Root struct {
	Hash     string    // Hash of current state
	Modified time.Time // Last modified timestamp
	Nodes    []string  // List of node IDs
}

// Repository defines the interface for content storage
type Repository interface {
	// Node operations
	AddNode(content []byte, nodeType string, meta map[string]any) (string, error)
	GetNode(id string) (Node, error)
	UpdateNode(id string, content []byte, meta map[string]any) error
	DeleteNode(id string) error

	// Version operations
	GetVersion(nodeID string, hash string) (Version, error)
	PruneVersion(nodeID string, hash string) error
	RestoreVersion(nodeID string, hash string) error

	// Link operations
	AddLink(source, target, linkType string, meta map[string]any) error
	DeleteLink(source, target string) error
	GetLinks(nodeID string) ([]Link, error)

	// Root operations
	GetRoot() (Root, error)
	UpdateRoot() error // Recalculates root hash

	// Search operations
	Search(query map[string]any) ([]Node, error)
	FindByType(nodeType string) ([]Node, error)

	// Chunk operations
	GetChunk(hash string) ([]byte, error)
	HasChunk(hash string) bool
}

// ChunkStore defines the interface for chunk storage
type ChunkStore interface {
	Store(content []byte) (string, error) // Returns chunk hash
	Load(hash string) ([]byte, error)     // Loads chunk content
	Delete(hash string) error             // Deletes a chunk
	Has(hash string) bool                 // Checks if chunk exists
	Dedupe() error                        // Deduplicates chunks
}
