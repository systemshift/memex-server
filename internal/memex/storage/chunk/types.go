package chunk

// IndexEntry represents a chunk index entry
type IndexEntry struct {
	ID     [32]byte // SHA-256 hash identifier
	Offset uint64   // File offset to data
	Length uint32   // Length of data
	Flags  uint32   // Entry flags
}

// Constants for index entries
const (
	IndexEntrySize = 48 // Size of IndexEntry in bytes (hash + offset + length + flags)
)

// Flags for index entries
const (
	FlagNone     uint32 = 0
	FlagDeleted  uint32 = 1 << 0
	FlagModified uint32 = 1 << 1
	FlagTemp     uint32 = 1 << 2
)
