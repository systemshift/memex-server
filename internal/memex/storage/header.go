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
	Created   time.Time // Repository creation time
	Modified  time.Time // Last modification time
	NodeCount uint32    // Number of nodes
	EdgeCount uint32    // Number of edges
	BlobCount uint32    // Number of blobs
	NodeIndex uint64    // Offset to node index
	EdgeIndex uint64    // Offset to edge index
	BlobIndex uint64    // Offset to blob index
}

// writeHeader writes the header to the file
func (s *MXStore) writeHeader() error {
	fmt.Printf("DEBUG: Writing header: version=%d nodes=%d edges=%d blobs=%d\n",
		s.header.Version, s.header.NodeCount, s.header.EdgeCount, s.header.BlobCount)
	fmt.Printf("DEBUG: Writing header indexes: node=%d edge=%d blob=%d\n",
		s.header.NodeIndex, s.header.EdgeIndex, s.header.BlobIndex)

	// Create a buffer to write all data atomically
	var buf [128]byte // Fixed size header
	binary.LittleEndian.PutUint32(buf[0:], s.header.Version)
	binary.LittleEndian.PutUint64(buf[4:], uint64(s.header.Created.Unix()))
	binary.LittleEndian.PutUint64(buf[12:], uint64(s.header.Modified.Unix()))
	binary.LittleEndian.PutUint32(buf[20:], s.header.NodeCount)
	binary.LittleEndian.PutUint32(buf[24:], s.header.EdgeCount)
	binary.LittleEndian.PutUint32(buf[28:], s.header.BlobCount)
	binary.LittleEndian.PutUint64(buf[32:], s.header.NodeIndex)
	binary.LittleEndian.PutUint64(buf[40:], s.header.EdgeIndex)
	binary.LittleEndian.PutUint64(buf[48:], s.header.BlobIndex)

	// Write buffer to file
	if _, err := s.file.Write(buf[:]); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	return nil
}

// readHeader reads the header from the file
func (s *MXStore) readHeader() error {
	// Seek to start of file
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seeking to start: %w", err)
	}

	// Read header data
	var buf [128]byte // Fixed size header
	if _, err := io.ReadFull(s.file, buf[:]); err != nil {
		return fmt.Errorf("reading header: %w", err)
	}

	// Parse header fields
	s.header.Version = binary.LittleEndian.Uint32(buf[0:])
	s.header.Created = time.Unix(int64(binary.LittleEndian.Uint64(buf[4:])), 0)
	s.header.Modified = time.Unix(int64(binary.LittleEndian.Uint64(buf[12:])), 0)
	s.header.NodeCount = binary.LittleEndian.Uint32(buf[20:])
	s.header.EdgeCount = binary.LittleEndian.Uint32(buf[24:])
	s.header.BlobCount = binary.LittleEndian.Uint32(buf[28:])
	s.header.NodeIndex = binary.LittleEndian.Uint64(buf[32:])
	s.header.EdgeIndex = binary.LittleEndian.Uint64(buf[40:])
	s.header.BlobIndex = binary.LittleEndian.Uint64(buf[48:])

	fmt.Printf("DEBUG: Read header: version=%d nodes=%d edges=%d blobs=%d\n",
		s.header.Version, s.header.NodeCount, s.header.EdgeCount, s.header.BlobCount)
	fmt.Printf("DEBUG: Read header indexes: node=%d edge=%d blob=%d\n",
		s.header.NodeIndex, s.header.EdgeIndex, s.header.BlobIndex)

	return nil
}

// readIndexes reads the indexes from the file
func (s *MXStore) readIndexes() error {
	fmt.Printf("DEBUG: Reading indexes from header: node=%d edge=%d blob=%d count=%d\n",
		s.header.NodeIndex, s.header.EdgeIndex, s.header.BlobIndex, s.header.NodeCount*40)

	// Read node index
	if s.header.NodeCount > 0 {
		if _, err := s.file.Seek(int64(s.header.NodeIndex), io.SeekStart); err != nil {
			return fmt.Errorf("seeking to node index: %w", err)
		}

		s.nodes = make([]IndexEntry, s.header.NodeCount)
		for i := range s.nodes {
			var buf [44]byte // 32 bytes ID + 8 bytes offset + 4 bytes length
			if _, err := io.ReadFull(s.file, buf[:]); err != nil {
				return fmt.Errorf("reading node entry: %w", err)
			}
			copy(s.nodes[i].ID[:], buf[:32])
			s.nodes[i].Offset = binary.LittleEndian.Uint64(buf[32:40])
			s.nodes[i].Length = binary.LittleEndian.Uint32(buf[40:44])
		}

		fmt.Printf("DEBUG: Read %d node index entries\n", len(s.nodes))
		for i, entry := range s.nodes {
			fmt.Printf("DEBUG: Node[%d] offset=%d length=%d id=%x\n",
				i, entry.Offset, entry.Length, entry.ID[:8])
		}
	}

	// Read edge index
	if s.header.EdgeCount > 0 {
		if _, err := s.file.Seek(int64(s.header.EdgeIndex), io.SeekStart); err != nil {
			return fmt.Errorf("seeking to edge index: %w", err)
		}

		s.edges = make([]IndexEntry, s.header.EdgeCount)
		for i := range s.edges {
			var buf [44]byte // 32 bytes ID + 8 bytes offset + 4 bytes length
			if _, err := io.ReadFull(s.file, buf[:]); err != nil {
				return fmt.Errorf("reading edge entry: %w", err)
			}
			copy(s.edges[i].ID[:], buf[:32])
			s.edges[i].Offset = binary.LittleEndian.Uint64(buf[32:40])
			s.edges[i].Length = binary.LittleEndian.Uint32(buf[40:44])
		}

		fmt.Printf("DEBUG: Read %d edge index entries\n", len(s.edges))
	}

	// Read blob index
	if s.header.BlobCount > 0 {
		if _, err := s.file.Seek(int64(s.header.BlobIndex), io.SeekStart); err != nil {
			return fmt.Errorf("seeking to blob index: %w", err)
		}

		s.blobs = make([]IndexEntry, s.header.BlobCount)
		for i := range s.blobs {
			var buf [44]byte // 32 bytes ID + 8 bytes offset + 4 bytes length
			if _, err := io.ReadFull(s.file, buf[:]); err != nil {
				return fmt.Errorf("reading blob entry: %w", err)
			}
			copy(s.blobs[i].ID[:], buf[:32])
			s.blobs[i].Offset = binary.LittleEndian.Uint64(buf[32:40])
			s.blobs[i].Length = binary.LittleEndian.Uint32(buf[40:44])
		}
	}

	return nil
}
