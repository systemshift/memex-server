package link

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"memex/internal/memex/core"
	"memex/internal/memex/storage/common"
)

// Store manages links between nodes
type Store struct {
	file  *common.File
	index *Index
	locks *common.LockManager
}

// NewStore creates a new link store
func NewStore(file *common.File, locks *common.LockManager) *Store {
	return &Store{
		file:  file,
		index: NewIndex(),
		locks: locks,
	}
}

// Add creates a link between two nodes
func (s *Store) Add(sourceID, targetID, linkType string, meta map[string]any) error {
	return s.locks.WithChunkLock(func() error {
		// Parse node IDs
		sourceBytes, err := decodeID(sourceID)
		if err != nil {
			return fmt.Errorf("invalid source ID: %w", err)
		}

		targetBytes, err := decodeID(targetID)
		if err != nil {
			return fmt.Errorf("invalid target ID: %w", err)
		}

		// Create link data
		now := time.Now().Unix()
		link := LinkData{
			Source:   sourceBytes,
			Target:   targetBytes,
			Type:     stringToFixedType(linkType),
			Created:  now,
			Modified: now,
		}

		// Marshal metadata
		if meta != nil {
			metaBytes, err := json.Marshal(meta)
			if err != nil {
				return fmt.Errorf("marshaling metadata: %w", err)
			}

			if len(metaBytes) > MaxMetaSize {
				return fmt.Errorf("metadata too large (max %d bytes)", MaxMetaSize)
			}

			link.MetaLen = uint32(len(metaBytes))
			link.Meta = metaBytes
		}

		// Calculate link ID
		idBytes := sha256.Sum256(append(append(sourceBytes[:], targetBytes[:]...), []byte(linkType)...))
		link.ID = idBytes

		// Write link data
		offset, err := s.writeLink(link)
		if err != nil {
			return fmt.Errorf("writing link: %w", err)
		}

		// Create index entry
		entry := IndexEntry{
			ID:     idBytes,
			Offset: offset,
			Length: uint32(link.Size()),
			Flags:  FlagNone,
		}

		// Add to index
		s.index.Add(entry)

		return nil
	})
}

// Get retrieves links for a node
func (s *Store) Get(nodeID string) ([]*core.Link, error) {
	return s.locks.WithChunkLockLinks(func() ([]*core.Link, error) {
		// Parse node ID
		nodeBytes, err := decodeID(nodeID)
		if err != nil {
			return nil, fmt.Errorf("invalid node ID: %w", err)
		}

		var links []*core.Link

		// Find all links involving this node
		for _, entry := range s.index.Entries() {
			// Read link data
			linkData, err := s.readLink(entry)
			if err != nil {
				continue
			}

			// Check if this link involves our node
			if linkData.Source == nodeBytes || linkData.Target == nodeBytes {
				// Parse metadata
				var meta map[string]any
				if linkData.MetaLen > 0 {
					if err := json.Unmarshal(linkData.Meta, &meta); err != nil {
						continue
					}
				}

				// Create link
				link := linkData.ToCore()
				link.Meta = meta

				links = append(links, link)
			}
		}

		return links, nil
	})
}

// Delete removes a link between two nodes
func (s *Store) Delete(sourceID, targetID, linkType string) error {
	return s.locks.WithChunkLock(func() error {
		// Parse node IDs
		sourceBytes, err := decodeID(sourceID)
		if err != nil {
			return fmt.Errorf("invalid source ID: %w", err)
		}

		targetBytes, err := decodeID(targetID)
		if err != nil {
			return fmt.Errorf("invalid target ID: %w", err)
		}

		// Calculate link ID
		idBytes := sha256.Sum256(append(append(sourceBytes[:], targetBytes[:]...), []byte(linkType)...))

		// Remove from index
		if !s.index.Remove(idBytes) {
			return fmt.Errorf("link not found: %s -[%s]-> %s", sourceID, linkType, targetID)
		}

		return nil
	})
}

// LoadIndex loads the link index from the file
func (s *Store) LoadIndex(offset uint64, count uint32) error {
	return s.locks.WithChunkLock(func() error {
		return s.index.Load(s.file, offset, count)
	})
}

// SaveIndex saves the link index to the file
func (s *Store) SaveIndex(offset uint64) error {
	return s.locks.WithChunkLock(func() error {
		return s.index.Save(s.file, offset)
	})
}

// Internal methods

func (s *Store) writeLink(link LinkData) (uint64, error) {
	// Get current position
	pos, err := s.file.Seek(0, os.SEEK_END)
	if err != nil {
		return 0, fmt.Errorf("seeking to end: %w", err)
	}

	// Write link magic
	if err := s.file.WriteUint32(common.LinkMagic); err != nil {
		return 0, fmt.Errorf("writing magic: %w", err)
	}

	// Write link ID
	if err := s.file.WriteBytes(link.ID[:]); err != nil {
		return 0, fmt.Errorf("writing ID: %w", err)
	}

	// Write source and target
	if err := s.file.WriteBytes(link.Source[:]); err != nil {
		return 0, fmt.Errorf("writing source: %w", err)
	}
	if err := s.file.WriteBytes(link.Target[:]); err != nil {
		return 0, fmt.Errorf("writing target: %w", err)
	}

	// Write link type
	if err := s.file.WriteBytes(link.Type[:]); err != nil {
		return 0, fmt.Errorf("writing type: %w", err)
	}

	// Write timestamps
	if err := s.file.WriteUint64(uint64(link.Created)); err != nil {
		return 0, fmt.Errorf("writing created: %w", err)
	}
	if err := s.file.WriteUint64(uint64(link.Modified)); err != nil {
		return 0, fmt.Errorf("writing modified: %w", err)
	}

	// Write metadata length
	if err := s.file.WriteUint32(link.MetaLen); err != nil {
		return 0, fmt.Errorf("writing meta length: %w", err)
	}

	// Write metadata
	if link.MetaLen > 0 {
		if err := s.file.WriteBytes(link.Meta); err != nil {
			return 0, fmt.Errorf("writing metadata: %w", err)
		}
	}

	return uint64(pos), nil
}

func (s *Store) readLink(entry IndexEntry) (*LinkData, error) {
	// Seek to link start
	if _, err := s.file.Seek(int64(entry.Offset), os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("seeking to link: %w", err)
	}

	// Read and verify magic
	magic, err := s.file.ReadUint32()
	if err != nil {
		return nil, fmt.Errorf("reading magic: %w", err)
	}
	if magic != common.LinkMagic {
		return nil, fmt.Errorf("invalid link magic")
	}

	// Read link data
	link := &LinkData{}

	// Read link ID
	idBytes, err := s.file.ReadBytes(32)
	if err != nil {
		return nil, fmt.Errorf("reading ID: %w", err)
	}
	copy(link.ID[:], idBytes)

	// Read source and target
	sourceBytes, err := s.file.ReadBytes(32)
	if err != nil {
		return nil, fmt.Errorf("reading source: %w", err)
	}
	copy(link.Source[:], sourceBytes)

	targetBytes, err := s.file.ReadBytes(32)
	if err != nil {
		return nil, fmt.Errorf("reading target: %w", err)
	}
	copy(link.Target[:], targetBytes)

	// Read link type
	typeBytes, err := s.file.ReadBytes(32)
	if err != nil {
		return nil, fmt.Errorf("reading type: %w", err)
	}
	copy(link.Type[:], typeBytes)

	// Read timestamps
	created, err := s.file.ReadUint64()
	if err != nil {
		return nil, fmt.Errorf("reading created: %w", err)
	}
	link.Created = int64(created)

	modified, err := s.file.ReadUint64()
	if err != nil {
		return nil, fmt.Errorf("reading modified: %w", err)
	}
	link.Modified = int64(modified)

	// Read metadata length
	metaLen, err := s.file.ReadUint32()
	if err != nil {
		return nil, fmt.Errorf("reading meta length: %w", err)
	}
	link.MetaLen = metaLen

	// Read metadata
	if metaLen > 0 {
		link.Meta, err = s.file.ReadBytes(int(metaLen))
		if err != nil {
			return nil, fmt.Errorf("reading metadata: %w", err)
		}
	}

	return link, nil
}
