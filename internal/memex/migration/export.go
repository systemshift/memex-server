package migration

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"memex/internal/memex/storage"
)

// ExportManifest represents the metadata of an export
type ExportManifest struct {
	Version string    `json:"version"`
	Created time.Time `json:"created"`
	Nodes   int       `json:"nodes"`
	Edges   int       `json:"edges"`
	Chunks  int       `json:"chunks"`
	Source  string    `json:"source"`
}

// ExportedLink represents a link in the export format
type ExportedLink struct {
	Source   string         `json:"source"`
	Target   string         `json:"target"`
	Type     string         `json:"type"`
	Meta     map[string]any `json:"meta"`
	Created  time.Time      `json:"created"`
	Modified time.Time      `json:"modified"`
}

// Exporter handles graph export
type Exporter struct {
	store  *storage.MXStore
	writer *tar.Writer
}

// NewExporter creates a new exporter
func NewExporter(store *storage.MXStore, w io.Writer) *Exporter {
	return &Exporter{
		store:  store,
		writer: tar.NewWriter(w),
	}
}

// Export exports the entire graph
func (e *Exporter) Export() error {
	defer e.writer.Close()

	fmt.Printf("Starting export from %s\n", e.store.Path())

	// Write manifest
	manifest := ExportManifest{
		Version: "1",
		Created: time.Now(),
		Source:  e.store.Path(),
	}

	// Export nodes
	fmt.Printf("Exporting nodes...\n")
	if err := e.exportNodes(e.store.Nodes(), &manifest); err != nil {
		return fmt.Errorf("exporting nodes: %w", err)
	}
	fmt.Printf("Exported %d nodes\n", manifest.Nodes)

	// Export edges
	fmt.Printf("Exporting edges...\n")
	if err := e.exportEdges(e.store.Nodes(), &manifest); err != nil {
		return fmt.Errorf("exporting edges: %w", err)
	}
	fmt.Printf("Exported %d edges\n", manifest.Edges)

	// Export chunks
	fmt.Printf("Exporting chunks...\n")
	if err := e.exportChunks(e.store.Nodes(), &manifest); err != nil {
		return fmt.Errorf("exporting chunks: %w", err)
	}
	fmt.Printf("Exported %d chunks\n", manifest.Chunks)

	// Write manifest last (now has correct counts)
	fmt.Printf("Writing manifest...\n")
	if err := e.writeManifest(manifest); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}

// ExportSubgraph exports a subgraph starting from the given nodes
func (e *Exporter) ExportSubgraph(seeds []string, depth int) error {
	defer e.writer.Close()

	fmt.Printf("Starting subgraph export from %s\n", e.store.Path())

	// Write manifest
	manifest := ExportManifest{
		Version: "1",
		Created: time.Now(),
		Source:  e.store.Path(),
	}

	// Collect nodes to export using BFS
	nodesToExport := make(map[string]storage.IndexEntry)
	visited := make(map[string]bool)
	queue := make([]string, len(seeds))
	copy(queue, seeds)

	currentDepth := 0
	nodesAtCurrentDepth := len(queue)
	nodesInNextDepth := 0

	// Add seed nodes to export
	for _, nodeID := range seeds {
		for _, entry := range e.store.Nodes() {
			if fmt.Sprintf("%x", entry.ID[:]) == nodeID {
				nodesToExport[nodeID] = entry
				break
			}
		}
	}

	// If depth is 0, only export seed nodes
	if depth == 0 {
		fmt.Printf("Depth 0: exporting only seed nodes\n")
	} else {
		for len(queue) > 0 && currentDepth < depth {
			// Get next node
			nodeID := queue[0]
			queue = queue[1:]
			nodesAtCurrentDepth--

			// Skip if already visited
			if visited[nodeID] {
				continue
			}
			visited[nodeID] = true

			// Get links and add targets to queue
			links, err := e.store.GetLinks(nodeID)
			if err != nil {
				continue
			}

			for _, link := range links {
				if !visited[link.Target] {
					queue = append(queue, link.Target)
					nodesInNextDepth++

					// Add target node to export
					for _, entry := range e.store.Nodes() {
						if fmt.Sprintf("%x", entry.ID[:]) == link.Target {
							nodesToExport[link.Target] = entry
							break
						}
					}
				}
			}

			// Move to next depth level if needed
			if nodesAtCurrentDepth == 0 {
				currentDepth++
				nodesAtCurrentDepth = nodesInNextDepth
				nodesInNextDepth = 0
				fmt.Printf("Moving to depth %d\n", currentDepth)
			}
		}
	}

	// Convert map to slice
	var entries []storage.IndexEntry
	for _, entry := range nodesToExport {
		entries = append(entries, entry)
	}

	// Export collected nodes
	fmt.Printf("Exporting nodes...\n")
	if err := e.exportNodes(entries, &manifest); err != nil {
		return fmt.Errorf("exporting nodes: %w", err)
	}
	fmt.Printf("Exported %d nodes\n", manifest.Nodes)

	// Export edges between collected nodes
	fmt.Printf("Exporting edges...\n")
	if err := e.exportEdges(entries, &manifest); err != nil {
		return fmt.Errorf("exporting edges: %w", err)
	}
	fmt.Printf("Exported %d edges\n", manifest.Edges)

	// Export chunks for collected nodes
	fmt.Printf("Exporting chunks...\n")
	if err := e.exportChunks(entries, &manifest); err != nil {
		return fmt.Errorf("exporting chunks: %w", err)
	}
	fmt.Printf("Exported %d chunks\n", manifest.Chunks)

	// Write manifest last (now has correct counts)
	fmt.Printf("Writing manifest...\n")
	if err := e.writeManifest(manifest); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}

func (e *Exporter) exportNodes(entries []storage.IndexEntry, manifest *ExportManifest) error {
	for _, entry := range entries {
		nodeID := fmt.Sprintf("%x", entry.ID[:])
		fmt.Printf("Exporting node %s\n", nodeID)

		node, err := e.store.GetNode(nodeID)
		if err != nil {
			return fmt.Errorf("getting node: %w", err)
		}

		// Write node data
		data, err := json.MarshalIndent(node, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling node: %w", err)
		}

		header := &tar.Header{
			Name:    fmt.Sprintf("nodes/%s.json", nodeID),
			Size:    int64(len(data)),
			Mode:    0644,
			ModTime: time.Now(),
		}

		if err := e.writer.WriteHeader(header); err != nil {
			return fmt.Errorf("writing node header: %w", err)
		}

		if _, err := e.writer.Write(data); err != nil {
			return fmt.Errorf("writing node data: %w", err)
		}

		manifest.Nodes++
	}

	return nil
}

func (e *Exporter) exportEdges(entries []storage.IndexEntry, manifest *ExportManifest) error {
	// Track exported edges to avoid duplicates
	exported := make(map[string]bool)

	// Get all nodes first
	for _, entry := range entries {
		nodeID := fmt.Sprintf("%x", entry.ID[:])
		fmt.Printf("Getting links for node %s\n", nodeID)

		// Get links for this node
		links, err := e.store.GetLinks(nodeID)
		if err != nil {
			return fmt.Errorf("getting links: %w", err)
		}

		for _, link := range links {
			// Create unique edge ID
			edgeID := fmt.Sprintf("%s-%s-%s", nodeID, link.Target, link.Type)
			if exported[edgeID] {
				continue
			}

			fmt.Printf("Exporting edge %s\n", edgeID)

			// Get node to get timestamps
			node, err := e.store.GetNode(nodeID)
			if err != nil {
				return fmt.Errorf("getting source node: %w", err)
			}

			// Create exported link format
			exportedLink := ExportedLink{
				Source:   nodeID,
				Target:   link.Target,
				Type:     link.Type,
				Meta:     link.Meta,
				Created:  node.Created,  // Use node timestamps for now
				Modified: node.Modified, // We could add timestamps to Link type later
			}

			// Write edge data
			data, err := json.MarshalIndent(exportedLink, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling edge: %w", err)
			}

			header := &tar.Header{
				Name:    fmt.Sprintf("edges/%s.json", edgeID),
				Size:    int64(len(data)),
				Mode:    0644,
				ModTime: time.Now(),
			}

			if err := e.writer.WriteHeader(header); err != nil {
				return fmt.Errorf("writing edge header: %w", err)
			}

			if _, err := e.writer.Write(data); err != nil {
				return fmt.Errorf("writing edge data: %w", err)
			}

			exported[edgeID] = true
			manifest.Edges++
		}
	}

	return nil
}

func (e *Exporter) exportChunks(entries []storage.IndexEntry, manifest *ExportManifest) error {
	// Track exported chunks to avoid duplicates
	exported := make(map[string]bool)

	// Get all nodes to find chunks
	for _, entry := range entries {
		nodeID := fmt.Sprintf("%x", entry.ID[:])
		fmt.Printf("Getting chunks for node %s\n", nodeID)

		node, err := e.store.GetNode(nodeID)
		if err != nil {
			return fmt.Errorf("getting node: %w", err)
		}

		// Get chunks from metadata
		if chunks, ok := node.Meta["chunks"].([]string); ok {
			for _, chunkHash := range chunks {
				// Skip if already exported
				if exported[chunkHash] {
					continue
				}

				fmt.Printf("Exporting chunk %s\n", chunkHash)

				// Get chunk content
				content, err := e.store.ReconstructContent(chunkHash)
				if err != nil {
					return fmt.Errorf("reconstructing chunk content: %w", err)
				}

				// Write chunk data
				header := &tar.Header{
					Name:    fmt.Sprintf("chunks/%s", chunkHash),
					Size:    int64(len(content)),
					Mode:    0644,
					ModTime: time.Now(),
				}

				if err := e.writer.WriteHeader(header); err != nil {
					return fmt.Errorf("writing chunk header: %w", err)
				}

				if _, err := e.writer.Write(content); err != nil {
					return fmt.Errorf("writing chunk data: %w", err)
				}

				exported[chunkHash] = true
				manifest.Chunks++
			}
		}
	}

	return nil
}

func (e *Exporter) writeManifest(manifest ExportManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	header := &tar.Header{
		Name:    "manifest.json",
		Size:    int64(len(data)),
		Mode:    0644,
		ModTime: time.Now(),
	}

	if err := e.writer.WriteHeader(header); err != nil {
		return fmt.Errorf("writing manifest header: %w", err)
	}

	if _, err := e.writer.Write(data); err != nil {
		return fmt.Errorf("writing manifest data: %w", err)
	}

	return nil
}
