package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"memex/internal/memex/core"
)

// EdgeData represents serialized edge data
type EdgeData struct {
	Source  [32]byte // Source node ID
	Target  [32]byte // Target node ID
	Type    [32]byte // Edge type
	MetaLen uint32   // Length of metadata
	Meta    []byte   // JSON-encoded metadata
}

// AddLink adds a link between nodes
func (s *MXStore) AddLink(sourceID, targetID, linkType string, meta map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Convert IDs to bytes
	sourceBytes, err := hex.DecodeString(sourceID)
	if err != nil {
		return fmt.Errorf("invalid source ID: %w", err)
	}
	targetBytes, err := hex.DecodeString(targetID)
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
	copy(edgeData.Type[:], []byte(linkType))
	edgeData.MetaLen = uint32(len(metaJSON))
	edgeData.Meta = metaJSON

	// Begin transaction
	tx, err := s.beginTransaction()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	// Write edge data
	offset, err := s.writeEdge(edgeData)
	if err != nil {
		tx.rollback()
		return fmt.Errorf("writing edge: %w", err)
	}

	// Add to index
	entry := IndexEntry{
		ID:     edgeData.Source, // Use source ID as edge ID
		Offset: offset,
		Length: uint32(72 + len(metaJSON)), // Fixed size + metadata
	}
	tx.addIndex(entry)

	// Update header
	s.header.EdgeCount++
	s.header.Modified = s.header.Created

	// Commit transaction
	if err := tx.commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// GetLinks returns all links connected to a node
func (s *MXStore) GetLinks(nodeID string) ([]core.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Convert ID to bytes
	idBytes, err := hex.DecodeString(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid ID: %w", err)
	}

	var links []core.Link
	for _, entry := range s.edges {
		// Read edge data
		if _, err := s.seek(int64(entry.Offset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("seeking to edge: %w", err)
		}

		var edgeData EdgeData
		// Read Source
		if _, err := io.ReadFull(s.file, edgeData.Source[:]); err != nil {
			return nil, fmt.Errorf("reading edge source: %w", err)
		}
		// Read Target
		if _, err := io.ReadFull(s.file, edgeData.Target[:]); err != nil {
			return nil, fmt.Errorf("reading edge target: %w", err)
		}
		// Read Type
		if _, err := io.ReadFull(s.file, edgeData.Type[:]); err != nil {
			return nil, fmt.Errorf("reading edge type: %w", err)
		}
		// Read metadata length
		if err := binary.Read(s.file, binary.LittleEndian, &edgeData.MetaLen); err != nil {
			return nil, fmt.Errorf("reading edge metadata length: %w", err)
		}
		// Read metadata
		edgeData.Meta = make([]byte, edgeData.MetaLen)
		if _, err := io.ReadFull(s.file, edgeData.Meta); err != nil {
			return nil, fmt.Errorf("reading edge metadata: %w", err)
		}

		// Check if edge is connected to the node
		if bytes.Equal(edgeData.Source[:], idBytes) || bytes.Equal(edgeData.Target[:], idBytes) {
			// Parse metadata
			var meta map[string]any
			if err := json.Unmarshal(edgeData.Meta, &meta); err != nil {
				return nil, fmt.Errorf("parsing edge metadata: %w", err)
			}

			// Create link
			link := core.Link{
				Source: hex.EncodeToString(edgeData.Source[:]),
				Target: hex.EncodeToString(edgeData.Target[:]),
				Type:   string(bytes.TrimRight(edgeData.Type[:], "\x00")),
				Meta:   meta,
			}
			links = append(links, link)
		}
	}

	return links, nil
}

// writeEdge writes edge data to the file
func (s *MXStore) writeEdge(edge EdgeData) (uint64, error) {
	// Get current file size
	offset, err := s.seek(0, io.SeekEnd)
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

	// Write metadata length
	if err := binary.Write(s.file, binary.LittleEndian, edge.MetaLen); err != nil {
		return 0, fmt.Errorf("writing metadata length: %w", err)
	}

	// Write metadata
	if _, err := s.file.Write(edge.Meta); err != nil {
		return 0, fmt.Errorf("writing metadata: %w", err)
	}

	return uint64(offset), nil
}
