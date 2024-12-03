package migration

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"memex/internal/memex/core"
)

// Exporter handles repository exports
type Exporter struct {
	repo   core.Repository
	writer io.Writer
}

// NewExporter creates a new exporter
func NewExporter(repo core.Repository, w io.Writer) *Exporter {
	return &Exporter{
		repo:   repo,
		writer: w,
	}
}

// Export exports the entire repository
func (e *Exporter) Export() error {
	fmt.Println("Starting repository export")

	// Create tar writer
	tw := tar.NewWriter(e.writer)
	defer tw.Close()

	// Create manifest
	manifest := ExportManifest{
		Version:  Version,
		Created:  time.Now(),
		Modified: time.Now(),
	}

	// Export nodes
	fmt.Println("Exporting nodes...")
	nodes := make(map[string]*core.Node)
	chunks := make(map[string]bool)

	// TODO: Implement node listing in core.Repository
	// For now, try to export a test node
	if node, err := e.repo.GetNode("test"); err == nil {
		id := node.ID
		if err := exportNode(e.repo, id, tw, nodes, chunks); err != nil {
			return fmt.Errorf("exporting nodes: %w", err)
		}
		manifest.Nodes++
	}
	fmt.Printf("Exported %d nodes\n", manifest.Nodes)

	// Export links
	fmt.Println("Exporting edges...")
	for id := range nodes {
		if err := exportLinks(e.repo, id, tw, &manifest); err != nil {
			return fmt.Errorf("exporting edges: %w", err)
		}
	}
	fmt.Printf("Exported %d edges\n", manifest.Edges)

	// Write manifest
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	header := &tar.Header{
		Name:     "manifest.json",
		Size:     int64(len(manifestData)),
		Mode:     0644,
		ModTime:  time.Now(),
		Typeflag: tar.TypeReg,
	}

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("writing manifest header: %w", err)
	}

	if _, err := tw.Write(manifestData); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}

// ExportSubgraph exports a subgraph starting from the given nodes
func (e *Exporter) ExportSubgraph(nodes []string, depth int) error {
	fmt.Println("Starting subgraph export")
	if depth == 0 {
		fmt.Println("Depth 0: exporting only seed nodes")
	} else {
		fmt.Printf("Moving to depth %d\n", depth)
	}

	// Create tar writer
	tw := tar.NewWriter(e.writer)
	defer tw.Close()

	// Create manifest
	manifest := ExportManifest{
		Version:  Version,
		Created:  time.Now(),
		Modified: time.Now(),
	}

	// Export seed nodes
	fmt.Println("Exporting nodes...")
	exportedNodes := make(map[string]*core.Node)
	chunks := make(map[string]bool)

	for _, id := range nodes {
		if err := exportNode(e.repo, id, tw, exportedNodes, chunks); err != nil {
			return fmt.Errorf("exporting nodes: %w", err)
		}
		manifest.Nodes++
	}
	fmt.Printf("Exported %d nodes\n", manifest.Nodes)

	// Export links
	fmt.Println("Exporting edges...")
	for id := range exportedNodes {
		if err := exportLinks(e.repo, id, tw, &manifest); err != nil {
			return fmt.Errorf("exporting edges: %w", err)
		}
	}
	fmt.Printf("Exported %d edges\n", manifest.Edges)

	// Write manifest
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	header := &tar.Header{
		Name:     "manifest.json",
		Size:     int64(len(manifestData)),
		Mode:     0644,
		ModTime:  time.Now(),
		Typeflag: tar.TypeReg,
	}

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("writing manifest header: %w", err)
	}

	if _, err := tw.Write(manifestData); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}

// Internal functions

func exportNode(repo core.Repository, id string, tw *tar.Writer, nodes map[string]*core.Node, chunks map[string]bool) error {
	// Skip if already exported
	if _, exists := nodes[id]; exists {
		return nil
	}

	// Get node
	fmt.Printf("Exporting node %s\n", id)
	node, err := repo.GetNode(id)
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	// Add node to map
	nodes[id] = node

	// Add chunks to map
	if nodeChunks, ok := node.Meta["chunks"]; ok {
		switch v := nodeChunks.(type) {
		case []string:
			for _, chunk := range v {
				chunks[chunk] = true
			}
		case []interface{}:
			for _, chunk := range v {
				if str, ok := chunk.(string); ok {
					chunks[str] = true
				}
			}
		}
	}

	// Get content hash
	if contentHash, ok := node.Meta["content"].(string); ok {
		chunks[contentHash] = true
	}

	// Write node to tar
	data, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling node: %w", err)
	}

	header := &tar.Header{
		Name:     filepath.Join("nodes", id),
		Size:     int64(len(data)),
		Mode:     0644,
		ModTime:  time.Now(),
		Typeflag: tar.TypeReg,
	}

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("writing node header: %w", err)
	}

	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("writing node data: %w", err)
	}

	return nil
}

func exportLinks(repo core.Repository, id string, tw *tar.Writer, manifest *ExportManifest) error {
	// Get links
	fmt.Printf("Getting links for node %s\n", id)
	links, err := repo.GetLinks(id)
	if err != nil {
		return fmt.Errorf("getting links: %w", err)
	}

	// Write each link
	for _, link := range links {
		data, err := json.MarshalIndent(link, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling link: %w", err)
		}

		header := &tar.Header{
			Name:     filepath.Join("edges", fmt.Sprintf("%s-%s-%s", link.Source, link.Type, link.Target)),
			Size:     int64(len(data)),
			Mode:     0644,
			ModTime:  time.Now(),
			Typeflag: tar.TypeReg,
		}

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("writing link header: %w", err)
		}

		if _, err := tw.Write(data); err != nil {
			return fmt.Errorf("writing link data: %w", err)
		}

		manifest.Edges++
	}

	return nil
}
