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

func main() {
	// Parse command line arguments
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Usage: memex <command> [args...]")
		fmt.Println("\nCommands:")
		fmt.Println("  add <file>                Add a file")
		fmt.Println("  delete <id>               Delete an object")
		fmt.Println("  link <src> <dst> <type>   Create a link")
		fmt.Println("  search <query>            Search objects")
		fmt.Println("  status                    Show repository status")
		os.Exit(1)
	}

	// Get current directory
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting current directory: %v", err)
	}

	// Open memex
	mx, err := memex.Open(dir)
	if err != nil {
		log.Fatalf("Error opening memex: %v", err)
	}

	// Handle commands
	cmd := args[0]
	switch cmd {
	case "add":
		if len(args) < 2 {
			log.Fatal("Usage: memex add <file>")
		}
		handleAdd(mx, args[1])

	case "delete":
		if len(args) < 2 {
			log.Fatal("Usage: memex delete <id>")
		}
		handleDelete(mx, args[1])

	case "link":
		if len(args) < 4 {
			log.Fatal("Usage: memex link <source> <target> <type> [note]")
		}
		note := ""
		if len(args) > 4 {
			note = args[4]
		}
		handleLink(mx, args[1], args[2], args[3], note)

	case "search":
		if len(args) < 2 {
			log.Fatal("Usage: memex search <query>")
		}
		handleSearch(mx, args[1:])

	case "status":
		handleStatus(mx)

	default:
		log.Fatalf("Unknown command: %s", cmd)
	}
}

func handleAdd(mx *memex.Memex, path string) {
	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	// Create metadata
	meta := map[string]any{
		"filename": filepath.Base(path),
		"added":    time.Now(),
	}

	// Add to repository
	id, err := mx.Add(content, "file", meta)
	if err != nil {
		log.Fatalf("Error adding to repository: %v", err)
	}

	fmt.Printf("Added %s (ID: %s)\n", filepath.Base(path), id[:8])
}

func handleDelete(mx *memex.Memex, id string) {
	// Get object first to verify it exists and get its name
	obj, err := mx.Get(id)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Delete the object
	if err := mx.Delete(id); err != nil {
		log.Fatalf("Error deleting object: %v", err)
	}

	name := id[:8]
	if filename, ok := obj.Meta["filename"].(string); ok {
		name = filename
	} else if title, ok := obj.Meta["title"].(string); ok {
		name = title
	}

	fmt.Printf("Deleted %s (ID: %s)\n", name, id[:8])
}

func handleLink(mx *memex.Memex, source, target, linkType, note string) {
	meta := map[string]any{}
	if note != "" {
		meta["note"] = note
	}

	err := mx.Link(source, target, linkType, meta)
	if err != nil {
		log.Fatalf("Error creating link: %v", err)
	}

	fmt.Printf("Created %s link from %s to %s\n", linkType, source[:8], target[:8])
}

func handleSearch(mx *memex.Memex, terms []string) {
	// Build query from terms
	query := make(map[string]any)
	for _, term := range terms {
		if strings.Contains(term, ":") {
			parts := strings.SplitN(term, ":", 2)
			query[parts[0]] = parts[1]
		} else {
			query["content"] = term
		}
	}

	// Search
	results := mx.Search(query)
	if len(results) == 0 {
		fmt.Println("No results found")
		return
	}

	fmt.Printf("Found %d results:\n\n", len(results))
	for _, obj := range results {
		fmt.Printf("ID: %s\n", obj.ID[:8])
		fmt.Printf("Type: %s\n", obj.Type)
		if title, ok := obj.Meta["title"].(string); ok {
			fmt.Printf("Title: %s\n", title)
		}
		fmt.Printf("Created: %s\n", obj.Created.UTC().Format("02 Jan 06 15:04 MST"))
		fmt.Println()
	}
}

func handleStatus(mx *memex.Memex) {
	fmt.Println("Memex Status ===")
	fmt.Println()

	// List notes
	notes := mx.FindByType("note")
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
	files := mx.FindByType("file")
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
}
