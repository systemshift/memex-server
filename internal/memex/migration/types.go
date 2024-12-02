package migration

import (
	"time"

	"memex/internal/memex/core"
)

// Version is the current export format version
const Version uint32 = 1

// Constants for metadata and content size limits
const (
	MaxMetaSize = 4096    // Maximum metadata size in bytes (4KB)
	MaxDataSize = 1 << 20 // Maximum data size in bytes (1MB)
)

// Export represents an exported repository
type Export struct {
	Version  uint32          `json:"version"`
	Created  time.Time       `json:"created"`
	Modified time.Time       `json:"modified"`
	Nodes    []*core.Node    `json:"nodes"`
	Links    []*core.Link    `json:"links"`
	Chunks   map[string]bool `json:"chunks"`
}

// ExportedLink represents a link in the export format
type ExportedLink struct {
	Source string         `json:"source"`
	Target string         `json:"target"`
	Type   string         `json:"type"`
	Meta   map[string]any `json:"meta"`
}

// ExportOptions configures how the export is performed
type ExportOptions struct {
	Depth int // How many levels of links to follow (0 = seed nodes only)
}

// ExportManifest represents the manifest of an export
type ExportManifest struct {
	Version  uint32    `json:"version"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
	Nodes    int       `json:"nodes"`     // Number of nodes
	Edges    int       `json:"edges"`     // Number of edges
	Chunks   int       `json:"chunks"`    // Number of chunks
	NodeIDs  []string  `json:"node_ids"`  // List of node IDs
	EdgeIDs  []string  `json:"edge_ids"`  // List of edge IDs
	ChunkIDs []string  `json:"chunk_ids"` // List of chunk hashes
}

// ImportOptions configures how content is imported
type ImportOptions struct {
	OnConflict ConflictStrategy // How to handle ID conflicts
	Merge      bool             // Whether to merge with existing content
	Prefix     string           // Optional prefix for imported node IDs
}

// ConflictStrategy determines how to handle ID conflicts
type ConflictStrategy int

const (
	Skip ConflictStrategy = iota
	Replace
	Rename
)
