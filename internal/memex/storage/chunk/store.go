package chunk

import (
	"crypto/sha256"
	"fmt"
	"os"

	"memex/internal/memex/storage/common"
)

// Store manages content chunks
type Store struct {
	file  *common.File
	index []IndexEntry
	locks *common.LockManager
}

// NewStore creates a new chunk store
func NewStore(file *common.File, locks *common.LockManager) *Store {
	return &Store{
		file:  file,
		index: make([]IndexEntry, 0),
		locks: locks,
	}
}

// Store adds a chunk to the store
func (s *Store) Store(content []byte) (string, error) {
	return s.locks.WithChunkLockString(func() (string, error) {
		// Calculate hash and checksum
		hash := sha256.Sum256(content)
		checksum := common.CalculateChecksum(content)

		// Check if chunk already exists
		hashStr := fmt.Sprintf("%x", hash)
		for _, entry := range s.index {
			if entry.ID == hash {
				return hashStr, nil
			}
		}

		// Create chunk data
		chunk := ChunkData{
			Content:  content,
			Hash:     hash,
			Length:   uint32(len(content)),
			Checksum: checksum,
		}

		// Write chunk data
		offset, err := s.writeChunk(chunk)
		if err != nil {
			return "", fmt.Errorf("writing chunk: %w", err)
		}

		// Create index entry
		entry := IndexEntry{
			ID:     hash,
			Offset: offset,
			Length: chunk.Length,
			Flags:  FlagNone,
		}

		// Add to index
		s.index = append(s.index, entry)

		return hashStr, nil
	})
}

// Get retrieves a chunk by its hash
func (s *Store) Get(hash string) ([]byte, error) {
	return s.locks.WithChunkRLockBytes(func() ([]byte, error) {
		// Find chunk in index
		var entry IndexEntry
		found := false
		for _, e := range s.index {
			if fmt.Sprintf("%x", e.ID) == hash {
				entry = e
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("chunk not found: %s", hash)
		}

		// Read chunk data
		chunk, err := s.readChunk(entry)
		if err != nil {
			return nil, fmt.Errorf("reading chunk: %w", err)
		}

		// Verify hash and checksum
		actualHash := sha256.Sum256(chunk.Content)
		if actualHash != chunk.Hash {
			return nil, fmt.Errorf("chunk hash mismatch")
		}

		if !common.ValidateChecksum(chunk.Content, chunk.Checksum) {
			return nil, fmt.Errorf("chunk checksum mismatch")
		}

		return chunk.Content, nil
	})
}

// writeChunk writes a chunk to the file
func (s *Store) writeChunk(chunk ChunkData) (uint64, error) {
	// Get current position
	pos, err := s.file.Seek(0, os.SEEK_END)
	if err != nil {
		return 0, fmt.Errorf("seeking to end: %w", err)
	}

	// Write chunk header
	if err := s.file.WriteUint32(common.ChunkMagic); err != nil {
		return 0, fmt.Errorf("writing magic: %w", err)
	}

	// Write chunk length
	if err := s.file.WriteUint32(chunk.Length); err != nil {
		return 0, fmt.Errorf("writing length: %w", err)
	}

	// Write chunk hash
	if err := s.file.WriteBytes(chunk.Hash[:]); err != nil {
		return 0, fmt.Errorf("writing hash: %w", err)
	}

	// Write chunk checksum
	if err := s.file.WriteUint32(chunk.Checksum); err != nil {
		return 0, fmt.Errorf("writing checksum: %w", err)
	}

	// Write chunk content
	if err := s.file.WriteBytes(chunk.Content); err != nil {
		return 0, fmt.Errorf("writing content: %w", err)
	}

	return uint64(pos), nil
}

// readChunk reads a chunk from the file
func (s *Store) readChunk(entry IndexEntry) (*ChunkData, error) {
	// Seek to chunk start
	if _, err := s.file.Seek(int64(entry.Offset), os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("seeking to chunk: %w", err)
	}

	// Read and verify magic
	magic, err := s.file.ReadUint32()
	if err != nil {
		return nil, fmt.Errorf("reading magic: %w", err)
	}
	if magic != common.ChunkMagic {
		return nil, fmt.Errorf("invalid chunk magic")
	}

	// Read chunk length
	length, err := s.file.ReadUint32()
	if err != nil {
		return nil, fmt.Errorf("reading length: %w", err)
	}
	if length != entry.Length {
		return nil, fmt.Errorf("chunk length mismatch: got %d want %d", length, entry.Length)
	}

	// Read chunk hash
	hashBytes, err := s.file.ReadBytes(32)
	if err != nil {
		return nil, fmt.Errorf("reading hash: %w", err)
	}
	var hash [32]byte
	copy(hash[:], hashBytes)
	if hash != entry.ID {
		return nil, fmt.Errorf("chunk hash mismatch")
	}

	// Read chunk checksum
	checksum, err := s.file.ReadUint32()
	if err != nil {
		return nil, fmt.Errorf("reading checksum: %w", err)
	}

	// Read chunk content
	content, err := s.file.ReadBytes(int(length))
	if err != nil {
		return nil, fmt.Errorf("reading content: %w", err)
	}

	return &ChunkData{
		Content:  content,
		Hash:     hash,
		Length:   length,
		Checksum: checksum,
	}, nil
}

// LoadIndex loads the chunk index from the file
func (s *Store) LoadIndex(offset uint64, count uint32) error {
	return s.locks.WithChunkLock(func() error {
		// Seek to index start
		if _, err := s.file.Seek(int64(offset), os.SEEK_SET); err != nil {
			return fmt.Errorf("seeking to index: %w", err)
		}

		// Read index entries
		s.index = make([]IndexEntry, count)
		for i := uint32(0); i < count; i++ {
			var entry IndexEntry
			if err := s.readIndexEntry(&entry); err != nil {
				return fmt.Errorf("reading index entry: %w", err)
			}
			s.index[i] = entry
		}

		return nil
	})
}

// SaveIndex saves the chunk index to the file
func (s *Store) SaveIndex(offset uint64) error {
	return s.locks.WithChunkLock(func() error {
		// Seek to index start
		if _, err := s.file.Seek(int64(offset), os.SEEK_SET); err != nil {
			return fmt.Errorf("seeking to index: %w", err)
		}

		// Write index entries
		for _, entry := range s.index {
			if err := s.writeIndexEntry(entry); err != nil {
				return fmt.Errorf("writing index entry: %w", err)
			}
		}

		return nil
	})
}

// readIndexEntry reads a single index entry
func (s *Store) readIndexEntry(entry *IndexEntry) error {
	// Read ID
	idBytes, err := s.file.ReadBytes(32)
	if err != nil {
		return fmt.Errorf("reading ID: %w", err)
	}
	copy(entry.ID[:], idBytes)

	// Read offset
	offset, err := s.file.ReadUint64()
	if err != nil {
		return fmt.Errorf("reading offset: %w", err)
	}
	entry.Offset = offset

	// Read length
	length, err := s.file.ReadUint32()
	if err != nil {
		return fmt.Errorf("reading length: %w", err)
	}
	entry.Length = length

	// Read flags
	flags, err := s.file.ReadUint32()
	if err != nil {
		return fmt.Errorf("reading flags: %w", err)
	}
	entry.Flags = flags

	return nil
}

// writeIndexEntry writes a single index entry
func (s *Store) writeIndexEntry(entry IndexEntry) error {
	// Write ID
	if err := s.file.WriteBytes(entry.ID[:]); err != nil {
		return fmt.Errorf("writing ID: %w", err)
	}

	// Write offset
	if err := s.file.WriteUint64(entry.Offset); err != nil {
		return fmt.Errorf("writing offset: %w", err)
	}

	// Write length
	if err := s.file.WriteUint32(entry.Length); err != nil {
		return fmt.Errorf("writing length: %w", err)
	}

	// Write flags
	if err := s.file.WriteUint32(entry.Flags); err != nil {
		return fmt.Errorf("writing flags: %w", err)
	}

	return nil
}

// Count returns the number of chunks in the store
func (s *Store) Count() int {
	return len(s.index)
}
