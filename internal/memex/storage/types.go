package storage

import (
	"time"

	"memex/internal/memex/core"
)

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

// IndexEntry represents an index entry in the store
type IndexEntry struct {
	ID     [32]byte // SHA-256 hash identifier
	Offset uint64   // File offset to data
	Length uint32   // Length of data
	Flags  uint32   // Entry flags
}

// Chunk represents a piece of content
type Chunk struct {
	ID       [32]byte // SHA-256 hash identifier
	Content  []byte   // Chunk content
	Length   uint32   // Content length
	Checksum uint32   // CRC32 checksum
}

// Constants for storage
const (
	Version        uint32 = 1    // Current file format version
	IndexEntrySize        = 48   // Size of IndexEntry in bytes (hash + offset + length + flags)
	MaxMetaSize           = 4096 // Maximum metadata size in bytes
	MaxChunkSize          = 4096 // Maximum chunk size in bytes
)

// Flags for index entries
const (
	FlagNone     uint32 = 0
	FlagDeleted  uint32 = 1 << 0
	FlagModified uint32 = 1 << 1
	FlagTemp     uint32 = 1 << 2
)

// ToData converts the header to its binary format
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

// FromData converts binary format to header
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

// Store represents the storage interface
type Store interface {
	// Path returns the repository path
	Path() string

	// Node operations
	GetNode(id string) (*core.Node, error)
	AddNode(content []byte, nodeType string, meta map[string]any) (string, error)
	DeleteNode(id string) error

	// Link operations
	GetLinks(nodeID string) ([]*core.Link, error)
	AddLink(sourceID, targetID, linkType string, meta map[string]any) error
	DeleteLink(sourceID, targetID, linkType string) error

	// Chunk operations
	GetChunk(hash string) ([]byte, error)
	StoreChunk(content []byte) (string, error)

	// Content operations
	ReconstructContent(contentHash string) ([]byte, error)

	// Close closes the store
	Close() error
}
