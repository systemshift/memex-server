package memex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Commands provides command functions for the CLI
type Commands struct {
	memex *Memex
}

// NewCommands creates a new Commands instance
func NewCommands() *Commands {
	return &Commands{}
}

// Close closes any open resources
func (c *Commands) Close() error {
	if c.memex != nil {
		return c.memex.Close()
	}
	return nil
}

// AutoConnect attempts to connect to a repository in the current directory
func (c *Commands) AutoConnect() error {
	// Look for .mx files in current directory
	entries, err := os.ReadDir(".")
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}

	// Find first .mx file
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".mx") {
			memex, err := Open(entry.Name())
			if err != nil {
				return fmt.Errorf("opening repository: %w", err)
			}
			c.memex = memex
			return nil
		}
	}

	return fmt.Errorf("no repository found in current directory")
}

// Init initializes a new repository
func (c *Commands) Init(name string) error {
	if !strings.HasSuffix(name, ".mx") {
		name += ".mx"
	}

	memex, err := Create(name)
	if err != nil {
		return fmt.Errorf("creating repository: %w", err)
	}
	c.memex = memex
	return nil
}

// Connect connects to an existing repository
func (c *Commands) Connect(path string) error {
	if !strings.HasSuffix(path, ".mx") {
		path += ".mx"
	}

	memex, err := Open(path)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}
	c.memex = memex
	return nil
}

// Status shows repository status
func (c *Commands) Status() error {
	if c.memex == nil {
		return fmt.Errorf("no repository connected")
	}

	// Check repository access by listing nodes
	_, err := c.memex.ListNodes()
	if err != nil {
		return fmt.Errorf("checking repository: %w", err)
	}

	fmt.Printf("Status: Ready\n")
	return nil
}

// Add adds a file to the repository
func (c *Commands) Add(path string) error {
	if c.memex == nil {
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

	_, err = c.memex.Add(content, "file", meta)
	return err
}

// Delete removes a node
func (c *Commands) Delete(id string) error {
	if c.memex == nil {
		return fmt.Errorf("no repository connected")
	}
	return c.memex.Delete(id)
}

// Edit opens the editor for a new note
func (c *Commands) Edit() error {
	if c.memex == nil {
		return fmt.Errorf("no repository connected")
	}

	// TODO: Implement editor
	return fmt.Errorf("editor not implemented yet")
}

// Link creates a link between nodes
func (c *Commands) Link(source, target, linkType, note string) error {
	if c.memex == nil {
		return fmt.Errorf("no repository connected")
	}

	meta := map[string]interface{}{}
	if note != "" {
		meta["note"] = note
	}

	return c.memex.Link(source, target, linkType, meta)
}

// Links shows links for a node
func (c *Commands) Links(id string) error {
	if c.memex == nil {
		return fmt.Errorf("no repository connected")
	}

	links, err := c.memex.GetLinks(id)
	if err != nil {
		return err
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

// Export exports the repository
func (c *Commands) Export(path string) error {
	if c.memex == nil {
		return fmt.Errorf("no repository connected")
	}

	// TODO: Implement export
	return fmt.Errorf("export not implemented yet")
}

// Import imports content into the repository
type ImportOptions struct {
	OnConflict string
	Merge      bool
	Prefix     string
}

// Import imports content from a file
func (c *Commands) Import(path string, opts ImportOptions) error {
	if c.memex == nil {
		return fmt.Errorf("no repository connected")
	}

	// TODO: Implement import
	return fmt.Errorf("import not implemented yet")
}

// Module handles module operations
func (c *Commands) Module(args ...string) error {
	if c.memex == nil {
		return fmt.Errorf("no repository connected")
	}

	if len(args) < 1 {
		return fmt.Errorf("module command requires subcommand (list, install, remove, run)")
	}

	switch args[0] {
	case "list":
		// TODO: Implement module listing
		return fmt.Errorf("module listing not implemented yet")

	case "install":
		if len(args) < 2 {
			return fmt.Errorf("install requires module path")
		}
		// TODO: Implement module installation
		return fmt.Errorf("module installation not implemented yet")

	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("remove requires module name")
		}
		// TODO: Implement module removal
		return fmt.Errorf("module removal not implemented yet")

	case "run":
		if len(args) < 2 {
			return fmt.Errorf("run requires module name")
		}
		// TODO: Implement module execution
		return fmt.Errorf("module execution not implemented yet")

	default:
		return fmt.Errorf("unknown module subcommand: %s", args[0])
	}
}
