package storage

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// writeIndexes writes the indexes to the file
func (s *MXStore) writeIndexes() error {
	// Create a buffer for all indexes
	var buf bytes.Buffer

	// Write node index
	for _, entry := range s.nodes {
		buf.Write(entry.ID[:])
		binary.Write(&buf, binary.LittleEndian, entry.Offset)
		binary.Write(&buf, binary.LittleEndian, entry.Length)
	}

	// Write edge index
	for _, entry := range s.edges {
		buf.Write(entry.ID[:])
		binary.Write(&buf, binary.LittleEndian, entry.Offset)
		binary.Write(&buf, binary.LittleEndian, entry.Length)
	}

	// Write blob index
	for _, entry := range s.blobs {
		buf.Write(entry.ID[:])
		binary.Write(&buf, binary.LittleEndian, entry.Offset)
		binary.Write(&buf, binary.LittleEndian, entry.Length)
	}

	// Write buffer to file
	if _, err := s.file.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("writing indexes: %w", err)
	}

	return nil
}
