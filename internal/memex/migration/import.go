package migration

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"memex/internal/memex/core"
)

// Importer handles repository imports
type Importer struct {
	repo    core.Repository
	reader  io.Reader
	options ImportOptions
}

// NewImporter creates a new importer
func NewImporter(repo core.Repository, r io.Reader, opts ImportOptions) *Importer {
	return &Importer{
		repo:    repo,
		reader:  r,
		options: opts,
	}
}

// Import imports content from a tar archive
func (i *Importer) Import() error {
	fmt.Println("Starting import")

	// Create tar reader
	tr := tar.NewReader(i.reader)

	// Read manifest first
	manifest, err := i.readManifest(tr)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	fmt.Printf("Importing version %d content\n", manifest.Version)

	// Import chunks
	fmt.Println("Importing chunks...")
	chunks := make(map[string]bool)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		// Skip non-chunk files
		if !strings.HasPrefix(header.Name, "chunks/") {
			continue
		}

		// Get chunk hash from filename
		hash := filepath.Base(header.Name)

		// Read chunk data
		data := make([]byte, header.Size)
		if _, err := io.ReadFull(tr, data); err != nil {
			return fmt.Errorf("reading chunk data: %w", err)
		}

		// Store chunk
		chunks[hash] = true
	}

	// Import nodes
	fmt.Println("Importing nodes...")
	nodes := make(map[string]*core.Node)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		// Skip non-node files
		if !strings.HasPrefix(header.Name, "nodes/") {
			continue
		}

		// Read node data
		data := make([]byte, header.Size)
		if _, err := io.ReadFull(tr, data); err != nil {
			return fmt.Errorf("reading node data: %w", err)
		}

		// Parse node
		var node core.Node
		if err := json.Unmarshal(data, &node); err != nil {
			return fmt.Errorf("parsing node: %w", err)
		}

		// Add prefix to ID if specified
		if i.options.Prefix != "" {
			node.ID = i.options.Prefix + node.ID
		}

		// Store node
		if _, err := i.repo.AddNode(node.Content, node.Type, node.Meta); err != nil {
			return fmt.Errorf("storing node: %w", err)
		}

		nodes[node.ID] = &node
	}

	// Import edges
	fmt.Println("Importing edges...")
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		// Skip non-edge files
		if !strings.HasPrefix(header.Name, "edges/") {
			continue
		}

		// Read edge data
		data := make([]byte, header.Size)
		if _, err := io.ReadFull(tr, data); err != nil {
			return fmt.Errorf("reading edge data: %w", err)
		}

		// Parse link
		var link core.Link
		if err := json.Unmarshal(data, &link); err != nil {
			return fmt.Errorf("parsing link: %w", err)
		}

		// Add prefix to IDs if specified
		if i.options.Prefix != "" {
			link.Source = i.options.Prefix + link.Source
			link.Target = i.options.Prefix + link.Target
		}

		// Store link
		if err := i.repo.AddLink(link.Source, link.Target, link.Type, link.Meta); err != nil {
			return fmt.Errorf("storing link: %w", err)
		}
	}

	return nil
}

func (i *Importer) readManifest(tr *tar.Reader) (*ExportManifest, error) {
	// Find manifest file
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("manifest not found")
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar: %w", err)
		}

		if header.Name == "manifest.json" {
			// Read manifest data
			data := make([]byte, header.Size)
			if _, err := io.ReadFull(tr, data); err != nil {
				return nil, fmt.Errorf("reading manifest data: %w", err)
			}

			// Parse manifest
			var manifest ExportManifest
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, fmt.Errorf("parsing manifest: %w", err)
			}

			return &manifest, nil
		}
	}
}
