package storage

import (
	"time"
)

// Common constants for storage implementation
const (
	maxMetaLen     = 1024 * 1024         // 1MB max metadata size
	indexEntrySize = 32 + 8 + 4          // ID (32) + Offset (8) + Length (4)
	nodeHeaderSize = 32 + 32 + 8 + 8 + 4 // Size of fixed fields in NodeData
	edgeHeaderSize = 32 + 32 + 32 + 4    // Size of fixed fields in EdgeData
)

// IndexEntry represents an index entry in the storage file
type IndexEntry struct {
	ID     [32]byte // Node/Edge ID
	Offset uint64   // File offset to data
	Length uint32   // Length of data
}

// Transaction represents an atomic storage operation
type Transaction struct {
	store    *MXStore
	startPos int64
	writes   [][]byte
	indexes  []IndexEntry
	isEdge   bool // Whether this transaction is for an edge
}

// NodeData represents the binary format for node storage
type NodeData struct {
	ID       [32]byte // Node ID
	Type     [32]byte // Node type
	Created  int64    // Unix timestamp
	Modified int64    // Unix timestamp
	MetaLen  uint32   // Length of metadata
	Meta     []byte   // JSON-encoded metadata
}

// EdgeData represents the binary format for edge storage
type EdgeData struct {
	Source  [32]byte // Source node ID
	Target  [32]byte // Target node ID
	Type    [32]byte // Edge type
	MetaLen uint32   // Length of metadata
	Meta    []byte   // Raw metadata
}

// Header represents the storage file header
type Header struct {
	Version    uint32    // File format version
	Created    time.Time // Repository creation time
	Modified   time.Time // Last modification time
	NodeCount  uint32    // Number of nodes
	EdgeCount  uint32    // Number of edges
	NodeIndex  uint64    // Offset to node index
	EdgeIndex  uint64    // Offset to edge index
	ChunkCount uint32    // Number of chunks
}
