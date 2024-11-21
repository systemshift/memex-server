package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"memex/internal/memex"
	"memex/internal/memex/storage"
)

var mx *storage.MXStore

func getConnectedRepo() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".memex"))
	if err != nil {
		return ""
	}
	return string(data)
}

func saveConnectedRepo(path string) error {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(home, ".memex"), []byte(absPath), 0644)
}

// InitCommand initializes a new repository
func InitCommand(name string) error {
	path := name + ".mx"
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if _, err := os.Stat(absPath); err == nil {
		return fmt.Errorf("repository already exists at %s", absPath)
	}

	mx, err = storage.CreateMX(absPath)
	if err != nil {
		return fmt.Errorf("creating repository: %w", err)
	}

	if err := saveConnectedRepo(absPath); err != nil {
		return fmt.Errorf("connecting to new repo: %w", err)
	}

	fmt.Printf("Created repository %s\n", absPath)
	return nil
}

// EditCommand opens the editor
func EditCommand() error {
	editor := memex.NewEditor(mx.Path())
	content, err := editor.Run()
	if err != nil {
		return fmt.Errorf("running editor: %w", err)
	}

	if content == "" {
		return nil // User cancelled
	}

	meta := map[string]any{
		"added": time.Now(),
	}

	id, err := mx.AddNode([]byte(content), "note", meta)
	if err != nil {
		return fmt.Errorf("adding note: %w", err)
	}

	fmt.Printf("Added note (ID: %s)\n", id[:8])
	return nil
}

// AddCommand adds a file to the repository
func AddCommand(path string) error {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	meta := map[string]any{
		"filename": filepath.Base(absPath),
		"added":    time.Now(),
	}

	id, err := mx.AddNode(content, "file", meta)
	if err != nil {
		return fmt.Errorf("adding to repository: %w", err)
	}

	fmt.Printf("Added %s (ID: %s)\n", filepath.Base(absPath), id[:8])
	return nil
}

// DeleteCommand deletes an object from the repository
func DeleteCommand(id string) error {
	obj, err := mx.GetNode(id)
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

	if err := mx.DeleteNode(id); err != nil {
		return fmt.Errorf("error deleting object: %w", err)
	}

	name := id[:8]
	if filename, ok := obj.Meta["filename"].(string); ok {
		name = filename
	}

	fmt.Printf("Deleted %s (ID: %s)\n", name, id[:8])
	return nil
}

// LinkCommand creates a link between objects
func LinkCommand(source, target, linkType string, note string) error {
	meta := map[string]any{}
	if note != "" {
		meta["note"] = note
	}

	err := mx.AddLink(source, target, linkType, meta)
	if err != nil {
		return fmt.Errorf("error creating link: %w", err)
	}

	fmt.Printf("Created %s link from %s to %s\n", linkType, source[:8], target[:8])
	return nil
}

// LinksCommand shows links for an object
func LinksCommand(id string) error {
	obj, err := mx.GetNode(id)
	if err != nil {
		return fmt.Errorf("error getting node: %w", err)
	}

	links, err := mx.GetLinks(id)
	if err != nil {
		return fmt.Errorf("error getting links: %w", err)
	}

	name := id[:8]
	if filename, ok := obj.Meta["filename"].(string); ok {
		name = filename
	}

	fmt.Printf("Links for %s (ID: %s):\n\n", name, id[:8])

	if len(links) == 0 {
		fmt.Println("No links found")
		return nil
	}

	for _, link := range links {
		targetObj, err := mx.GetNode(link.Target)
		if err != nil {
			continue
		}

		targetName := link.Target[:8]
		if filename, ok := targetObj.Meta["filename"].(string); ok {
			targetName = filename
		}

		fmt.Printf("Type: %s\n", link.Type)
		fmt.Printf("Target: %s (ID: %s)\n", targetName, link.Target[:8])
		if note, ok := link.Meta["note"].(string); ok && note != "" {
			fmt.Printf("Note: %s\n", note)
		}
		fmt.Println()
	}

	return nil
}

func showUsage() {
	fmt.Println("Usage:")
	fmt.Println("  memex init <name>     Create a new repository")
	fmt.Println("  memex connect <path>  Connect to an existing repository")
	fmt.Println("  memex add <file>      Add a file to the repository")
	fmt.Println("  memex delete <id>     Delete an object")
	fmt.Println("  memex link <src> <dst> <type> [note]  Create a link between objects")
	fmt.Println("  memex links <id>      Show links for an object")
	os.Exit(1)
}

func main() {
	args := os.Args[1:]

	// Handle init and connect commands first
	if len(args) > 0 {
		switch args[0] {
		case "init":
			if len(args) != 2 {
				fmt.Println("Error: Repository name required")
				showUsage()
			}
			if err := InitCommand(args[1]); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return

		case "connect":
			if len(args) != 2 {
				fmt.Println("Error: Repository path required")
				showUsage()
			}
			// Convert to absolute path
			absPath, err := filepath.Abs(args[1])
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			if err := saveConnectedRepo(absPath); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Connected to %s\n", absPath)
			return
		}
	}

	// For all other commands, need a connected repo
	repoPath := getConnectedRepo()
	if repoPath == "" {
		fmt.Println("Error: No repository connected. Use 'init <name>' or 'connect <path>' first")
		showUsage()
	}

	// Check if the connected repo exists
	if _, err := os.Stat(repoPath); err != nil {
		fmt.Printf("Error: Connected repository '%s' not found. Use 'init <name>' or 'connect <path>' to connect to a valid repository\n", repoPath)
		showUsage()
	}

	// Open the repository
	var err error
	mx, err = storage.OpenMX(repoPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	defer mx.Close()

	// If no command provided, open editor
	if len(args) == 0 {
		if err := EditCommand(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Execute command
	cmd := args[0]
	args = args[1:]

	switch cmd {
	case "add":
		if len(args) != 1 {
			fmt.Println("Error: File path required")
			showUsage()
		}
		err = AddCommand(args[0])

	case "delete":
		if len(args) != 1 {
			fmt.Println("Error: ID required")
			showUsage()
		}
		err = DeleteCommand(args[0])

	case "link":
		if len(args) < 3 {
			fmt.Println("Error: Source, target, and link type required")
			showUsage()
		}
		note := ""
		if len(args) > 3 {
			note = args[3]
		}
		err = LinkCommand(args[0], args[1], args[2], note)

	case "links":
		if len(args) != 1 {
			fmt.Println("Error: ID required")
			showUsage()
		}
		err = LinksCommand(args[0])

	default:
		fmt.Printf("Error: Unknown command: %s\n", cmd)
		showUsage()
	}

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
