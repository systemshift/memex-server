package storage

import (
	"encoding/binary"
	"fmt"
	"io"
)

// readIndexes reads the node and edge indexes from the file
func (s *MXStore) readIndexes() error {
	// Read node index
	if s.header.NodeCount > 0 {
		if _, err := s.file.Seek(int64(s.header.NodeIndex), io.SeekStart); err != nil {
			return fmt.Errorf("seeking to node index: %w", err)
		}

		s.nodes = make([]IndexEntry, s.header.NodeCount)
		for i := uint32(0); i < s.header.NodeCount; i++ {
			var entry IndexEntry
			if _, err := io.ReadFull(s.file, entry.ID[:]); err != nil {
				return fmt.Errorf("reading node ID: %w", err)
			}
			if err := binary.Read(s.file, binary.LittleEndian, &entry.Offset); err != nil {
				return fmt.Errorf("reading node offset: %w", err)
			}
			if err := binary.Read(s.file, binary.LittleEndian, &entry.Length); err != nil {
				return fmt.Errorf("reading node length: %w", err)
			}
			s.nodes[i] = entry
		}
	}

	// Read edge index
	if s.header.EdgeCount > 0 {
		if _, err := s.file.Seek(int64(s.header.EdgeIndex), io.SeekStart); err != nil {
			return fmt.Errorf("seeking to edge index: %w", err)
		}

		s.edges = make([]IndexEntry, s.header.EdgeCount)
		for i := uint32(0); i < s.header.EdgeCount; i++ {
			var entry IndexEntry
			if _, err := io.ReadFull(s.file, entry.ID[:]); err != nil {
				return fmt.Errorf("reading edge ID: %w", err)
			}
			if err := binary.Read(s.file, binary.LittleEndian, &entry.Offset); err != nil {
				return fmt.Errorf("reading edge offset: %w", err)
			}
			if err := binary.Read(s.file, binary.LittleEndian, &entry.Length); err != nil {
				return fmt.Errorf("reading edge length: %w", err)
			}
			s.edges[i] = entry
		}
	}

	return nil
}
