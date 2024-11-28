package storage

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

// Header represents the file header
type Header struct {
	Version   uint32    // File format version
	Created   time.Time // Creation timestamp
	Modified  time.Time // Last modified timestamp
	NodeCount uint32    // Number of nodes
	EdgeCount uint32    // Number of edges
	NodeIndex uint64    // Offset to node index
	EdgeIndex uint64    // Offset to edge index
}

// readHeader reads the header from the file
func (s *MXStore) readHeader() error {
	// Seek to start of file
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seeking to start: %w", err)
	}

	// Read version
	if err := binary.Read(s.file, binary.LittleEndian, &s.header.Version); err != nil {
		return fmt.Errorf("reading version: %w", err)
	}

	// Verify version
	if s.header.Version != Version {
		return fmt.Errorf("unsupported version: %d", s.header.Version)
	}

	// Read timestamps
	var created, modified int64
	if err := binary.Read(s.file, binary.LittleEndian, &created); err != nil {
		return fmt.Errorf("reading created time: %w", err)
	}
	if err := binary.Read(s.file, binary.LittleEndian, &modified); err != nil {
		return fmt.Errorf("reading modified time: %w", err)
	}
	s.header.Created = time.Unix(created, 0)
	s.header.Modified = time.Unix(modified, 0)

	// Read counts
	if err := binary.Read(s.file, binary.LittleEndian, &s.header.NodeCount); err != nil {
		return fmt.Errorf("reading node count: %w", err)
	}
	if err := binary.Read(s.file, binary.LittleEndian, &s.header.EdgeCount); err != nil {
		return fmt.Errorf("reading edge count: %w", err)
	}

	// Read index offsets
	if err := binary.Read(s.file, binary.LittleEndian, &s.header.NodeIndex); err != nil {
		return fmt.Errorf("reading node index offset: %w", err)
	}
	if err := binary.Read(s.file, binary.LittleEndian, &s.header.EdgeIndex); err != nil {
		return fmt.Errorf("reading edge index offset: %w", err)
	}

	return nil
}

// writeHeader writes the header to the file
func (s *MXStore) writeHeader() error {
	// Seek to start of file
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seeking to start: %w", err)
	}

	// Write version
	if err := binary.Write(s.file, binary.LittleEndian, s.header.Version); err != nil {
		return fmt.Errorf("writing version: %w", err)
	}

	// Write timestamps
	if err := binary.Write(s.file, binary.LittleEndian, s.header.Created.Unix()); err != nil {
		return fmt.Errorf("writing created time: %w", err)
	}
	if err := binary.Write(s.file, binary.LittleEndian, s.header.Modified.Unix()); err != nil {
		return fmt.Errorf("writing modified time: %w", err)
	}

	// Write counts
	if err := binary.Write(s.file, binary.LittleEndian, s.header.NodeCount); err != nil {
		return fmt.Errorf("writing node count: %w", err)
	}
	if err := binary.Write(s.file, binary.LittleEndian, s.header.EdgeCount); err != nil {
		return fmt.Errorf("writing edge count: %w", err)
	}

	// Write index offsets
	if err := binary.Write(s.file, binary.LittleEndian, s.header.NodeIndex); err != nil {
		return fmt.Errorf("writing node index offset: %w", err)
	}
	if err := binary.Write(s.file, binary.LittleEndian, s.header.EdgeIndex); err != nil {
		return fmt.Errorf("writing edge index offset: %w", err)
	}

	return nil
}
