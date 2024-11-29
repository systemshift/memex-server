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

const (
	edgeHeaderSize = 32 + 32 + 32 + 4 // Size of fixed fields in EdgeData
)

// AddLink adds a link between nodes
func (s *MXStore) AddLink(sourceID, targetID, linkType string, meta map[string]any) error {
	fmt.Printf("DEBUG: Adding link %s -> %s [%s]\n", sourceID[:8], targetID[:8], linkType)

	// Convert IDs to bytes
	sourceBytes, err := hex.DecodeString(sourceID)
	if err != nil {
		return fmt.Errorf("invalid source ID: %w", err)
	}
	targetBytes, err := hex.DecodeString(targetID)
	if err != nil {
		return fmt.Errorf("invalid target ID: %w", err)
	}

	// Ensure metadata is a copy to avoid modifying the input
	metaCopy := make(map[string]any)
	for k, v := range meta {
		switch k {
		case "shared":
			// Convert shared to int
			switch vt := v.(type) {
			case float64:
				metaCopy[k] = int(vt)
			case int:
				metaCopy[k] = vt
			default:
				metaCopy[k] = v
			}
		case "similarity":
			// Ensure similarity is float64
			switch vt := v.(type) {
			case float64:
				metaCopy[k] = vt
			case int:
				metaCopy[k] = float64(vt)
			default:
				metaCopy[k] = v
			}
		default:
			metaCopy[k] = v
		}
	}

	// Serialize metadata
	metaJSON, err := json.MarshalIndent(metaCopy, "", "")
	if err != nil {
		return fmt.Errorf("serializing metadata: %w", err)
	}

	if len(metaJSON) > maxMetaLen {
		return fmt.Errorf("metadata too large: %d > %d bytes", len(metaJSON), maxMetaLen)
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
	tx.isEdge = true // Mark this as an edge transaction

	// Write edge data
	offset, err := s.writeEdge(edgeData)
	if err != nil {
		tx.rollback()
		return fmt.Errorf("writing edge: %w", err)
	}

	// Calculate total length
	totalLen := edgeHeaderSize + len(metaJSON)

	// Add to index
	entry := IndexEntry{
		ID:     edgeData.Source, // Use source ID as edge ID
		Offset: offset,
		Length: uint32(totalLen),
	}
	tx.addIndex(entry)

	// Commit transaction
	if err := tx.commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	fmt.Printf("DEBUG: Added link successfully at offset %d with length %d (header=%d + meta=%d)\n",
		offset, totalLen, edgeHeaderSize, len(metaJSON))
	return nil
}

// GetLinks returns all links connected to a node
func (s *MXStore) GetLinks(nodeID string) ([]core.Link, error) {
	fmt.Printf("DEBUG: Getting links for node %s\n", nodeID[:8])
	fmt.Printf("DEBUG: Found %d edges in index\n", len(s.edges))

	// Convert ID to bytes
	idBytes, err := hex.DecodeString(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid ID: %w", err)
	}

	var links []core.Link
	for i, entry := range s.edges {
		fmt.Printf("DEBUG: Reading edge %d at offset %d with length %d\n", i, entry.Offset, entry.Length)

		// Read edge data
		if _, err := s.seek(int64(entry.Offset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("seeking to edge: %w", err)
		}

		// Create a buffer to read all data atomically
		buf := make([]byte, entry.Length)
		n, err := io.ReadFull(s.file, buf)
		if err != nil {
			return nil, fmt.Errorf("reading edge data (%d/%d bytes): %w", n, entry.Length, err)
		}

		// Create a reader for the buffer
		reader := bytes.NewReader(buf)

		var edgeData EdgeData
		// Read Source
		if _, err := io.ReadFull(reader, edgeData.Source[:]); err != nil {
			return nil, fmt.Errorf("reading edge source: %w", err)
		}
		// Read Target
		if _, err := io.ReadFull(reader, edgeData.Target[:]); err != nil {
			return nil, fmt.Errorf("reading edge target: %w", err)
		}
		// Read Type
		if _, err := io.ReadFull(reader, edgeData.Type[:]); err != nil {
			return nil, fmt.Errorf("reading edge type: %w", err)
		}
		// Read metadata length
		if err := binary.Read(reader, binary.LittleEndian, &edgeData.MetaLen); err != nil {
			return nil, fmt.Errorf("reading edge metadata length: %w", err)
		}

		fmt.Printf("DEBUG: Edge %d metadata length: %d\n", i, edgeData.MetaLen)

		// Validate metadata length
		if edgeData.MetaLen == 0 || edgeData.MetaLen > maxMetaLen {
			return nil, fmt.Errorf("invalid metadata length: %d", edgeData.MetaLen)
		}

		// Read metadata
		edgeData.Meta = make([]byte, edgeData.MetaLen)
		if _, err := io.ReadFull(reader, edgeData.Meta); err != nil {
			return nil, fmt.Errorf("reading edge metadata: %w", err)
		}

		fmt.Printf("DEBUG: Edge %d metadata: %s\n", i, string(edgeData.Meta))

		// Check if edge is connected to the node
		if bytes.Equal(edgeData.Source[:], idBytes) || bytes.Equal(edgeData.Target[:], idBytes) {
			// Get edge type
			linkType := string(bytes.TrimRight(edgeData.Type[:], "\x00"))

			// Skip self-referential edges
			if bytes.Equal(edgeData.Source[:], edgeData.Target[:]) {
				continue
			}

			// Parse metadata
			var meta map[string]any
			if err := json.Unmarshal(edgeData.Meta, &meta); err != nil {
				return nil, fmt.Errorf("parsing edge metadata: %w", err)
			}

			// Convert metadata types
			if shared, ok := meta["shared"].(float64); ok {
				meta["shared"] = int(shared)
			}

			// Create link
			link := core.Link{
				Source: hex.EncodeToString(edgeData.Source[:]),
				Target: hex.EncodeToString(edgeData.Target[:]),
				Type:   linkType,
				Meta:   meta,
			}

			// For similarity links, only include if this node is the source AND similarity metadata exists
			if linkType == "similar" {
				if bytes.Equal(edgeData.Source[:], idBytes) && meta["similarity"] != nil {
					links = append(links, link)
				}
			} else if bytes.Equal(edgeData.Source[:], idBytes) {
				// For other links, only include if this node is the source
				links = append(links, link)
			}
		}
	}

	// If we have both similarity and reference links, only return reference links
	if len(links) > 1 {
		var refLinks []core.Link
		for _, link := range links {
			if link.Type != "similar" {
				refLinks = append(refLinks, link)
			}
		}
		if len(refLinks) > 0 {
			links = refLinks
		}
	}

	fmt.Printf("DEBUG: Returning %d links\n", len(links))
	return links, nil
}

// writeEdge writes edge data to the file
func (s *MXStore) writeEdge(edge EdgeData) (uint64, error) {
	// Get current file size
	offset, err := s.seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("seeking to end: %w", err)
	}

	// Create a buffer to write all data atomically
	var buf bytes.Buffer
	buf.Grow(edgeHeaderSize + len(edge.Meta))

	// Write Source
	buf.Write(edge.Source[:])

	// Write Target
	buf.Write(edge.Target[:])

	// Write Type
	buf.Write(edge.Type[:])

	// Write metadata length
	binary.Write(&buf, binary.LittleEndian, edge.MetaLen)

	// Write metadata
	buf.Write(edge.Meta)

	// Write buffer to file
	if _, err := s.file.Write(buf.Bytes()); err != nil {
		return 0, fmt.Errorf("writing edge data: %w", err)
	}

	return uint64(offset), nil
}
