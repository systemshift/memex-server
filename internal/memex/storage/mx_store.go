package storage

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const (
	// Magic number to identify .mx files
	mxMagic = "MEMEX01"

	// Section identifiers
	nodeSection  = uint32(1)
	edgeSection  = uint32(2)
	blobSection  = uint32(3)
	indexSection = uint32(4)

	// Initial sizes
	headerSize = 128 // Fixed header size
)

// Header represents the .mx file header
type Header struct {
	Magic     [7]byte   // File identifier
	Version   uint8     // Format version
	Created   time.Time // Creation timestamp
	Modified  time.Time // Last modified timestamp
	NodeCount uint32    // Number of nodes
	EdgeCount uint32    // Number of edges
	BlobCount uint32    // Number of content blobs
	NodeIndex uint64    // Offset to node index
	EdgeIndex uint64    // Offset to edge index
	BlobIndex uint64    // Offset to blob index
	Reserved  [64]byte  // Reserved for future use
}

// IndexEntry represents an index entry
type IndexEntry struct {
	ID     [32]byte // Node ID, edge ID, or content hash
	Offset uint64   // File offset to data
	Length uint32   // Length of data
}

// MXStore implements a graph-oriented file format
type MXStore struct {
	path   string       // Path to .mx file
	file   *os.File     // File handle
	header Header       // File header
	nodes  []IndexEntry // Node index
	edges  []IndexEntry // Edge index
	blobs  []IndexEntry // Blob index
}

// CreateMX creates a new .mx file
func CreateMX(path string) (*MXStore, error) {
	fmt.Fprintf(os.Stderr, "Creating file: %s\n", path)

	// Ensure path ends with .mx
	if !strings.HasSuffix(path, ".mx") {
		path += ".mx"
	}

	// Create file with write permissions
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("creating file: %w", err)
	}
	fmt.Fprintf(os.Stderr, "File created successfully\n")

	// Initialize store
	store := &MXStore{
		path: path,
		file: file,
		header: Header{
			Version:  1,
			Created:  time.Now(),
			Modified: time.Now(),
		},
		nodes: make([]IndexEntry, 0),
		edges: make([]IndexEntry, 0),
		blobs: make([]IndexEntry, 0),
	}

	// Write magic number
	copy(store.header.Magic[:], mxMagic)

	// Write initial header
	if err := store.writeHeader(); err != nil {
		file.Close()
		return nil, fmt.Errorf("writing header: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Header written successfully\n")

	// Write initial indexes
	if err := store.writeIndexes(); err != nil {
		file.Close()
		return nil, fmt.Errorf("writing indexes: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Indexes written successfully\n")

	// Sync to disk
	if err := file.Sync(); err != nil {
		file.Close()
		return nil, fmt.Errorf("syncing file: %w", err)
	}

	return store, nil
}

// OpenMX opens an existing .mx file
func OpenMX(path string) (*MXStore, error) {
	fmt.Fprintf(os.Stderr, "Opening file: %s\n", path)

	// Ensure path ends with .mx
	if !strings.HasSuffix(path, ".mx") {
		path += ".mx"
	}

	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("checking file: %w", err)
	}
	fmt.Fprintf(os.Stderr, "File exists, size: %d (0x%x) bytes\n", info.Size(), info.Size())

	// Open file with read/write permissions
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	fmt.Fprintf(os.Stderr, "File opened successfully\n")

	// Initialize store
	store := &MXStore{
		path: path,
		file: file,
	}

	// Read header
	if err := store.readHeader(); err != nil {
		file.Close()
		return nil, fmt.Errorf("reading header: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Header read successfully\n")

	// Verify magic number
	if string(store.header.Magic[:]) != mxMagic {
		file.Close()
		return nil, fmt.Errorf("invalid file format")
	}
	fmt.Fprintf(os.Stderr, "Magic number verified\n")

	// Read indexes
	if err := store.readIndexes(); err != nil {
		file.Close()
		return nil, fmt.Errorf("reading indexes: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Indexes read successfully\n")

	// Dump file contents for debugging
	fmt.Fprintf(os.Stderr, "File contents:\n")
	buf := make([]byte, info.Size())
	if _, err := file.ReadAt(buf, 0); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "=== First 128 bytes ===\n")
		for i := 0; i < 128 && i < len(buf); i += 16 {
			end := i + 16
			if end > len(buf) {
				end = len(buf)
			}
			fmt.Fprintf(os.Stderr, "%04x: % x\n", i, buf[i:end])
		}
		if info.Size() > 2500 {
			fmt.Fprintf(os.Stderr, "\n=== Node Index (at 0x%x) ===\n", store.header.NodeIndex)
			start := store.header.NodeIndex
			for i := uint64(0); i < 44 && start+i < uint64(len(buf)); i += 16 {
				end := start + i + 16
				if end > uint64(len(buf)) {
					end = uint64(len(buf))
				}
				fmt.Fprintf(os.Stderr, "%04x: % x\n", start+i, buf[start+i:end])
			}
		}
	}

	return store, nil
}

// Close closes the file
func (s *MXStore) Close() error {
	fmt.Fprintf(os.Stderr, "Closing file\n")

	// Write final indexes
	if err := s.writeIndexes(); err != nil {
		return fmt.Errorf("writing indexes: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Final indexes written\n")

	// Sync to disk
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("syncing file: %w", err)
	}
	fmt.Fprintf(os.Stderr, "File synced\n")

	return s.file.Close()
}

// writeHeader writes the file header
func (s *MXStore) writeHeader() error {
	fmt.Fprintf(os.Stderr, "Writing header...\n")

	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seeking to start: %w", err)
	}

	// Create a temporary buffer to write the header
	var buf bytes.Buffer

	// Write magic number
	if _, err := buf.Write([]byte(mxMagic)); err != nil {
		return fmt.Errorf("writing magic number: %w", err)
	}

	// Write version
	if err := binary.Write(&buf, binary.LittleEndian, s.header.Version); err != nil {
		return fmt.Errorf("writing version: %w", err)
	}

	// Write timestamps as int64
	if err := binary.Write(&buf, binary.LittleEndian, s.header.Created.Unix()); err != nil {
		return fmt.Errorf("writing created time: %w", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, s.header.Modified.Unix()); err != nil {
		return fmt.Errorf("writing modified time: %w", err)
	}

	// Write counts
	if err := binary.Write(&buf, binary.LittleEndian, s.header.NodeCount); err != nil {
		return fmt.Errorf("writing node count: %w", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, s.header.EdgeCount); err != nil {
		return fmt.Errorf("writing edge count: %w", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, s.header.BlobCount); err != nil {
		return fmt.Errorf("writing blob count: %w", err)
	}

	// Write offsets
	if err := binary.Write(&buf, binary.LittleEndian, s.header.NodeIndex); err != nil {
		return fmt.Errorf("writing node index offset: %w", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, s.header.EdgeIndex); err != nil {
		return fmt.Errorf("writing edge index offset: %w", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, s.header.BlobIndex); err != nil {
		return fmt.Errorf("writing blob index offset: %w", err)
	}

	// Write reserved space
	if _, err := buf.Write(s.header.Reserved[:]); err != nil {
		return fmt.Errorf("writing reserved space: %w", err)
	}

	// Write the buffer to the file
	if _, err := s.file.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("writing header to file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Wrote header: magic=%s version=%d counts=[nodes=%d edges=%d blobs=%d] offsets=[nodes=0x%x edges=0x%x blobs=0x%x]\n",
		mxMagic, s.header.Version,
		s.header.NodeCount, s.header.EdgeCount, s.header.BlobCount,
		s.header.NodeIndex, s.header.EdgeIndex, s.header.BlobIndex)

	// Sync to disk
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("syncing file: %w", err)
	}

	return nil
}

// readHeader reads the file header
func (s *MXStore) readHeader() error {
	fmt.Fprintf(os.Stderr, "Reading header...\n")

	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seeking to start: %w", err)
	}

	// Read magic number
	magic := make([]byte, 7)
	if _, err := io.ReadFull(s.file, magic); err != nil {
		return fmt.Errorf("reading magic number: %w", err)
	}
	copy(s.header.Magic[:], magic)

	// Read version
	if err := binary.Read(s.file, binary.LittleEndian, &s.header.Version); err != nil {
		return fmt.Errorf("reading version: %w", err)
	}

	// Read timestamps as int64
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
	if err := binary.Read(s.file, binary.LittleEndian, &s.header.BlobCount); err != nil {
		return fmt.Errorf("reading blob count: %w", err)
	}

	// Read offsets
	if err := binary.Read(s.file, binary.LittleEndian, &s.header.NodeIndex); err != nil {
		return fmt.Errorf("reading node index offset: %w", err)
	}
	if err := binary.Read(s.file, binary.LittleEndian, &s.header.EdgeIndex); err != nil {
		return fmt.Errorf("reading edge index offset: %w", err)
	}
	if err := binary.Read(s.file, binary.LittleEndian, &s.header.BlobIndex); err != nil {
		return fmt.Errorf("reading blob index offset: %w", err)
	}

	// Read reserved space
	if _, err := io.ReadFull(s.file, s.header.Reserved[:]); err != nil {
		return fmt.Errorf("reading reserved space: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Read header: magic=%s version=%d counts=[nodes=%d edges=%d blobs=%d] offsets=[nodes=0x%x edges=0x%x blobs=0x%x]\n",
		magic, s.header.Version,
		s.header.NodeCount, s.header.EdgeCount, s.header.BlobCount,
		s.header.NodeIndex, s.header.EdgeIndex, s.header.BlobIndex)

	return nil
}
