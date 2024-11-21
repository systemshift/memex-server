package storage

import (
	"encoding/binary"
	"fmt"
	"io"
)

// writeIndexes writes the index sections
func (s *MXStore) writeIndexes() error {
	// Write node index
	nodeOffset, err := s.file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seeking to end: %w", err)
	}
	if err := binary.Write(s.file, binary.LittleEndian, s.nodes); err != nil {
		return fmt.Errorf("writing node index: %w", err)
	}
	s.header.NodeIndex = uint64(nodeOffset)

	// Write edge index
	edgeOffset, err := s.file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seeking to end: %w", err)
	}
	if err := binary.Write(s.file, binary.LittleEndian, s.edges); err != nil {
		return fmt.Errorf("writing edge index: %w", err)
	}
	s.header.EdgeIndex = uint64(edgeOffset)

	// Write blob index
	blobOffset, err := s.file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seeking to end: %w", err)
	}
	if err := binary.Write(s.file, binary.LittleEndian, s.blobs); err != nil {
		return fmt.Errorf("writing blob index: %w", err)
	}
	s.header.BlobIndex = uint64(blobOffset)

	// Update header with new index offsets
	return s.writeHeader()
}

// readIndexes reads the index sections
func (s *MXStore) readIndexes() error {
	// Read node index
	if _, err := s.file.Seek(int64(s.header.NodeIndex), io.SeekStart); err != nil {
		return fmt.Errorf("seeking to node index: %w", err)
	}

	s.nodes = make([]IndexEntry, s.header.NodeCount)
	if err := binary.Read(s.file, binary.LittleEndian, &s.nodes); err != nil {
		return fmt.Errorf("reading node index: %w", err)
	}

	// Read edge index
	if _, err := s.file.Seek(int64(s.header.EdgeIndex), io.SeekStart); err != nil {
		return fmt.Errorf("seeking to edge index: %w", err)
	}

	s.edges = make([]IndexEntry, s.header.EdgeCount)
	if err := binary.Read(s.file, binary.LittleEndian, &s.edges); err != nil {
		return fmt.Errorf("reading edge index: %w", err)
	}

	// Read blob index
	if _, err := s.file.Seek(int64(s.header.BlobIndex), io.SeekStart); err != nil {
		return fmt.Errorf("seeking to blob index: %w", err)
	}

	s.blobs = make([]IndexEntry, s.header.BlobCount)
	if err := binary.Read(s.file, binary.LittleEndian, &s.blobs); err != nil {
		return fmt.Errorf("reading blob index: %w", err)
	}

	return nil
}
