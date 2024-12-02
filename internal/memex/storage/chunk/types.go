package chunk

// MaxChunkSize is the maximum size for fixed-size chunks (4KB)
const MaxChunkSize = 4096

// Flag constants for chunk entries
const (
	FlagNone    uint32 = 0
	FlagDeleted uint32 = 1 << 0
	FlagTemp    uint32 = 1 << 1
)

// ChunkData represents a chunk in memory
type ChunkData struct {
	Content  []byte
	Hash     [32]byte
	Length   uint32
	Checksum uint32
}

// IndexEntry represents a chunk index entry
type IndexEntry struct {
	ID     [32]byte // SHA-256 hash identifier
	Offset uint64   // File offset to chunk data
	Length uint32   // Length of chunk data
	Flags  uint32   // Chunk flags
}
