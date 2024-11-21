package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// EdgeData represents serialized edge data
type EdgeData struct {
	Source   [32]byte
	Target   [32]byte
	Type     [32]byte
	Created  int64 // Unix timestamp
	Modified int64 // Unix timestamp
	MetaLen  uint32
	Meta     []byte // JSON-encoded metadata
}

// writeEdge writes edge data to the file
func (s *MXStore) writeEdge(edge EdgeData) (uint64, error) {
	offset, err := s.file.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("seeking to end: %w", err)
	}

	// Write Source
	if _, err := s.file.Write(edge.Source[:]); err != nil {
		return 0, fmt.Errorf("writing source: %w", err)
	}

	// Write Target
	if _, err := s.file.Write(edge.Target[:]); err != nil {
		return 0, fmt.Errorf("writing target: %w", err)
	}

	// Write Type
	if _, err := s.file.Write(edge.Type[:]); err != nil {
		return 0, fmt.Errorf("writing type: %w", err)
	}

	// Write timestamps
	if err := binary.Write(s.file, binary.LittleEndian, &edge.Created); err != nil {
		return 0, fmt.Errorf("writing created time: %w", err)
	}
	if err := binary.Write(s.file, binary.LittleEndian, &edge.Modified); err != nil {
		return 0, fmt.Errorf("writing modified time: %w", err)
	}

	// Write metadata length
	if err := binary.Write(s.file, binary.LittleEndian, &edge.MetaLen); err != nil {
		return 0, fmt.Errorf("writing metadata length: %w", err)
	}

	// Write metadata
	if _, err := s.file.Write(edge.Meta); err != nil {
		return 0, fmt.Errorf("writing metadata: %w", err)
	}

	return uint64(offset), nil
}

// AddLink creates a link between nodes
func (s *MXStore) AddLink(source, target, linkType string, meta map[string]any) error {
	// Convert source ID to bytes
	sourceBytes, err := hex.DecodeString(source)
	if err != nil {
		return fmt.Errorf("invalid source ID: %w", err)
	}
	var sourceID [32]byte
	copy(sourceID[:], sourceBytes)

	// Convert target ID to bytes
	targetBytes, err := hex.DecodeString(target)
	if err != nil {
		return fmt.Errorf("invalid target ID: %w", err)
	}
	var targetID [32]byte
	copy(targetID[:], targetBytes)

	// Verify nodes exist
	sourceFound := false
	targetFound := false
	for _, node := range s.nodes {
		if bytes.HasPrefix(node.ID[:], sourceBytes) {
			sourceFound = true
			copy(sourceID[:], node.ID[:])
		}
		if bytes.HasPrefix(node.ID[:], targetBytes) {
			targetFound = true
			copy(targetID[:], node.ID[:])
		}
	}
	if !sourceFound {
		return fmt.Errorf("source node not found")
	}
	if !targetFound {
		return fmt.Errorf("target node not found")
	}

	// Serialize metadata
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("serializing metadata: %w", err)
	}

	// Create edge data
	now := time.Now().Unix()
	var edgeData EdgeData
	copy(edgeData.Source[:], sourceID[:])
	copy(edgeData.Target[:], targetID[:])
	copy(edgeData.Type[:], []byte(linkType))
	edgeData.Created = now
	edgeData.Modified = now
	edgeData.MetaLen = uint32(len(metaJSON))
	edgeData.Meta = metaJSON

	// Write edge data
	offset, err := s.writeEdge(edgeData)
	if err != nil {
		return fmt.Errorf("writing edge: %w", err)
	}

	// Add to index
	s.edges = append(s.edges, IndexEntry{
		ID:     sourceID, // Use source ID as edge ID
		Offset: offset,
		Length: uint32(104 + len(metaJSON)), // Fixed size + metadata
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
func (s *MXStore) GetLinks(nodeID string) ([]Link, error) {
	// Convert node ID to bytes
	idBytes, err := hex.DecodeString(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid ID: %w", err)
	}

	// Find full node ID
	var fullID []byte
	for _, node := range s.nodes {
		if bytes.HasPrefix(node.ID[:], idBytes) {
			fullID = node.ID[:]
			break
		}
	}
	if fullID == nil {
		return nil, fmt.Errorf("node not found")
	}

	var links []Link
	for _, e := range s.edges {
		// Seek to edge data
		if _, err := s.file.Seek(int64(e.Offset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("seeking to edge: %w", err)
		}

		// Read edge data
		var edgeData EdgeData

		// Read Source
		if _, err := io.ReadFull(s.file, edgeData.Source[:]); err != nil {
			return nil, fmt.Errorf("reading source: %w", err)
		}

		// Read Target
		if _, err := io.ReadFull(s.file, edgeData.Target[:]); err != nil {
			return nil, fmt.Errorf("reading target: %w", err)
		}

		// Read Type
		if _, err := io.ReadFull(s.file, edgeData.Type[:]); err != nil {
			return nil, fmt.Errorf("reading type: %w", err)
		}

		// Read timestamps
		if err := binary.Read(s.file, binary.LittleEndian, &edgeData.Created); err != nil {
			return nil, fmt.Errorf("reading created time: %w", err)
		}
		if err := binary.Read(s.file, binary.LittleEndian, &edgeData.Modified); err != nil {
			return nil, fmt.Errorf("reading modified time: %w", err)
		}

		// Read metadata length
		if err := binary.Read(s.file, binary.LittleEndian, &edgeData.MetaLen); err != nil {
			return nil, fmt.Errorf("reading metadata length: %w", err)
		}

		// Read metadata
		edgeData.Meta = make([]byte, edgeData.MetaLen)
		if _, err := io.ReadFull(s.file, edgeData.Meta); err != nil {
			return nil, fmt.Errorf("reading metadata: %w", err)
		}

		// Parse metadata
		var meta map[string]any
		if err := json.Unmarshal(edgeData.Meta, &meta); err != nil {
			return nil, fmt.Errorf("parsing metadata: %w", err)
		}

		// Check if this edge is connected to the requested node
		if bytes.Equal(edgeData.Source[:], fullID) {
			links = append(links, Link{
				Target: hex.EncodeToString(bytes.TrimRight(edgeData.Target[:], "\x00")),
				Type:   string(bytes.TrimRight(edgeData.Type[:], "\x00")),
				Meta:   meta,
			})
		}
	}

	return links, nil
}

// Link represents a link between nodes
type Link struct {
	Target string         // Target node ID
	Type   string         // Link type
	Meta   map[string]any // Link metadata
}
