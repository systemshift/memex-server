package node

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"memex/internal/memex/core"
	"memex/internal/memex/storage/common"
)

// Store manages nodes in the repository
type Store struct {
	file  *common.File
	index *Index
	locks *common.LockManager
}

// NewStore creates a new node store
func NewStore(file *common.File, locks *common.LockManager) *Store {
	return &Store{
		file:  file,
		index: NewIndex(),
		locks: locks,
	}
}

// Add adds a node to the store
func (s *Store) Add(nodeType string, meta map[string]any) (string, error) {
	return s.locks.WithChunkLockString(func() (string, error) {
		// Create node data
		now := time.Now().Unix()
		node := NodeData{
			Type:     stringToFixedType(nodeType),
			Created:  now,
			Modified: now,
		}

		// Marshal metadata
		if meta != nil {
			metaBytes, err := json.Marshal(meta)
			if err != nil {
				return "", fmt.Errorf("marshaling metadata: %w", err)
			}

			if len(metaBytes) > MaxMetaSize {
				return "", fmt.Errorf("metadata too large (max %d bytes)", MaxMetaSize)
			}

			node.MetaLen = uint32(len(metaBytes))
			node.Meta = metaBytes
		}

		// Calculate node ID
		idBytes := sha256.Sum256(append([]byte(nodeType), node.Meta...))
		node.ID = idBytes

		// Write node data
		offset, err := s.writeNode(node)
		if err != nil {
			return "", fmt.Errorf("writing node: %w", err)
		}

		// Create index entry
		entry := IndexEntry{
			ID:     idBytes,
			Offset: offset,
			Length: uint32(node.Size()),
			Flags:  FlagNone,
		}

		// Add to index
		s.index.Add(entry)

		return fmt.Sprintf("%x", idBytes), nil
	})
}

// Get retrieves a node by ID
func (s *Store) Get(id string) (*core.Node, error) {
	return s.locks.WithChunkLockNode(func() (*core.Node, error) {
		// Find node in index
		entry, found := s.index.FindByString(id)
		if !found {
			return nil, fmt.Errorf("node not found: %s", id)
		}

		// Read node data
		nodeData, err := s.readNode(entry)
		if err != nil {
			return nil, fmt.Errorf("reading node: %w", err)
		}

		// Parse metadata
		var meta map[string]any
		if nodeData.MetaLen > 0 {
			if err := json.Unmarshal(nodeData.Meta, &meta); err != nil {
				return nil, fmt.Errorf("parsing metadata: %w", err)
			}
		}

		// Create node
		node := nodeData.ToCore()
		node.Meta = meta

		return node, nil
	})
}

// Delete removes a node from the store
func (s *Store) Delete(id string) error {
	return s.locks.WithChunkLock(func() error {
		// Find node in index
		entry, found := s.index.FindByString(id)
		if !found {
			return fmt.Errorf("node not found: %s", id)
		}

		// Remove from index
		if !s.index.Remove(entry.ID) {
			return fmt.Errorf("failed to remove node from index: %s", id)
		}

		return nil
	})
}

// LoadIndex loads the node index from the file
func (s *Store) LoadIndex(offset uint64, count uint32) error {
	return s.locks.WithChunkLock(func() error {
		return s.index.Load(s.file, offset, count)
	})
}

// SaveIndex saves the node index to the file
func (s *Store) SaveIndex(offset uint64) error {
	return s.locks.WithChunkLock(func() error {
		return s.index.Save(s.file, offset)
	})
}

// Internal methods

func (s *Store) writeNode(node NodeData) (uint64, error) {
	// Get current position
	pos, err := s.file.Seek(0, os.SEEK_END)
	if err != nil {
		return 0, fmt.Errorf("seeking to end: %w", err)
	}

	// Write node magic
	if err := s.file.WriteUint32(common.NodeMagic); err != nil {
		return 0, fmt.Errorf("writing magic: %w", err)
	}

	// Write node ID
	if err := s.file.WriteBytes(node.ID[:]); err != nil {
		return 0, fmt.Errorf("writing ID: %w", err)
	}

	// Write node type
	if err := s.file.WriteBytes(node.Type[:]); err != nil {
		return 0, fmt.Errorf("writing type: %w", err)
	}

	// Write timestamps
	if err := s.file.WriteUint64(uint64(node.Created)); err != nil {
		return 0, fmt.Errorf("writing created: %w", err)
	}
	if err := s.file.WriteUint64(uint64(node.Modified)); err != nil {
		return 0, fmt.Errorf("writing modified: %w", err)
	}

	// Write metadata length
	if err := s.file.WriteUint32(node.MetaLen); err != nil {
		return 0, fmt.Errorf("writing meta length: %w", err)
	}

	// Write metadata
	if node.MetaLen > 0 {
		if err := s.file.WriteBytes(node.Meta); err != nil {
			return 0, fmt.Errorf("writing metadata: %w", err)
		}
	}

	return uint64(pos), nil
}

func (s *Store) readNode(entry IndexEntry) (*NodeData, error) {
	// Seek to node start
	if _, err := s.file.Seek(int64(entry.Offset), os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("seeking to node: %w", err)
	}

	// Read and verify magic
	magic, err := s.file.ReadUint32()
	if err != nil {
		return nil, fmt.Errorf("reading magic: %w", err)
	}
	if magic != common.NodeMagic {
		return nil, fmt.Errorf("invalid node magic")
	}

	// Read node data
	node := &NodeData{}

	// Read node ID
	idBytes, err := s.file.ReadBytes(32)
	if err != nil {
		return nil, fmt.Errorf("reading ID: %w", err)
	}
	copy(node.ID[:], idBytes)

	// Read node type
	typeBytes, err := s.file.ReadBytes(32)
	if err != nil {
		return nil, fmt.Errorf("reading type: %w", err)
	}
	copy(node.Type[:], typeBytes)

	// Read timestamps
	created, err := s.file.ReadUint64()
	if err != nil {
		return nil, fmt.Errorf("reading created: %w", err)
	}
	node.Created = int64(created)

	modified, err := s.file.ReadUint64()
	if err != nil {
		return nil, fmt.Errorf("reading modified: %w", err)
	}
	node.Modified = int64(modified)

	// Read metadata length
	metaLen, err := s.file.ReadUint32()
	if err != nil {
		return nil, fmt.Errorf("reading meta length: %w", err)
	}
	node.MetaLen = metaLen

	// Read metadata
	if metaLen > 0 {
		node.Meta, err = s.file.ReadBytes(int(metaLen))
		if err != nil {
			return nil, fmt.Errorf("reading metadata: %w", err)
		}
	}

	return node, nil
}
