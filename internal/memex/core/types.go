package core

import (
	"time"
)

// Node represents a node in the DAG
type Node struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Meta     map[string]any `json:"meta"`
	Created  time.Time      `json:"created"`
	Modified time.Time      `json:"modified"`
	Versions []Version      `json:"versions"`
	Links    []Link         `json:"links"`
	Current  string         `json:"current"`
}

// Version represents a specific version of content
type Version struct {
	Hash      string         `json:"hash"`
	Chunks    []string       `json:"chunks"`
	Created   time.Time      `json:"created"`
	Meta      map[string]any `json:"meta"`
	Available bool           `json:"available"`
}

// Link represents a relationship between nodes
type Link struct {
	Source      string         `json:"source"`
	Target      string         `json:"target"`
	Type        string         `json:"type"`
	Meta        map[string]any `json:"meta"`
	SourceChunk string         `json:"sourceChunk"`
	TargetChunk string         `json:"targetChunk"`
}

// Root represents the root of the DAG
type Root struct {
	Hash     string    `json:"hash"`
	Modified time.Time `json:"modified"`
	Nodes    []string  `json:"nodes"`
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
