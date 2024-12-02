package migration

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"memex/internal/memex/core"
	"memex/internal/memex/storage"
)

// Exporter handles repository exports
type Exporter struct {
	store  *storage.MXStore
	writer *json.Encoder
}

// NewExporter creates a new exporter
func NewExporter(store *storage.MXStore, w io.Writer) *Exporter {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return &Exporter{
		store:  store,
		writer: encoder,
	}
}

// Export exports the entire repository
func (e *Exporter) Export() error {
	fmt.Printf("Starting full export from %s\n", e.store.Path())

	// Create export
	export := &Export{
		Version:  Version,
		Created:  time.Now(),
		Modified: time.Now(),
		Nodes:    make([]*core.Node, 0),
		Links:    make([]*core.Link, 0),
		Chunks:   make(map[string]bool),
	}

	// Export all nodes
	fmt.Println("Exporting nodes...")
	for _, entry := range e.store.Nodes() {
		id := fmt.Sprintf("%x", entry.ID)
		if err := exportNode(e.store, id, export, make(map[string]bool)); err != nil {
			return fmt.Errorf("exporting nodes: %w", err)
		}
	}
	fmt.Printf("Exported %d nodes\n", len(export.Nodes))

	// Export all links
	fmt.Println("Exporting edges...")
	for _, entry := range e.store.Nodes() {
		id := fmt.Sprintf("%x", entry.ID)
		if err := exportLinks(e.store, id, export); err != nil {
			return fmt.Errorf("exporting edges: %w", err)
		}
	}
	fmt.Printf("Exported %d edges\n", len(export.Links))

	// Export chunks
	fmt.Println("Exporting chunks...")
	if err := exportChunks(e.store, export); err != nil {
		return fmt.Errorf("exporting chunks: %w", err)
	}

	// Write export
	return e.writer.Encode(export)
}

// ExportSubgraph exports a subgraph starting from the given nodes
func (e *Exporter) ExportSubgraph(nodes []string, depth int) error {
	opts := ExportOptions{Depth: depth}
	fmt.Printf("Starting subgraph export from %s\n", e.store.Path())
	if opts.Depth == 0 {
		fmt.Println("Depth 0: exporting only seed nodes")
	} else {
		fmt.Printf("Moving to depth %d\n", opts.Depth)
	}

	// Create export
	export := &Export{
		Version:  Version,
		Created:  time.Now(),
		Modified: time.Now(),
		Nodes:    make([]*core.Node, 0),
		Links:    make([]*core.Link, 0),
		Chunks:   make(map[string]bool),
	}

	// Track visited nodes to avoid cycles
	visited := make(map[string]bool)

	// Export seed nodes
	fmt.Println("Exporting nodes...")
	for _, id := range nodes {
		if err := exportNode(e.store, id, export, visited); err != nil {
			return fmt.Errorf("exporting nodes: %w", err)
		}
	}
	fmt.Printf("Exported %d nodes\n", len(export.Nodes))

	// Export links
	fmt.Println("Exporting edges...")
	for _, id := range nodes {
		if err := exportLinks(e.store, id, export); err != nil {
			return fmt.Errorf("exporting edges: %w", err)
		}
	}
	fmt.Printf("Exported %d edges\n", len(export.Links))

	// Export chunks
	fmt.Println("Exporting chunks...")
	if err := exportChunks(e.store, export); err != nil {
		return fmt.Errorf("exporting chunks: %w", err)
	}

	// Write export
	return e.writer.Encode(export)
}

// SaveExport saves an export to a file
func SaveExport(export *Export, path string) error {
	// Create output directory if needed
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Create file
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer file.Close()

	// Create encoder
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	// Write export
	if err := encoder.Encode(export); err != nil {
		return fmt.Errorf("encoding export: %w", err)
	}

	return nil
}

// LoadExport loads an export from a file
func LoadExport(path string) (*Export, error) {
	// Open file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	// Create decoder
	decoder := json.NewDecoder(file)

	// Read export
	var export Export
	if err := decoder.Decode(&export); err != nil {
		return nil, fmt.Errorf("decoding export: %w", err)
	}

	return &export, nil
}

// Internal functions

func exportNode(store *storage.MXStore, id string, export *Export, visited map[string]bool) error {
	// Skip if already visited
	if visited[id] {
		return nil
	}
	visited[id] = true

	// Get node
	fmt.Printf("Exporting node %s\n", id)
	node, err := store.GetNode(id)
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	// Add node to export
	export.Nodes = append(export.Nodes, node)

	// Add chunks to export
	if chunks, ok := node.Meta["chunks"].([]string); ok {
		for _, chunk := range chunks {
			export.Chunks[chunk] = true
		}
	}

	return nil
}

func exportLinks(store *storage.MXStore, id string, export *Export) error {
	// Get links
	fmt.Printf("Getting links for node %s\n", id)
	links, err := store.GetLinks(id)
	if err != nil {
		return fmt.Errorf("getting links: %w", err)
	}

	// Add links to export
	export.Links = append(export.Links, links...)

	return nil
}

func exportChunks(store *storage.MXStore, export *Export) error {
	// Export each chunk
	for chunk := range export.Chunks {
		fmt.Printf("Exporting chunk %s\n", chunk)
		content, err := store.GetChunk(chunk)
		if err != nil {
			return fmt.Errorf("getting chunk content: %w", err)
		}

		// Verify chunk hash
		hash := sha256.Sum256(content)
		if fmt.Sprintf("%x", hash) != chunk {
			return fmt.Errorf("chunk hash mismatch: got %x want %s", hash, chunk)
		}
	}

	return nil
}
