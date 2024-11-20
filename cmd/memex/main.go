package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memex/pkg/memex"
)

var mx *memex.Memex

func initCommand(path string) error {
	var err error
	mx, err = memex.Open(path)
	if err != nil {
		return fmt.Errorf("initializing repository: %w", err)
	}
	return nil
}

func addCommand(path string) error {
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

func deleteCommand(id string) error {
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

func linkCommand(source, target, linkType string, note string) error {
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

func linksCommand(id string) error {
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

func updateCommand(id string, content []byte) error {
	// Get existing object to preserve metadata
	obj, err := mx.Get(id)
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

	// Update with new content but keep metadata
	if err := mx.Update(id, content, obj.Meta); err != nil {
		return fmt.Errorf("error updating object: %w", err)
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

func searchCommand(query string) error {
	// Parse query into map
	queryMap := make(map[string]any)
	parts := strings.Split(query, ":")
	if len(parts) == 2 {
		queryMap[parts[0]] = parts[1]
	} else {
		queryMap["content"] = query
	}

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

func statusCommand() error {
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

	if err := initCommand(*repoPath); err != nil {
		log.Fatal(err)
	}

	// Execute command
	var err error
	switch cmd {
	case "add":
		if len(args) != 1 {
			log.Fatal("File path required")
		}
		err = addCommand(args[0])

	case "delete":
		if len(args) != 1 {
			log.Fatal("ID required")
		}
		err = deleteCommand(args[0])

	case "link":
		if len(args) < 3 {
			log.Fatal("Source, target, and link type required")
		}
		note := ""
		if len(args) > 3 {
			note = args[3]
		}
		err = linkCommand(args[0], args[1], args[2], note)

	case "links":
		if len(args) != 1 {
			log.Fatal("ID required")
		}
		err = linksCommand(args[0])

	case "update":
		if len(args) != 2 {
			log.Fatal("ID and file path required")
		}
		content, err := os.ReadFile(args[1])
		if err != nil {
			log.Fatal(err)
		}
		err = updateCommand(args[0], content)

	case "search":
		if len(args) != 1 {
			log.Fatal("Search query required")
		}
		err = searchCommand(args[0])

	case "status":
		err = statusCommand()

	default:
		log.Fatalf("Unknown command: %s", cmd)
	}

	if err != nil {
		log.Fatal(err)
	}
}
