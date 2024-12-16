package memex

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"memex/internal/memex/core"
	"memex/internal/memex/migration"
	"memex/internal/memex/repository"
)

var (
	currentRepo   core.Repository
	repoPath      string
	moduleManager *ModuleManager
)

// SetRepository sets the current repository (used for testing)
func SetRepository(repo core.Repository) {
	currentRepo = repo
	repoPath = "test.mx"
	if moduleManager == nil {
		moduleManager, _ = NewModuleManager()
	}
	moduleManager.SetRepository(repo)
}

// GetModuleCommands returns available commands for a module
func GetModuleCommands(moduleID string) ([]core.ModuleCommand, error) {
	if moduleManager == nil {
		var err error
		moduleManager, err = NewModuleManager()
		if err != nil {
			return nil, fmt.Errorf("initializing module manager: %w", err)
		}
		if currentRepo != nil {
			moduleManager.SetRepository(currentRepo)
		}
	}
	return moduleManager.GetModuleCommands(moduleID)
}

// ModuleCommand handles module operations
func ModuleCommand(args ...string) error {
	if len(args) < 1 {
		return fmt.Errorf("module command requires subcommand (list, install, remove)")
	}

	fmt.Fprintf(os.Stderr, "Debug: Initializing module manager...\n")
	// Initialize module manager if needed
	if moduleManager == nil {
		var err error
		moduleManager, err = NewModuleManager()
		if err != nil {
			return fmt.Errorf("initializing module manager: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Debug: Module manager initialized\n")
	} else {
		fmt.Fprintf(os.Stderr, "Debug: Module manager already initialized\n")
	}

	fmt.Fprintf(os.Stderr, "Debug: Getting repository...\n")
	// Get current repository
	repo, err := GetRepository()
	if err != nil {
		return fmt.Errorf("getting repository: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Debug: Got repository\n")

	fmt.Fprintf(os.Stderr, "Debug: Setting repository for module manager...\n")
	// Set repository for module manager
	moduleManager.SetRepository(repo)
	fmt.Fprintf(os.Stderr, "Debug: Repository set for module manager\n")

	cmd := args[0]
	switch cmd {
	case "list":
		// List installed modules
		modules := moduleManager.ListModules()
		if len(modules) == 0 {
			fmt.Println("No modules installed")
			return nil
		}

		fmt.Println("Installed modules:")
		for _, moduleID := range modules {
			if config, exists := moduleManager.GetModuleConfig(moduleID); exists {
				fmt.Printf("  %s (%s)\n", moduleID, config.Type)
				fmt.Printf("    Path: %s\n", config.Path)

				// Show available commands
				if commands, err := moduleManager.GetModuleCommands(moduleID); err == nil && len(commands) > 0 {
					fmt.Println("    Commands:")
					for _, cmd := range commands {
						fmt.Printf("      %s - %s\n", cmd.Name, cmd.Description)
					}
				}
			}
		}
		return nil

	case "install":
		if len(args) < 2 {
			return fmt.Errorf("install requires module path")
		}
		if err := moduleManager.InstallModule(args[1]); err != nil {
			return fmt.Errorf("installing module: %w", err)
		}
		fmt.Printf("Module installed: %s\n", filepath.Base(args[1]))
		return nil

	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("remove requires module name")
		}
		if err := moduleManager.RemoveModule(args[1]); err != nil {
			return fmt.Errorf("removing module: %w", err)
		}
		fmt.Printf("Module removed: %s\n", args[1])
		return nil

	default:
		// Try to handle as module command (e.g., ast parse main.go)
		moduleID := args[0]
		if len(args) < 2 {
			return fmt.Errorf("module command required")
		}
		cmd := args[1]
		cmdArgs := args[2:]

		return moduleManager.HandleCommand(moduleID, cmd, cmdArgs)
	}
}

// StatusCommand shows repository status
func StatusCommand(args ...string) error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

	fmt.Printf("Repository: %s\n", repoPath)

	// Check repository access by listing nodes
	_, err = repo.ListNodes()
	if err != nil {
		return fmt.Errorf("checking repository: %w", err)
	}

	fmt.Printf("Status: Ready\n")
	return nil
}

// InitCommand initializes a new repository
func InitCommand(args ...string) error {
	if len(args) < 1 {
		return fmt.Errorf("init requires repository name")
	}

	name := args[0]
	if !strings.HasSuffix(name, ".mx") {
		name += ".mx"
	}

	repo, err := repository.Create(name)
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

	repo, err := repository.Open(path)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}

	currentRepo = repo
	repoPath = path
	return nil
}

// GetRepository returns the current repository
func GetRepository() (core.Repository, error) {
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
			repo, err := repository.Open(entry.Name())
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

	ed := NewEditor(repoPath)
	content, err := ed.Run()
	if err != nil {
		return fmt.Errorf("editing content: %w", err)
	}

	if len(content) == 0 {
		return fmt.Errorf("empty note not saved")
	}

	meta := map[string]interface{}{
		"type": "note",
	}

	if _, err := repo.AddNode([]byte(content), "note", meta); err != nil {
		return fmt.Errorf("adding note: %w", err)
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

	meta := map[string]interface{}{
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

	meta := map[string]interface{}{}
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

	nodeID := args[0]
	links, err := repo.GetLinks(nodeID)
	if err != nil {
		return err
	}

	fmt.Printf("Links for node %s:\n", nodeID)
	for _, link := range links {
		fmt.Printf("%s -> %s [%s]\n", nodeID[:8], link.Target[:8], link.Type)
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

// ImportCommand imports content from a tar archive
func ImportCommand(args ...string) error {
	if len(args) < 1 {
		return fmt.Errorf("import requires input file path")
	}

	// Get input path
	inputPath := args[0]

	// Parse flags
	var opts migration.ImportOptions
	opts.OnConflict = migration.Skip // Default to skip conflicts
	opts.Merge = false               // Default to no merge

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--on-conflict":
			if i+1 >= len(args) {
				return fmt.Errorf("--on-conflict requires strategy (skip, replace, rename)")
			}
			i++
			switch args[i] {
			case "skip":
				opts.OnConflict = migration.Skip
			case "replace":
				opts.OnConflict = migration.Replace
			case "rename":
				opts.OnConflict = migration.Rename
			default:
				return fmt.Errorf("invalid conflict strategy: %s", args[i])
			}

		case "--merge":
			opts.Merge = true

		case "--prefix":
			if i+1 >= len(args) {
				return fmt.Errorf("--prefix requires value")
			}
			i++
			opts.Prefix = args[i]
		}
	}

	// Open input file
	input, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("opening input file: %w", err)
	}
	defer input.Close()

	// Get current repository
	repo, err := GetRepository()
	if err != nil {
		return fmt.Errorf("getting repository: %w", err)
	}

	// Create importer
	importer := migration.NewImporter(repo, input, opts)

	// Import
	if err := importer.Import(); err != nil {
		return fmt.Errorf("importing content: %w", err)
	}

	fmt.Printf("Content imported from %s\n", inputPath)
	return nil
}
