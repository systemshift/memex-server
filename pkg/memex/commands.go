package memex

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"memex/internal/memex/migration"
)

// Commands provides CLI operations for Memex
type Commands struct {
	mx *Memex
}

// ImportOptions configures import behavior
type ImportOptions struct {
	Prefix     string // Prefix to add to imported node IDs
	OnConflict string // How to handle ID conflicts (skip/replace/rename)
	Merge      bool   // Whether to merge with existing content
}

// NewCommands creates a new Commands instance
func NewCommands() *Commands {
	return &Commands{}
}

// Init initializes a new repository
func (c *Commands) Init(name string) error {
	mx, err := Create(name)
	if err != nil {
		return fmt.Errorf("creating repository: %w", err)
	}
	c.mx = mx
	return nil
}

// Connect connects to an existing repository
func (c *Commands) Connect(path string) error {
	mx, err := Open(path)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}
	c.mx = mx
	return nil
}

// Add adds a file to the repository
func (c *Commands) Add(path string) error {
	if c.mx == nil {
		return fmt.Errorf("no repository connected")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	meta := map[string]interface{}{
		"filename": filepath.Base(path),
		"type":     "file",
	}

	_, err = c.mx.Add(content, "file", meta)
	if err != nil {
		return fmt.Errorf("adding file: %w", err)
	}

	return nil
}

// Delete removes a node
func (c *Commands) Delete(id string) error {
	if c.mx == nil {
		return fmt.Errorf("no repository connected")
	}
	return c.mx.Delete(id)
}

// Link creates a link between nodes
func (c *Commands) Link(source, target, linkType string, note string) error {
	if c.mx == nil {
		return fmt.Errorf("no repository connected")
	}

	meta := map[string]interface{}{}
	if note != "" {
		meta["note"] = note
	}

	return c.mx.Link(source, target, linkType, meta)
}

// Links shows links for a node
func (c *Commands) Links(id string) error {
	if c.mx == nil {
		return fmt.Errorf("no repository connected")
	}

	links, err := c.mx.GetLinks(id)
	if err != nil {
		return fmt.Errorf("getting links: %w", err)
	}

	fmt.Printf("Links for node %s:\n", id)
	for _, link := range links {
		fmt.Printf("%s -> %s [%s]\n", id[:8], link.Target[:8], link.Type)
		if note, ok := link.Meta["note"].(string); ok {
			fmt.Printf("  Note: %s\n", note)
		}
	}

	return nil
}

// Status shows repository status
func (c *Commands) Status() error {
	if c.mx == nil {
		return fmt.Errorf("no repository connected")
	}

	// For now, just verify we can list nodes
	_, err := c.mx.ListNodes()
	if err != nil {
		return fmt.Errorf("checking repository: %w", err)
	}

	fmt.Printf("Status: Ready\n")
	return nil
}

// Export exports the repository to a tar archive
func (c *Commands) Export(path string) error {
	if c.mx == nil {
		return fmt.Errorf("no repository connected")
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating export file: %w", err)
	}
	defer f.Close()

	exporter := migration.NewExporter(c.mx.repo, f)
	return exporter.Export()
}

// Import imports a repository from a tar archive
func (c *Commands) Import(path string, opts ImportOptions) error {
	if c.mx == nil {
		return fmt.Errorf("no repository connected")
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening import file: %w", err)
	}
	defer f.Close()

	migrationOpts := migration.ImportOptions{
		Prefix:     opts.Prefix,
		OnConflict: opts.OnConflict,
		Merge:      opts.Merge,
	}

	importer := migration.NewImporter(c.mx.repo, f, migrationOpts)
	return importer.Import()
}

// Edit opens an editor for creating or editing content
func (c *Commands) Edit() error {
	if c.mx == nil {
		return fmt.Errorf("no repository connected")
	}

	// Create temporary file
	tmpfile, err := os.CreateTemp("", "memex-*.txt")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpfile.Name())

	// Get editor from environment or default to vi
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	// Open editor
	cmd := exec.Command(editor, tmpfile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running editor: %w", err)
	}

	// Read edited content
	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		return fmt.Errorf("reading edited content: %w", err)
	}

	// Skip if empty
	if len(content) == 0 {
		return nil
	}

	// Add to repository
	meta := map[string]interface{}{
		"type": "note",
	}

	_, err = c.mx.Add(content, "note", meta)
	if err != nil {
		return fmt.Errorf("adding note: %w", err)
	}

	return nil
}

// Close closes the repository
func (c *Commands) Close() error {
	if c.mx == nil {
		return nil
	}
	return c.mx.Close()
}

// AutoConnect attempts to find and connect to a repository in the current directory
func (c *Commands) AutoConnect() error {
	// Look for .mx files in current directory
	entries, err := os.ReadDir(".")
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}

	// Find first .mx file
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".mx" {
			return c.Connect(entry.Name())
		}
	}

	return fmt.Errorf("no repository found in current directory")
}
