package storage

import (
	"encoding/binary"
	"fmt"
	"io"
)

// readIndexes reads the index sections
func (s *MXStore) readIndexes() error {
	fmt.Printf("DEBUG: Reading indexes from header: node=%d edge=%d blob=%d count=%d,%d,%d\n",
		s.header.NodeIndex, s.header.EdgeIndex, s.header.BlobIndex,
		s.header.NodeCount, s.header.EdgeCount, s.header.BlobCount)

	// Read node index
	if _, err := s.seek(int64(s.header.NodeIndex), io.SeekStart); err != nil {
		return fmt.Errorf("seeking to node index: %w", err)
	}

	s.nodes = make([]IndexEntry, s.header.NodeCount)
	if err := binary.Read(s.file, binary.LittleEndian, &s.nodes); err != nil {
		return fmt.Errorf("reading node index: %w", err)
	}
	fmt.Printf("DEBUG: Read %d node index entries\n", len(s.nodes))
	for i, entry := range s.nodes {
		fmt.Printf("DEBUG: Node[%d] offset=%d length=%d id=%x\n", i, entry.Offset, entry.Length, entry.ID[:8])
	}

	// Validate node index entries
	for i, entry := range s.nodes {
		if entry.Offset >= uint64(s.header.NodeIndex) {
			return fmt.Errorf("invalid node offset at index %d: %d >= %d", i, entry.Offset, s.header.NodeIndex)
		}
		if entry.Length == 0 {
			return fmt.Errorf("invalid node length at index %d: length is 0", i)
		}
	}

	// Read edge index
	if _, err := s.seek(int64(s.header.EdgeIndex), io.SeekStart); err != nil {
		return fmt.Errorf("seeking to edge index: %w", err)
	}

	s.edges = make([]IndexEntry, s.header.EdgeCount)
	if err := binary.Read(s.file, binary.LittleEndian, &s.edges); err != nil {
		return fmt.Errorf("reading edge index: %w", err)
	}
	fmt.Printf("DEBUG: Read %d edge index entries\n", len(s.edges))

	// Validate edge index entries
	for i, entry := range s.edges {
		if entry.Offset >= uint64(s.header.EdgeIndex) {
			return fmt.Errorf("invalid edge offset at index %d: %d >= %d", i, entry.Offset, s.header.EdgeIndex)
		}
		if entry.Length == 0 {
			return fmt.Errorf("invalid edge length at index %d: length is 0", i)
		}
	}

	// Read blob index
	if _, err := s.seek(int64(s.header.BlobIndex), io.SeekStart); err != nil {
		return fmt.Errorf("seeking to blob index: %w", err)
	}

	s.blobs = make([]IndexEntry, s.header.BlobCount)
	if err := binary.Read(s.file, binary.LittleEndian, &s.blobs); err != nil {
		return fmt.Errorf("reading blob index: %w", err)
	}
	fmt.Printf("DEBUG: Read %d blob index entries\n", len(s.blobs))

	// Validate blob index entries
	for i, entry := range s.blobs {
		if entry.Offset >= uint64(s.header.BlobIndex) {
			return fmt.Errorf("invalid blob offset at index %d: %d >= %d", i, entry.Offset, s.header.BlobIndex)
		}
		if entry.Length == 0 {
			return fmt.Errorf("invalid blob length at index %d: length is 0", i)
		}
	}

	return nil
}
