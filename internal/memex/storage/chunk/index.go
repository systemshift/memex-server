package chunk

import (
	"fmt"
	"sync"

	"memex/internal/memex/storage/common"
)

// Index manages chunk index entries
type Index struct {
	entries []IndexEntry
	mutex   sync.RWMutex
}

// NewIndex creates a new chunk index
func NewIndex() *Index {
	return &Index{
		entries: make([]IndexEntry, 0),
	}
}

// Add adds a new entry to the index
func (idx *Index) Add(entry IndexEntry) {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()
	idx.entries = append(idx.entries, entry)
}

// Find finds an entry by its ID
func (idx *Index) Find(id [32]byte) (IndexEntry, bool) {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()

	for _, entry := range idx.entries {
		if entry.ID == id {
			return entry, true
		}
	}
	return IndexEntry{}, false
}

// Count returns the number of entries
func (idx *Index) Count() int {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()
	return len(idx.entries)
}

// Load loads the index from the file
func (idx *Index) Load(file *common.File, offset uint64, count uint32) error {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()

	// Seek to index start
	if _, err := file.Seek(int64(offset), 0); err != nil {
		return fmt.Errorf("seeking to index: %w", err)
	}

	// Read entries
	idx.entries = make([]IndexEntry, count)
	for i := uint32(0); i < count; i++ {
		var entry IndexEntry

		// Read ID
		idBytes, err := file.ReadBytes(32)
		if err != nil {
			return fmt.Errorf("reading ID: %w", err)
		}
		copy(entry.ID[:], idBytes)

		// Read offset
		offset, err := file.ReadUint64()
		if err != nil {
			return fmt.Errorf("reading offset: %w", err)
		}
		entry.Offset = offset

		// Read length
		length, err := file.ReadUint32()
		if err != nil {
			return fmt.Errorf("reading length: %w", err)
		}
		entry.Length = length

		// Read flags
		flags, err := file.ReadUint32()
		if err != nil {
			return fmt.Errorf("reading flags: %w", err)
		}
		entry.Flags = flags

		idx.entries[i] = entry
	}

	return nil
}

// Save saves the index to the file
func (idx *Index) Save(file *common.File, offset uint64) error {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()

	// Seek to index start
	if _, err := file.Seek(int64(offset), 0); err != nil {
		return fmt.Errorf("seeking to index: %w", err)
	}

	// Write entries
	for _, entry := range idx.entries {
		// Write ID
		if err := file.WriteBytes(entry.ID[:]); err != nil {
			return fmt.Errorf("writing ID: %w", err)
		}

		// Write offset
		if err := file.WriteUint64(entry.Offset); err != nil {
			return fmt.Errorf("writing offset: %w", err)
		}

		// Write length
		if err := file.WriteUint32(entry.Length); err != nil {
			return fmt.Errorf("writing length: %w", err)
		}

		// Write flags
		if err := file.WriteUint32(entry.Flags); err != nil {
			return fmt.Errorf("writing flags: %w", err)
		}
	}

	return nil
}

// Entries returns a copy of all entries
func (idx *Index) Entries() []IndexEntry {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()

	entries := make([]IndexEntry, len(idx.entries))
	copy(entries, idx.entries)
	return entries
}
