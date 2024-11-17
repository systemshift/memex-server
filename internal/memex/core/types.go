package core

import (
	"time"
)

// Object represents stored content with metadata
type Object struct {
	ID       string         // Unique identifier
	Content  []byte         // Raw content (for backward compatibility)
	Chunks   []string       // List of chunk hashes that make up the content
	Type     string         // Content type
	Version  int            // Version number
	Created  time.Time      // Creation timestamp
	Modified time.Time      // Last modified
	Meta     map[string]any // Flexible metadata
}

// Link represents relationship between objects
type Link struct {
	Source string         // Source object ID
	Target string         // Target object ID
	Type   string         // Relationship type
	Meta   map[string]any // Link metadata
	// New fields for chunk-level linking
	SourceChunk string // Optional: specific chunk in source object
	TargetChunk string // Optional: specific chunk in target object
}

// Repository manages objects and their relationships
type Repository interface {
	// Repository Operations
	Init(path string) error
	Open(path string) error
	Close() error

	// Object Operations
	Add(content []byte, contentType string, meta map[string]any) (string, error)
	Get(id string) (Object, error)
	Update(id string, content []byte) error
	Delete(id string) error

	// Link Operations
	Link(source, target string, linkType string, meta map[string]any) error
	Unlink(source, target string) error
	GetLinks(id string) ([]Link, error)

	// Version Operations
	GetVersion(id string, version int) (Object, error)
	ListVersions(id string) ([]int, error)

	// Query Operations
	List() []string
	FindByType(contentType string) []Object
	Search(query map[string]any) []Object

	// Chunk Operations
	GetChunk(hash string) ([]byte, error)
	GetObjectChunks(id string) ([][]byte, error)
	LinkChunks(sourceID, sourceChunk, targetID, targetChunk string, linkType string, meta map[string]any) error
}

// ObjectStore handles the storage and retrieval of objects
type ObjectStore interface {
	// Store stores an object and returns its ID
	Store(obj Object) (string, error)

	// Load retrieves an object by ID
	Load(id string) (Object, error)

	// Delete removes an object
	Delete(id string) error

	// List returns all object IDs
	List() []string

	// StoreChunk stores a content chunk
	StoreChunk(hash string, content []byte) error

	// LoadChunk retrieves a content chunk
	LoadChunk(hash string) ([]byte, error)
}

// LinkStore handles the storage and retrieval of links
type LinkStore interface {
	// Store stores a link
	Store(link Link) error

	// Delete removes a link
	Delete(source, target string) error

	// GetBySource returns all links from a source
	GetBySource(source string) []Link

	// GetByTarget returns all links to a target
	GetByTarget(target string) []Link

	// GetByChunk returns all links involving a specific chunk
	GetByChunk(hash string) []Link
}

// VersionStore handles version tracking
type VersionStore interface {
	// Store stores a version of an object
	Store(id string, version int, chunks []string) error

	// Load retrieves a specific version
	Load(id string, version int) ([]string, error)

	// List returns all versions of an object
	List(id string) []int
}

// MetaStore handles metadata operations
type MetaStore interface {
	// Store stores metadata for an object
	Store(id string, meta map[string]any) error

	// Load retrieves metadata for an object
	Load(id string) (map[string]any, error)

	// Update updates metadata for an object
	Update(id string, meta map[string]any) error

	// Search finds objects matching metadata criteria
	Search(query map[string]any) []string
}
