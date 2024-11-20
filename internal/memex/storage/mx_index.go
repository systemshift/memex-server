package storage

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// writeIndexes writes the index sections
func (s *MXStore) writeIndexes() error {
	fmt.Fprintf(os.Stderr, "Writing indexes...\n")

	// Write node index
	nodeOffset, err := s.file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seeking to end: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Writing node index at offset %d (0x%x)\n", nodeOffset, nodeOffset)
	if err := binary.Write(s.file, binary.LittleEndian, s.nodes); err != nil {
		return fmt.Errorf("writing node index: %w", err)
	}
	s.header.NodeIndex = uint64(nodeOffset)
	fmt.Fprintf(os.Stderr, "Wrote %d node index entries\n", len(s.nodes))
	for i, entry := range s.nodes {
		fmt.Fprintf(os.Stderr, "Node %d: ID=%x offset=%d (0x%x) length=%d\n",
			i, entry.ID, entry.Offset, entry.Offset, entry.Length)
	}

	// Write edge index
	edgeOffset, err := s.file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seeking to end: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Writing edge index at offset %d (0x%x)\n", edgeOffset, edgeOffset)
	if err := binary.Write(s.file, binary.LittleEndian, s.edges); err != nil {
		return fmt.Errorf("writing edge index: %w", err)
	}
	s.header.EdgeIndex = uint64(edgeOffset)
	fmt.Fprintf(os.Stderr, "Wrote %d edge index entries\n", len(s.edges))
	for i, entry := range s.edges {
		fmt.Fprintf(os.Stderr, "Edge %d: ID=%x offset=%d (0x%x) length=%d\n",
			i, entry.ID, entry.Offset, entry.Offset, entry.Length)
	}

	// Write blob index
	blobOffset, err := s.file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seeking to end: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Writing blob index at offset %d (0x%x)\n", blobOffset, blobOffset)
	if err := binary.Write(s.file, binary.LittleEndian, s.blobs); err != nil {
		return fmt.Errorf("writing blob index: %w", err)
	}
	s.header.BlobIndex = uint64(blobOffset)
	fmt.Fprintf(os.Stderr, "Wrote %d blob index entries\n", len(s.blobs))
	for i, entry := range s.blobs {
		fmt.Fprintf(os.Stderr, "Blob %d: ID=%x offset=%d (0x%x) length=%d\n",
			i, entry.ID, entry.Offset, entry.Offset, entry.Length)
	}

	// Update header with new index offsets
	return s.writeHeader()
}

// readIndexes reads the index sections
func (s *MXStore) readIndexes() error {
	fmt.Fprintf(os.Stderr, "Reading indexes...\n")
	fmt.Fprintf(os.Stderr, "Node index offset: %d (0x%x)\n", s.header.NodeIndex, s.header.NodeIndex)
	fmt.Fprintf(os.Stderr, "Edge index offset: %d (0x%x)\n", s.header.EdgeIndex, s.header.EdgeIndex)
	fmt.Fprintf(os.Stderr, "Blob index offset: %d (0x%x)\n", s.header.BlobIndex, s.header.BlobIndex)
	fmt.Fprintf(os.Stderr, "Node count: %d\n", s.header.NodeCount)
	fmt.Fprintf(os.Stderr, "Edge count: %d\n", s.header.EdgeCount)
	fmt.Fprintf(os.Stderr, "Blob count: %d\n", s.header.BlobCount)

	// Read node index
	pos, err := s.file.Seek(int64(s.header.NodeIndex), io.SeekStart)
	if err != nil {
		return fmt.Errorf("seeking to node index: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Seeked to node index at %d (0x%x)\n", pos, pos)

	s.nodes = make([]IndexEntry, s.header.NodeCount)
	if err := binary.Read(s.file, binary.LittleEndian, &s.nodes); err != nil {
		return fmt.Errorf("reading node index: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Read %d node index entries\n", len(s.nodes))
	for i, entry := range s.nodes {
		fmt.Fprintf(os.Stderr, "Node %d: ID=%x offset=%d (0x%x) length=%d\n",
			i, entry.ID, entry.Offset, entry.Offset, entry.Length)
	}

	// Read edge index
	pos, err = s.file.Seek(int64(s.header.EdgeIndex), io.SeekStart)
	if err != nil {
		return fmt.Errorf("seeking to edge index: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Seeked to edge index at %d (0x%x)\n", pos, pos)

	s.edges = make([]IndexEntry, s.header.EdgeCount)
	if err := binary.Read(s.file, binary.LittleEndian, &s.edges); err != nil {
		return fmt.Errorf("reading edge index: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Read %d edge index entries\n", len(s.edges))
	for i, entry := range s.edges {
		fmt.Fprintf(os.Stderr, "Edge %d: ID=%x offset=%d (0x%x) length=%d\n",
			i, entry.ID, entry.Offset, entry.Offset, entry.Length)
	}

	// Read blob index
	pos, err = s.file.Seek(int64(s.header.BlobIndex), io.SeekStart)
	if err != nil {
		return fmt.Errorf("seeking to blob index: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Seeked to blob index at %d (0x%x)\n", pos, pos)

	s.blobs = make([]IndexEntry, s.header.BlobCount)
	if err := binary.Read(s.file, binary.LittleEndian, &s.blobs); err != nil {
		return fmt.Errorf("reading blob index: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Read %d blob index entries\n", len(s.blobs))
	for i, entry := range s.blobs {
		fmt.Fprintf(os.Stderr, "Blob %d: ID=%x offset=%d (0x%x) length=%d\n",
			i, entry.ID, entry.Offset, entry.Offset, entry.Length)
	}

	return nil
}
