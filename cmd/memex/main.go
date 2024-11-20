package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"memex/internal/memex/storage"
)

var mx *storage.MXStore

// InitCommand initializes a new repository
func InitCommand(path string) error {
	fmt.Fprintf(os.Stderr, "Initializing repository at %s\n", path)

	// Check if file exists
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "Repository already exists at %s\n", path)
		// Try to open existing repository
		mx, err = storage.OpenMX(path)
		if err != nil {
			return fmt.Errorf("opening existing repository: %w", err)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking repository: %w", err)
	}

	// Create new repository
	var err error
	mx, err = storage.CreateMX(path)
	if err != nil {
		return fmt.Errorf("creating repository: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Repository initialized successfully\n")
	return nil
}

// AddCommand adds a file to the repository
func AddCommand(path string) error {
	fmt.Fprintf(os.Stderr, "Adding file: %s\n", path)

	// Check if file exists
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("checking file: %w", err)
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Read %d bytes from file\n", len(content))

	// Create metadata
	meta := map[string]any{
		"filename": filepath.Base(path),
		"added":    time.Now(),
	}

	// Add to repository
	id, err := mx.AddNode(content, "file", meta)
	if err != nil {
		return fmt.Errorf("adding to repository: %w", err)
	}

	fmt.Printf("Added %s (ID: %s)\n", filepath.Base(path), id[:8])
	return nil
}

// DeleteCommand deletes an object from the repository
func DeleteCommand(id string) error {
	fmt.Fprintf(os.Stderr, "Deleting object: %s\n", id)

	// Get object first to verify it exists and get its name
	obj, err := mx.GetNode(id)
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

	// Delete the object
	if err := mx.DeleteNode(id); err != nil {
		return fmt.Errorf("error deleting object: %w", err)
	}

	name := id[:8]
	if filename, ok := obj.Meta["filename"].(string); ok {
		name = filename
	} else if title, ok := obj.Meta["title"].(string); ok {
		name = title
	}

	fmt.Printf("Deleted %s (ID: %s)\n", name, id[:8])
	return nil
}

// LinkCommand creates a link between objects
func LinkCommand(source, target, linkType string, note string) error {
	fmt.Fprintf(os.Stderr, "Creating link: %s -> %s (%s)\n", source, target, linkType)

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
	fmt.Fprintf(os.Stderr, "Getting links for: %s\n", id)

	// Get object first to verify it exists and get its name
	obj, err := mx.GetNode(id)
	if err != nil {
		return fmt.Errorf("error getting node: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Found node: %+v\n", obj)

	// Get links
	links, err := mx.GetLinks(id)
	if err != nil {
		return fmt.Errorf("error getting links: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Found %d links\n", len(links))

	name := id[:8]
	if filename, ok := obj.Meta["filename"].(string); ok {
		name = filename
	} else if title, ok := obj.Meta["title"].(string); ok {
		name = title
	}

	fmt.Printf("Links for %s (ID: %s):\n\n", name, id[:8])

	if len(links) == 0 {
		fmt.Println("No links found")
		return nil
	}

	for _, link := range links {
		// Get target object name
		targetObj, err := mx.GetNode(link.Target)
		if err != nil {
			continue
		}

		targetName := link.Target[:8]
		if filename, ok := targetObj.Meta["filename"].(string); ok {
			targetName = filename
		} else if title, ok := targetObj.Meta["title"].(string); ok {
			targetName = title
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

func main() {
	// Set up logging
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	log.SetOutput(os.Stderr)

	// Parse flags
	repoPath := flag.String("repo", "", "Repository path")
	flag.Parse()

	// Get command and args
	args := flag.Args()
	if len(args) < 1 {
		log.Fatal("Command required")
	}
	cmd := args[0]
	args = args[1:]

	// Initialize repository
	if *repoPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		*repoPath = filepath.Join(home, "memex.mx")
	}

	fmt.Fprintf(os.Stderr, "Using repository: %s\n", *repoPath)

	if err := InitCommand(*repoPath); err != nil {
		log.Fatal(err)
	}

	if mx == nil {
		log.Fatal("Repository not initialized")
	}

	defer func() {
		fmt.Fprintf(os.Stderr, "Closing repository\n")
		if err := mx.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing repository: %v\n", err)
		}
	}()

	// Execute command
	var err error
	switch cmd {
	case "add":
		if len(args) != 1 {
			log.Fatal("File path required")
		}
		err = AddCommand(args[0])

	case "delete":
		if len(args) != 1 {
			log.Fatal("ID required")
		}
		err = DeleteCommand(args[0])

	case "link":
		if len(args) < 3 {
			log.Fatal("Source, target, and link type required")
		}
		note := ""
		if len(args) > 3 {
			note = args[3]
		}
		err = LinkCommand(args[0], args[1], args[2], note)

	case "links":
		if len(args) != 1 {
			log.Fatal("ID required")
		}
		err = LinksCommand(args[0])

	default:
		log.Fatalf("Unknown command: %s", cmd)
	}

	if err != nil {
		log.Fatal(err)
	}
}
