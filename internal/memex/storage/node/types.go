package node

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"memex/internal/memex/core"
)

// IndexEntry represents a node index entry
type IndexEntry struct {
	ID     [32]byte // SHA-256 hash identifier
	Offset uint64   // File offset to data
	Length uint32   // Length of data
	Flags  uint32   // Entry flags
}

// NodeData represents a node's binary format
type NodeData struct {
	ID       [32]byte // SHA-256 hash identifier
	Type     [32]byte // Fixed-size type string
	Created  int64    // Unix timestamp
	Modified int64    // Unix timestamp
	MetaLen  uint32   // Length of metadata JSON
	Meta     []byte   // Metadata as JSON
}

// Constants for node entries
const (
	IndexEntrySize = 48   // Size of IndexEntry in bytes (hash + offset + length + flags)
	MaxMetaSize    = 4096 // Maximum metadata size in bytes
)

// Flags for index entries
const (
	FlagNone     uint32 = 0
	FlagDeleted  uint32 = 1 << 0
	FlagModified uint32 = 1 << 1
	FlagTemp     uint32 = 1 << 2
)

// ToCore converts NodeData to core.Node
func (n *NodeData) ToCore() *core.Node {
	return &core.Node{
		ID:       fmt.Sprintf("%x", n.ID),
		Type:     fixedTypeToString(n.Type),
		Created:  time.Unix(n.Created, 0),
		Modified: time.Unix(n.Modified, 0),
		Meta:     make(map[string]any), // Will be populated from Meta JSON
	}
}

// FromCore converts core.Node to NodeData
func (n *NodeData) FromCore(node *core.Node) error {
	// Parse ID from hex string
	if node.ID != "" {
		idBytes, err := hex.DecodeString(node.ID)
		if err != nil {
			return fmt.Errorf("decoding ID: %w", err)
		}
		copy(n.ID[:], idBytes)
	}

	// Convert type to fixed-size
	n.Type = stringToFixedType(node.Type)
	n.Created = node.Created.Unix()
	n.Modified = node.Modified.Unix()

	// Marshal metadata
	if node.Meta != nil {
		metaBytes, err := json.Marshal(node.Meta)
		if err != nil {
			return fmt.Errorf("marshaling metadata: %w", err)
		}
		n.Meta = metaBytes
		n.MetaLen = uint32(len(metaBytes))
	}

	return nil
}

// Helper functions for fixed-size type conversion
func stringToFixedType(s string) [32]byte {
	var fixed [32]byte
	copy(fixed[:], []byte(s))
	return fixed
}

func fixedTypeToString(fixed [32]byte) string {
	n := 0
	for i, b := range fixed {
		if b == 0 {
			break
		}
		n = i + 1
	}
	return string(fixed[:n])
}

// Size returns the size of NodeData in bytes
func (n *NodeData) Size() int {
	return 32 + // ID
		32 + // Type
		8 + // Created
		8 + // Modified
		4 + // MetaLen
		len(n.Meta) // Meta
}
