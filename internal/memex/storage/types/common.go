package types

import (
	"time"
)

// Common types shared between storage packages

// Node represents a node in the graph
type Node struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Created  time.Time      `json:"created"`
	Modified time.Time      `json:"modified"`
	Meta     map[string]any `json:"meta,omitempty"`
}

// Link represents a link between nodes
type Link struct {
	Source string         `json:"source"`
	Target string         `json:"target"`
	Type   string         `json:"type"`
	Meta   map[string]any `json:"meta,omitempty"`
}

// IndexEntry represents an index entry in the store
type IndexEntry struct {
	ID     [32]byte // SHA-256 hash identifier
	Offset uint64   // File offset to data
	Length uint32   // Length of data
	Flags  uint32   // Entry flags
}

// Header represents the file header with metadata
type Header struct {
	Version    uint32    // File format version
	Created    time.Time // When the repository was created
	Modified   time.Time // When the repository was last modified
	NodeCount  uint32    // Number of nodes
	EdgeCount  uint32    // Number of edges
	ChunkCount uint32    // Number of chunks
	NodeIndex  uint64    // Offset to node index
	EdgeIndex  uint64    // Offset to edge index
	ChunkIndex uint64    // Offset to chunk index
}

// HeaderData represents the binary format of the file header
type HeaderData struct {
	Version    uint32 // File format version
	Created    int64  // When the repository was created (Unix timestamp)
	Modified   int64  // When the repository was last modified (Unix timestamp)
	NodeCount  uint32 // Number of nodes
	EdgeCount  uint32 // Number of edges
	ChunkCount uint32 // Number of chunks
	NodeIndex  uint64 // Offset to node index
	EdgeIndex  uint64 // Offset to edge index
	ChunkIndex uint64 // Offset to chunk index
}

// Constants for storage
const (
	Version        uint32 = 1    // Current file format version
	IndexEntrySize        = 48   // Size of IndexEntry in bytes (hash + offset + length + flags)
	MaxMetaSize           = 4096 // Maximum metadata size in bytes
	MaxChunkSize          = 4096 // Maximum chunk size in bytes
)

// ToData converts a Header to HeaderData
func (h *Header) ToData() HeaderData {
	return HeaderData{
		Version:    h.Version,
		Created:    h.Created.Unix(),
		Modified:   h.Modified.Unix(),
		NodeCount:  h.NodeCount,
		EdgeCount:  h.EdgeCount,
		ChunkCount: h.ChunkCount,
		NodeIndex:  h.NodeIndex,
		EdgeIndex:  h.EdgeIndex,
		ChunkIndex: h.ChunkIndex,
	}
}

// FromData converts HeaderData to a Header
func (h *Header) FromData(data HeaderData) {
	h.Version = data.Version
	h.Created = time.Unix(data.Created, 0)
	h.Modified = time.Unix(data.Modified, 0)
	h.NodeCount = data.NodeCount
	h.EdgeCount = data.EdgeCount
	h.ChunkCount = data.ChunkCount
	h.NodeIndex = data.NodeIndex
	h.EdgeIndex = data.EdgeIndex
	h.ChunkIndex = data.ChunkIndex
}
