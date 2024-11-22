package memex

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"memex/internal/memex/migration"
	"memex/internal/memex/storage"
)

var (
	currentRepo *storage.MXStore
	repoPath    string
)

// InitCommand initializes a new repository
func InitCommand(args ...string) error {
	if len(args) < 1 {
		return fmt.Errorf("init requires repository name")
	}

	name := args[0]
	if !strings.HasSuffix(name, ".mx") {
		name += ".mx"
	}

	repo, err := storage.CreateMX(name)
	if err != nil {
		return fmt.Errorf("creating repository: %w", err)
	}

	currentRepo = repo
	repoPath = name
	return nil
}

// ConnectCommand connects to an existing repository
func ConnectCommand(args ...string) error {
	if len(args) < 1 {
		return fmt.Errorf("connect requires repository path")
	}

	path := args[0]
	if !strings.HasSuffix(path, ".mx") {
		path += ".mx"
	}

	repo, err := storage.OpenMX(path)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}

	currentRepo = repo
	repoPath = path
	return nil
}

// GetRepository returns the current repository
func GetRepository() (*storage.MXStore, error) {
	if currentRepo == nil {
		return nil, fmt.Errorf("no repository connected")
	}
	return currentRepo, nil
}

// OpenRepository opens a repository in the current directory
func OpenRepository() error {
	// Look for .mx files in current directory
	entries, err := os.ReadDir(".")
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}

	// Find first .mx file
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".mx") {
			repo, err := storage.OpenMX(entry.Name())
			if err != nil {
				return fmt.Errorf("opening repository: %w", err)
			}

			currentRepo = repo
			repoPath = entry.Name()
			return nil
		}
	}

	return fmt.Errorf("no repository found in current directory")
}

// CloseRepository closes the current repository
func CloseRepository() error {
	if currentRepo != nil {
		if err := currentRepo.Close(); err != nil {
			return fmt.Errorf("closing repository: %w", err)
		}
		currentRepo = nil
		repoPath = ""
	}
	return nil
}

// EditCommand opens the editor for a new note
func EditCommand(args ...string) error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

	editor := NewEditor(repoPath)
	content, err := editor.Run()
	if err != nil {
		return fmt.Errorf("editing content: %w", err)
	}

	if len(content) == 0 {
		return fmt.Errorf("empty note not saved")
	}

	meta := map[string]any{
		"type": "note",
	}

	if _, err := repo.AddNode([]byte(content), "note", meta); err != nil {
		return fmt.Errorf("adding note: %w", err)
	}

	return nil
}

// StatusCommand shows repository status
func StatusCommand(args ...string) error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

	fmt.Printf("Repository: %s\n", repoPath)
	fmt.Printf("Nodes: %d\n", len(repo.Nodes()))

	// List all nodes
	for _, entry := range repo.Nodes() {
		node, err := repo.GetNode(fmt.Sprintf("%x", entry.ID[:]))
		if err != nil {
			continue
		}
		fmt.Printf("Node ID: %s\n", node.ID)
		if filename, ok := node.Meta["filename"].(string); ok {
			fmt.Printf("  File: %s\n", filename)
		}
	}
	return nil
}

// AddCommand adds a file to the repository
func AddCommand(args ...string) error {
	if len(args) < 1 {
		return fmt.Errorf("add requires file path")
	}

	path := args[0]
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	repo, err := GetRepository()
	if err != nil {
		return err
	}

	meta := map[string]any{
		"filename": filepath.Base(path),
		"type":     "file",
	}

	if _, err := repo.AddNode(content, "file", meta); err != nil {
		return fmt.Errorf("adding file: %w", err)
	}

	return nil
}

// DeleteCommand removes a node
func DeleteCommand(args ...string) error {
	if len(args) < 1 {
		return fmt.Errorf("delete requires node ID")
	}

	repo, err := GetRepository()
	if err != nil {
		return err
	}

	return repo.DeleteNode(args[0])
}

// LinkCommand creates a link between nodes
func LinkCommand(args ...string) error {
	if len(args) < 3 {
		return fmt.Errorf("link requires source, target, and type")
	}

	repo, err := GetRepository()
	if err != nil {
		return err
	}

	source := args[0]
	target := args[1]
	linkType := args[2]

	meta := map[string]any{}
	if len(args) > 3 {
		meta["note"] = args[3]
	}

	return repo.AddLink(source, target, linkType, meta)
}

// LinksCommand shows links for a node
func LinksCommand(args ...string) error {
	if len(args) < 1 {
		return fmt.Errorf("links requires node ID")
	}

	repo, err := GetRepository()
	if err != nil {
		return err
	}

	links, err := repo.GetLinks(args[0])
	if err != nil {
		return err
	}

	for _, link := range links {
		fmt.Printf("%s -> %s [%s]\n", args[0], link.Target, link.Type)
		if note, ok := link.Meta["note"].(string); ok {
			fmt.Printf("  Note: %s\n", note)
		}
	}

	return nil
}

// ExportCommand exports the repository to a tar archive
func ExportCommand(args ...string) error {
	if len(args) < 1 {
		return fmt.Errorf("export requires output file path")
	}

	// Get output path
	outputPath := args[0]

	// Parse flags
	var nodes []string
	var depth int
	if len(args) > 1 && args[1] == "--nodes" {
		if len(args) < 3 {
			return fmt.Errorf("--nodes requires node IDs")
		}
		// Split comma-separated node IDs
		nodes = strings.Split(args[2], ",")

		// If depth specified
		if len(args) > 3 && args[3] == "--depth" {
			if len(args) < 5 {
				return fmt.Errorf("--depth requires a number")
			}
			var err error
			depth, err = strconv.Atoi(args[4])
			if err != nil {
				return fmt.Errorf("invalid depth: %w", err)
			}
		}
	}

	// Ensure output path has .tar extension
	if filepath.Ext(outputPath) != ".tar" {
		outputPath += ".tar"
	}

	// Create output file
	output, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer output.Close()

	// Get current repository
	repo, err := GetRepository()
	if err != nil {
		return fmt.Errorf("getting repository: %w", err)
	}

	// Create exporter
	exporter := migration.NewExporter(repo, output)

	// Export
	if len(nodes) > 0 {
		fmt.Printf("Exporting subgraph from nodes: %v (depth: %d)\n", nodes, depth)
		if err := exporter.ExportSubgraph(nodes, depth); err != nil {
			return fmt.Errorf("exporting subgraph: %w", err)
		}
	} else {
		if err := exporter.Export(); err != nil {
			return fmt.Errorf("exporting repository: %w", err)
		}
	}

	fmt.Printf("Repository exported to %s\n", outputPath)
	return nil
}
