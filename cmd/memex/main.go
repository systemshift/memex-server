package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"memex/pkg/memex"
)

var mx *memex.Memex

// InitCommand initializes a new repository
func InitCommand(path string) error {
	var err error
	mx, err = memex.Open(path)
	if err != nil {
		return fmt.Errorf("initializing repository: %w", err)
	}
	return nil
}

// AddCommand adds a file to the repository
func AddCommand(path string) error {
	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Create metadata
	meta := map[string]any{
		"filename": filepath.Base(path),
		"added":    time.Now(),
	}

	// Add to repository
	id, err := mx.Add(content, "file", meta)
	if err != nil {
		return fmt.Errorf("adding to repository: %w", err)
	}

	fmt.Printf("Added %s (ID: %s)\n", filepath.Base(path), id[:8])
	return nil
}

// DeleteCommand deletes an object from the repository
func DeleteCommand(id string) error {
	// Get object first to verify it exists and get its name
	obj, err := mx.Get(id)
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

	// Delete the object
	if err := mx.Delete(id); err != nil {
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
	meta := map[string]any{}
	if note != "" {
		meta["note"] = note
	}

	err := mx.Link(source, target, linkType, meta)
	if err != nil {
		return fmt.Errorf("error creating link: %w", err)
	}

	fmt.Printf("Created %s link from %s to %s\n", linkType, source[:8], target[:8])
	return nil
}

// LinksCommand shows links for an object
func LinksCommand(id string) error {
	// Get object first to verify it exists and get its name
	obj, err := mx.Get(id)
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

	// Get links
	links, err := mx.GetLinks(id)
	if err != nil {
		return fmt.Errorf("error getting links: %w", err)
	}

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
		targetObj, err := mx.Get(link.Target)
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

// UpdateCommand updates an object's content
func UpdateCommand(id string, path string) error {
	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Update object
	if err := mx.Update(id, content); err != nil {
		return fmt.Errorf("error updating object: %w", err)
	}

	// Get updated object to show name
	obj, err := mx.Get(id)
	if err != nil {
		return fmt.Errorf("error getting updated object: %w", err)
	}

	name := id[:8]
	if filename, ok := obj.Meta["filename"].(string); ok {
		name = filename
	} else if title, ok := obj.Meta["title"].(string); ok {
		name = title
	}

	fmt.Printf("Updated %s (ID: %s)\n", name, id[:8])
	return nil
}

// SearchCommand searches for objects
func SearchCommand(query string) error {
	// Parse query into map
	queryMap := make(map[string]any)
	queryMap["content"] = query

	// Search
	results, err := mx.Search(queryMap)
	if err != nil {
		return fmt.Errorf("error searching: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found")
		return nil
	}

	fmt.Printf("Found %d results:\n\n", len(results))
	for _, obj := range results {
		name := obj.ID[:8]
		if filename, ok := obj.Meta["filename"].(string); ok {
			name = filename
		} else if title, ok := obj.Meta["title"].(string); ok {
			name = title
		}

		fmt.Printf("%s (ID: %s)\n", name, obj.ID[:8])
		fmt.Printf("Type: %s\n", obj.Type)
		fmt.Printf("Created: %s\n", obj.Created.Format("02 Jan 06 15:04 MST"))
		fmt.Println()
	}

	return nil
}

// StatusCommand shows repository status
func StatusCommand() error {
	fmt.Println("Memex Status ===")
	fmt.Println()

	// List notes
	notes, err := mx.FindByType("note")
	if err != nil {
		return fmt.Errorf("finding notes: %w", err)
	}

	if len(notes) > 0 {
		fmt.Printf("Notes (%d):\n", len(notes))
		for _, obj := range notes {
			title := "Untitled"
			if t, ok := obj.Meta["title"].(string); ok {
				title = t
			}
			fmt.Printf("  %s - %s (%s)\n", obj.ID[:8], title, obj.Created.UTC().Format("02 Jan 06 15:04 MST"))
		}
		fmt.Println()
	}

	// List files
	files, err := mx.FindByType("file")
	if err != nil {
		return fmt.Errorf("finding files: %w", err)
	}

	if len(files) > 0 {
		fmt.Printf("Files (%d):\n", len(files))
		for _, obj := range files {
			filename := "unknown"
			if f, ok := obj.Meta["filename"].(string); ok {
				filename = f
			}
			fmt.Printf("  %s - %s (%s)\n", obj.ID[:8], filename, obj.Created.UTC().Format("02 Jan 06 15:04 MST"))
		}
		fmt.Println()
	}

	if len(notes) == 0 && len(files) == 0 {
		fmt.Println("No content found")
	}

	return nil
}

func main() {
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
		*repoPath = filepath.Join(home, ".memex")
	}

	if err := InitCommand(*repoPath); err != nil {
		log.Fatal(err)
	}

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

	case "update":
		if len(args) != 2 {
			log.Fatal("ID and file path required")
		}
		err = UpdateCommand(args[0], args[1])

	case "search":
		if len(args) != 1 {
			log.Fatal("Search query required")
		}
		err = SearchCommand(args[0])

	case "status":
		err = StatusCommand()

	default:
		log.Fatalf("Unknown command: %s", cmd)
	}

	if err != nil {
		log.Fatal(err)
	}
}
