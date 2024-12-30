package memex

import (
	"fmt"
	"strings"

	"github.com/systemshift/memex/internal/memex/repository"
)

// Commands wraps the command functions into a struct interface
type Commands struct{}

// NewCommands creates a new Commands instance
func NewCommands() *Commands {
	return &Commands{}
}

// Close closes any open resources
func (c *Commands) Close() error {
	return CloseRepository()
}

// AutoConnect attempts to connect to a repository in the current directory
func (c *Commands) AutoConnect() error {
	return OpenRepository()
}

// Init initializes a new repository
func (c *Commands) Init(name string) error {
	return InitCommand(name)
}

// Connect connects to an existing repository
func (c *Commands) Connect(path string) error {
	return ConnectCommand(path)
}

// Add adds a file to the repository
func (c *Commands) Add(path string) error {
	return AddCommand(path)
}

// Delete removes a node
func (c *Commands) Delete(id string) error {
	return DeleteCommand(id)
}

// Edit opens the editor for a new note
func (c *Commands) Edit() error {
	return EditCommand()
}

// Link creates a link between nodes
func (c *Commands) Link(source, target, linkType, note string) error {
	if note == "" {
		return LinkCommand(source, target, linkType)
	}
	return LinkCommand(source, target, linkType, note)
}

// Links shows links for a node
func (c *Commands) Links(id string) error {
	return LinksCommand(id)
}

// Status shows repository status
func (c *Commands) Status() error {
	return StatusCommand()
}

// Export exports the repository
func (c *Commands) Export(path string) error {
	return ExportCommand(path)
}

// ImportOptions defines options for importing
type ImportOptions struct {
	OnConflict string
	Merge      bool
	Prefix     string
}

// ShowVersion shows version information for memex and the repository
func (c *Commands) ShowVersion() error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

	// Show memex version
	fmt.Println("Memex Version:", BuildInfo())

	// Get repository version info
	repoInfo := repo.(*repository.Repository).GetVersionInfo()

	// Show repository version
	fmt.Printf("\nRepository Format Version: %d.%d\n",
		repoInfo.FormatVersion,
		repoInfo.FormatMinor)

	// Show version that created the repository
	repoVersion := strings.TrimRight(repoInfo.MemexVersion, "\x00")
	fmt.Printf("Created by Memex Version: %s\n", repoVersion)

	return nil
}

// Import imports content from a tar archive
func (c *Commands) Import(path string, opts ImportOptions) error {
	args := []string{path}
	if opts.OnConflict != "" {
		args = append(args, "--on-conflict", opts.OnConflict)
	}
	if opts.Merge {
		args = append(args, "--merge")
	}
	if opts.Prefix != "" {
		args = append(args, "--prefix", opts.Prefix)
	}
	return ImportCommand(args...)
}
