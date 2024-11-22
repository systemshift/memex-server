package migration

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"memex/internal/memex/core"
	"memex/internal/memex/storage"
)

// ImportOptions configures how content is imported
type ImportOptions struct {
	OnConflict ConflictStrategy // How to handle ID conflicts
	Merge      bool             // Whether to merge with existing content
	Prefix     string           // Optional prefix for imported node IDs
}

// ConflictStrategy determines how to handle ID conflicts
type ConflictStrategy int

const (
	Skip ConflictStrategy = iota
	Replace
	Rename
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
	nodes := make(map[string][]byte) // node ID -> JSON content
	edges := make(map[string][]byte) // edge ID -> JSON content
	blobs := make(map[string][]byte) // blob hash -> content
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
		content := make([]byte, header.Size)
		if _, err := io.ReadFull(i.reader, content); err != nil {
			return fmt.Errorf("reading content: %w", err)
		}

		// Sort files by type
		switch {
		case header.Name == "manifest.json":
			if err := json.Unmarshal(content, &manifest); err != nil {
				return fmt.Errorf("parsing manifest: %w", err)
			}
			manifestFound = true

		case strings.HasPrefix(header.Name, "nodes/"):
			id := strings.TrimSuffix(filepath.Base(header.Name), ".json")
			nodes[id] = content

		case strings.HasPrefix(header.Name, "edges/"):
			id := strings.TrimSuffix(filepath.Base(header.Name), ".json")
			edges[id] = content

		case strings.HasPrefix(header.Name, "blobs/"):
			hash := filepath.Base(header.Name)
			blobs[hash] = content
		}
	}

	if !manifestFound {
		return fmt.Errorf("manifest not found in archive")
	}

	i.manifest = manifest

	// Import blobs first
	fmt.Printf("Importing blobs...\n")
	for hash, content := range blobs {
		if err := i.importBlob(hash, content); err != nil {
			return fmt.Errorf("importing blob %s: %w", hash, err)
		}
	}

	// Import nodes
	fmt.Printf("Importing nodes...\n")
	for id, content := range nodes {
		if err := i.importNode(id, content); err != nil {
			return fmt.Errorf("importing node %s: %w", id, err)
		}
	}

	// Import edges
	fmt.Printf("Importing edges...\n")
	for id, content := range edges {
		if err := i.importEdge(content); err != nil {
			return fmt.Errorf("importing edge %s: %w", id, err)
		}
	}

	return nil
}

func (i *Importer) importBlob(hash string, content []byte) error {
	// Check if blob already exists
	if i.store.HasBlob(hash) {
		fmt.Printf("Blob %s already exists, skipping\n", hash)
		return nil
	}

	// Store blob
	fmt.Printf("Storing blob %s\n", hash)
	if err := i.store.StoreBlob(content); err != nil {
		return fmt.Errorf("storing blob: %w", err)
	}

	return nil
}

func (i *Importer) importNode(oldID string, content []byte) error {
	var node core.Node
	if err := json.Unmarshal(content, &node); err != nil {
		return fmt.Errorf("parsing node: %w", err)
	}

	// Get content hash from metadata
	contentHash, ok := node.Meta["content"].(string)
	if !ok {
		return fmt.Errorf("node missing content hash")
	}

	// Load blob content first to ensure we have it
	content, err := i.store.LoadBlob(contentHash)
	if err != nil {
		return fmt.Errorf("loading blob: %w", err)
	}

	// Check if node exists
	exists := false
	var existingID string
	for _, entry := range i.store.Nodes() {
		// Check by content hash to identify same file
		existingNode, err := i.store.GetNode(fmt.Sprintf("%x", entry.ID[:]))
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
			fmt.Printf("Node %s already exists, replacing\n", oldID)
			if err := i.store.DeleteNode(existingID); err != nil {
				return fmt.Errorf("deleting existing node: %w", err)
			}
		case Rename:
			// Generate new ID by adding prefix
			newID = i.options.Prefix + oldID
			fmt.Printf("Node exists, using new ID %s\n", newID)
		}
	}

	// Add node
	fmt.Printf("Adding node %s\n", newID)
	resultID, err := i.store.AddNode(content, node.Type, node.Meta)
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

func (i *Importer) importEdge(content []byte) error {
	var edge ExportedLink
	if err := json.Unmarshal(content, &edge); err != nil {
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
