package storage

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"memex/internal/memex/core"
)

// EdgeData represents serialized edge data
type EdgeData struct {
	Source  [32]byte
	Target  [32]byte
	Type    [32]byte
	MetaLen uint32
	Meta    []byte // JSON-encoded metadata
}

// AddLink creates a link between nodes
func (s *MXStore) AddLink(source, target, linkType string, meta map[string]any) error {
	// Convert IDs to bytes
	sourceBytes, err := hex.DecodeString(source)
	if err != nil {
		return fmt.Errorf("invalid source ID: %w", err)
	}
	targetBytes, err := hex.DecodeString(target)
	if err != nil {
		return fmt.Errorf("invalid target ID: %w", err)
	}

	// Serialize metadata
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("serializing metadata: %w", err)
	}

	// Create edge data
	var edgeData EdgeData
	copy(edgeData.Source[:], sourceBytes)
	copy(edgeData.Target[:], targetBytes)
	copy(edgeData.Type[:], linkType)
	edgeData.MetaLen = uint32(len(metaJSON))
	edgeData.Meta = metaJSON

	// Write edge data
	offset, err := s.writeEdge(edgeData)
	if err != nil {
		return fmt.Errorf("writing edge: %w", err)
	}

	// Add to index
	id := sha256.Sum256(append(sourceBytes, targetBytes...))
	s.edges = append(s.edges, IndexEntry{
		ID:     id,
		Offset: offset,
		Length: uint32(binary.Size(edgeData)),
	})

	// Update header
	s.header.EdgeCount++
	s.header.Modified = time.Now()
	if err := s.writeHeader(); err != nil {
		return fmt.Errorf("updating header: %w", err)
	}

	return nil
}

// GetLinks returns all links for a node
func (s *MXStore) GetLinks(nodeID string) ([]core.Link, error) {
	// Convert ID to bytes
	idBytes, err := hex.DecodeString(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid ID: %w", err)
	}

	var links []core.Link
	for _, entry := range s.edges {
		// Read edge data
		if _, err := s.file.Seek(int64(entry.Offset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("seeking to edge: %w", err)
		}

		var edgeData EdgeData
		if err := binary.Read(s.file, binary.LittleEndian, &edgeData); err != nil {
			return nil, fmt.Errorf("reading edge: %w", err)
		}

		// Check if node is source or target
		if string(edgeData.Source[:]) != string(idBytes) &&
			string(edgeData.Target[:]) != string(idBytes) {
			continue
		}

		// Parse metadata
		var meta map[string]any
		if err := json.Unmarshal(edgeData.Meta, &meta); err != nil {
			return nil, fmt.Errorf("parsing metadata: %w", err)
		}

		// Add to result
		links = append(links, core.Link{
			Source: hex.EncodeToString(edgeData.Source[:]),
			Target: hex.EncodeToString(edgeData.Target[:]),
			Type:   string(edgeData.Type[:]),
			Meta:   meta,
		})
	}

	return links, nil
}

// writeEdge writes edge data to the file
func (s *MXStore) writeEdge(edge EdgeData) (uint64, error) {
	offset, err := s.file.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("seeking to end: %w", err)
	}

	if err := binary.Write(s.file, binary.LittleEndian, edge); err != nil {
		return 0, fmt.Errorf("writing edge: %w", err)
	}

	return uint64(offset), nil
}
