package link

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"memex/internal/memex/core"
)

// IndexEntry represents a link index entry
type IndexEntry struct {
	ID     [32]byte // SHA-256 hash identifier
	Offset uint64   // File offset to data
	Length uint32   // Length of data
	Flags  uint32   // Entry flags
}

// LinkData represents a link's binary format
type LinkData struct {
	ID       [32]byte // SHA-256 hash identifier
	Source   [32]byte // Source node ID
	Target   [32]byte // Target node ID
	Type     [32]byte // Fixed-size type string
	Created  int64    // Unix timestamp
	Modified int64    // Unix timestamp
	MetaLen  uint32   // Length of metadata JSON
	Meta     []byte   // Metadata as JSON
}

// Constants for link entries
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

// ToCore converts LinkData to core.Link
func (l *LinkData) ToCore() *core.Link {
	return &core.Link{
		Source: fmt.Sprintf("%x", l.Source),
		Target: fmt.Sprintf("%x", l.Target),
		Type:   fixedTypeToString(l.Type),
		Meta:   make(map[string]any), // Will be populated from Meta JSON
	}
}

// FromCore converts core.Link to LinkData
func (l *LinkData) FromCore(link *core.Link) error {
	// Parse source ID
	sourceBytes, err := decodeID(link.Source)
	if err != nil {
		return fmt.Errorf("decoding source ID: %w", err)
	}
	l.Source = sourceBytes

	// Parse target ID
	targetBytes, err := decodeID(link.Target)
	if err != nil {
		return fmt.Errorf("decoding target ID: %w", err)
	}
	l.Target = targetBytes

	// Convert type to fixed-size
	l.Type = stringToFixedType(link.Type)

	// Set timestamps
	now := time.Now().Unix()
	l.Created = now
	l.Modified = now

	// Marshal metadata
	if link.Meta != nil {
		metaBytes, err := json.Marshal(link.Meta)
		if err != nil {
			return fmt.Errorf("marshaling metadata: %w", err)
		}
		l.Meta = metaBytes
		l.MetaLen = uint32(len(metaBytes))
	}

	// Calculate link ID
	idBytes := sha256.Sum256(append(append(l.Source[:], l.Target[:]...), []byte(link.Type)...))
	l.ID = idBytes

	return nil
}

// Size returns the size of LinkData in bytes
func (l *LinkData) Size() int {
	return 32 + // ID
		32 + // Source
		32 + // Target
		32 + // Type
		8 + // Created
		8 + // Modified
		4 + // MetaLen
		len(l.Meta) // Meta
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

// decodeID decodes a hex string into a 32-byte array
func decodeID(id string) ([32]byte, error) {
	var result [32]byte
	if len(id) != 64 {
		return result, fmt.Errorf("invalid ID length: got %d want 64", len(id))
	}

	bytes, err := hex.DecodeString(id)
	if err != nil {
		return result, fmt.Errorf("decoding hex string: %w", err)
	}

	copy(result[:], bytes)
	return result, nil
}
