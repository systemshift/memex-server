package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"memex/internal/memex/core"
	"memex/internal/memex/logger"
)

// AddLink adds a link between nodes
func (s *MXStore) AddLink(sourceID, targetID, linkType string, meta map[string]any) error {
	logger.Log("Adding link %s -> %s [%s]", sourceID[:8], targetID[:8], linkType)

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

	// Create a buffer for the edge data
	var buf bytes.Buffer
	buf.Grow(edgeHeaderSize + len(metaJSON))

	// Write edge data
	if err := binary.Write(&buf, binary.LittleEndian, edgeData.Source); err != nil {
		tx.rollback()
		return fmt.Errorf("writing source: %w", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, edgeData.Target); err != nil {
		tx.rollback()
		return fmt.Errorf("writing target: %w", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, edgeData.Type); err != nil {
		tx.rollback()
		return fmt.Errorf("writing type: %w", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, edgeData.MetaLen); err != nil {
		tx.rollback()
		return fmt.Errorf("writing metadata length: %w", err)
	}
	if _, err := buf.Write(edgeData.Meta); err != nil {
		tx.rollback()
		return fmt.Errorf("writing metadata: %w", err)
	}

	// Write edge data to file
	offset, err := tx.write(buf.Bytes())
	if err != nil {
		tx.rollback()
		return fmt.Errorf("writing edge: %w", err)
	}

	// Add to index
	entry := IndexEntry{
		ID:     edgeData.Source, // Use source ID as edge ID
		Offset: offset,
		Length: uint32(buf.Len() + 4), // Include length prefix
	}
	tx.addIndex(entry)

	// Commit transaction
	if err := tx.commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	logger.Log("Added link successfully at offset %d with length %d (header=%d + meta=%d)",
		offset, buf.Len()+4, edgeHeaderSize, len(metaJSON))
	return nil
}

// GetLinks returns all links connected to a node
func (s *MXStore) GetLinks(nodeID string) ([]core.Link, error) {
	logger.Log("Getting links for node %s", nodeID[:8])
	logger.Log("Found %d edges in index", len(s.edges))

	// Convert ID to bytes
	idBytes, err := hex.DecodeString(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid ID: %w", err)
	}

	var links []core.Link
	for i, entry := range s.edges {
		logger.Log("Reading edge %d at offset %d with length %d", i, entry.Offset, entry.Length)

		// Read edge data
		if _, err := s.seek(int64(entry.Offset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("seeking to edge: %w", err)
		}

		// Read length prefix
		var length uint32
		if err := binary.Read(s.file, binary.LittleEndian, &length); err != nil {
			return nil, fmt.Errorf("reading length prefix: %w", err)
		}

		// Create a buffer to read all data atomically
		buf := make([]byte, length)
		if _, err := io.ReadFull(s.file, buf); err != nil {
			return nil, fmt.Errorf("reading edge data: %w", err)
		}

		// Create a reader for the buffer
		reader := bytes.NewReader(buf)

		var edgeData EdgeData
		// Read Source
		if err := binary.Read(reader, binary.LittleEndian, &edgeData.Source); err != nil {
			return nil, fmt.Errorf("reading edge source: %w", err)
		}
		// Read Target
		if err := binary.Read(reader, binary.LittleEndian, &edgeData.Target); err != nil {
			return nil, fmt.Errorf("reading edge target: %w", err)
		}
		// Read Type
		if err := binary.Read(reader, binary.LittleEndian, &edgeData.Type); err != nil {
			return nil, fmt.Errorf("reading edge type: %w", err)
		}
		// Read metadata length
		if err := binary.Read(reader, binary.LittleEndian, &edgeData.MetaLen); err != nil {
			return nil, fmt.Errorf("reading edge metadata length: %w", err)
		}

		logger.Log("Edge %d metadata length: %d", i, edgeData.MetaLen)

		// Validate metadata length
		if edgeData.MetaLen == 0 || edgeData.MetaLen > maxMetaLen {
			return nil, fmt.Errorf("invalid metadata length: %d", edgeData.MetaLen)
		}

		// Read metadata
		edgeData.Meta = make([]byte, edgeData.MetaLen)
		if _, err := io.ReadFull(reader, edgeData.Meta); err != nil {
			return nil, fmt.Errorf("reading edge metadata: %w", err)
		}

		logger.Log("Edge %d metadata: %s", i, string(edgeData.Meta))

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

	logger.Log("Returning %d links", len(links))
	return links, nil
}
