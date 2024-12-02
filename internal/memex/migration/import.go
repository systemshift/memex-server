package migration

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"memex/internal/memex/core"
	"memex/internal/memex/storage"
)

// Importer handles graph import
type Importer struct {
	store     *storage.MXStore
	reader    *tar.Reader
	manifest  ExportManifest
	idMapping map[string]string // Old ID -> New ID
	options   ImportOptions
}

// NewImporter creates a new importer
func NewImporter(store *storage.MXStore, r io.Reader, opts ImportOptions) *Importer {
	return &Importer{
		store:     store,
		reader:    tar.NewReader(r),
		idMapping: make(map[string]string),
		options:   opts,
	}
}

// Import imports content from a tar archive
func (i *Importer) Import() error {
	fmt.Printf("Starting import into %s\n", i.store.Path())

	// First pass: read manifest and collect files
	nodes := make(map[string][]byte)  // node ID -> JSON content
	edges := make(map[string][]byte)  // edge ID -> JSON content
	chunks := make(map[string][]byte) // chunk hash -> content
	manifest := ExportManifest{}
	manifestFound := false

	for {
		header, err := i.reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		// Read file content
		fileContent := make([]byte, header.Size)
		if _, err := io.ReadFull(i.reader, fileContent); err != nil {
			return fmt.Errorf("reading content: %w", err)
		}

		// Sort files by type
		switch {
		case header.Name == "manifest.json":
			if err := json.Unmarshal(fileContent, &manifest); err != nil {
				return fmt.Errorf("parsing manifest: %w", err)
			}
			manifestFound = true

		case strings.HasPrefix(header.Name, "nodes/"):
			id := strings.TrimSuffix(filepath.Base(header.Name), ".json")
			nodes[id] = fileContent

		case strings.HasPrefix(header.Name, "edges/"):
			id := strings.TrimSuffix(filepath.Base(header.Name), ".json")
			edges[id] = fileContent

		case strings.HasPrefix(header.Name, "chunks/"):
			hash := filepath.Base(header.Name)
			chunks[hash] = fileContent
		}
	}

	if !manifestFound {
		return fmt.Errorf("manifest not found in archive")
	}

	i.manifest = manifest

	// Import chunks first
	fmt.Printf("Importing chunks...\n")
	for hash, chunkContent := range chunks {
		// Store chunk using store's StoreChunk method
		storedHash, err := i.store.StoreChunk(chunkContent)
		if err != nil {
			return fmt.Errorf("storing chunk %s: %w", hash, err)
		}
		if storedHash != hash {
			return fmt.Errorf("chunk hash mismatch: got %s, want %s", storedHash, hash)
		}
	}

	// Import nodes
	fmt.Printf("Importing nodes...\n")
	for id, nodeContent := range nodes {
		if err := i.importNode(id, nodeContent); err != nil {
			return fmt.Errorf("importing node %s: %w", id, err)
		}
	}

	// Import edges
	fmt.Printf("Importing edges...\n")
	for id, edgeContent := range edges {
		if err := i.importEdge(edgeContent); err != nil {
			return fmt.Errorf("importing edge %s: %w", id, err)
		}
	}

	return nil
}

func (i *Importer) importNode(oldID string, nodeContent []byte) error {
	var node core.Node
	if err := json.Unmarshal(nodeContent, &node); err != nil {
		return fmt.Errorf("parsing node: %w", err)
	}

	// Get content hash from metadata
	contentHash, ok := node.Meta["content"].(string)
	if !ok {
		return fmt.Errorf("node missing content hash")
	}

	// Check if node exists
	exists := false
	var existingID string
	for _, entry := range i.store.Nodes() {
		// Check by content hash to identify same file
		existingNode, err := i.store.GetNode(entry.ID)
		if err != nil {
			continue
		}
		if existingHash, ok := existingNode.Meta["content"].(string); ok {
			if existingHash == contentHash {
				exists = true
				existingID = existingNode.ID
				break
			}
		}
	}

	// Generate new ID if needed
	newID := oldID
	if exists {
		switch i.options.OnConflict {
		case Skip:
			fmt.Printf("Node %s already exists, skipping\n", oldID)
			i.idMapping[oldID] = existingID // Map to existing ID
			return nil
		case Replace:
			fmt.Printf("Node %s already exists replacing\n", oldID)
			if err := i.store.DeleteNode(existingID); err != nil {
				return fmt.Errorf("deleting existing node: %w", err)
			}
		case Rename:
			// Generate new ID by adding prefix
			newID = i.options.Prefix + oldID
			fmt.Printf("Node exists using new ID %s\n", newID)
		}
	}

	// Get chunks from metadata
	chunksRaw, ok := node.Meta["chunks"].([]interface{})
	if !ok {
		return fmt.Errorf("node missing chunks")
	}

	// Convert chunks to []string
	var chunkList []string
	for _, chunk := range chunksRaw {
		if chunkStr, ok := chunk.(string); ok {
			chunkStr = strings.Trim(chunkStr, `"`)
			chunkList = append(chunkList, chunkStr)
		}
	}

	// Load and concatenate chunks
	var buf bytes.Buffer
	for _, chunkHash := range chunkList {
		chunk, err := i.store.GetChunk(chunkHash)
		if err != nil {
			return fmt.Errorf("loading chunk %s: %w", chunkHash, err)
		}
		buf.Write(chunk)
	}

	// Add node
	fmt.Printf("Adding node %s\n", newID)
	resultID, err := i.store.AddNode(buf.Bytes(), node.Type, node.Meta)
	if err != nil {
		return fmt.Errorf("adding node: %w", err)
	}

	// Store ID mapping
	i.idMapping[oldID] = resultID
	if newID != oldID {
		i.idMapping[newID] = resultID // Also map prefixed ID
	}
	return nil
}

func (i *Importer) importEdge(edgeContent []byte) error {
	var edge ExportedLink
	if err := json.Unmarshal(edgeContent, &edge); err != nil {
		return fmt.Errorf("parsing edge: %w", err)
	}

	// Map old IDs to new IDs
	sourceID, ok := i.idMapping[edge.Source]
	if !ok {
		sourceID = edge.Source // Use original if not remapped
	}
	targetID, ok := i.idMapping[edge.Target]
	if !ok {
		targetID = edge.Target // Use original if not remapped
	}

	// Create link
	fmt.Printf("Creating link %s -> %s [%s]\n", sourceID, targetID, edge.Type)
	if err := i.store.AddLink(sourceID, targetID, edge.Type, edge.Meta); err != nil {
		return fmt.Errorf("creating link: %w", err)
	}

	return nil
}
