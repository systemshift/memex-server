package memex

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"memex/internal/memex/storage"
)

var repo *storage.MXStore

// InitCommand initializes a new repository
func InitCommand(path string) error {
	var err error
	repo, err = storage.OpenMX(path)
	if err != nil {
		// If repository doesn't exist, create it
		repo, err = storage.CreateMX(path)
		if err != nil {
			return fmt.Errorf("creating repository: %w", err)
		}
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
		"added":    time.Now().Format(time.RFC3339),
	}

	// Add to repository
	id, err := repo.AddNode(content, "file", meta)
	if err != nil {
		return fmt.Errorf("adding to repository: %w", err)
	}

	fmt.Printf("Added %s (ID: %s)\n", filepath.Base(path), id[:8])
	return nil
}

// DeleteCommand deletes an object from the repository
func DeleteCommand(id string) error {
	// Get object first to verify it exists and get its name
	node, err := repo.GetNode(id)
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

	// Delete the object
	if err := repo.DeleteNode(id); err != nil {
		return fmt.Errorf("error deleting object: %w", err)
	}

	name := id[:8]
	if filename, ok := node.Meta["filename"].(string); ok {
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

	err := repo.AddLink(source, target, linkType, meta)
	if err != nil {
		return fmt.Errorf("error creating link: %w", err)
	}

	fmt.Printf("Created %s link from %s to %s\n", linkType, source[:8], target[:8])
	return nil
}

// LinksCommand shows links for an object
func LinksCommand(id string) error {
	// Get object first to verify it exists and get its name
	node, err := repo.GetNode(id)
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

	// Get links
	links, err := repo.GetLinks(id)
	if err != nil {
		return fmt.Errorf("error getting links: %w", err)
	}

	name := id[:8]
	if filename, ok := node.Meta["filename"].(string); ok {
		name = filename
	}

	fmt.Printf("Links for %s (ID: %s):\n\n", name, id[:8])

	if len(links) == 0 {
		fmt.Println("No links found")
		return nil
	}

	for _, link := range links {
		// Get target object name
		targetNode, err := repo.GetNode(link.Target)
		if err != nil {
			continue
		}

		targetName := link.Target[:8]
		if filename, ok := targetNode.Meta["filename"].(string); ok {
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

// StatusCommand shows repository status
func StatusCommand() error {
	fmt.Printf("\nMemex Status ===\n\n")

	// Show connected repo
	fmt.Printf("Repository: %s\n\n", repo.Path())

	// Get all nodes
	var files []storage.IndexEntry
	var notes []storage.IndexEntry
	for _, entry := range repo.Nodes() {
		// Read node metadata
		obj, err := repo.GetNode(fmt.Sprintf("%x", entry.ID[:]))
		if err != nil {
			continue
		}

		if obj.Type == "file" {
			files = append(files, entry)
		} else if obj.Type == "note" {
			notes = append(notes, entry)
		}
	}

	// Show counts
	fmt.Printf("Files (%d):\n", len(files))
	for _, entry := range files {
		obj, err := repo.GetNode(fmt.Sprintf("%x", entry.ID[:]))
		if err != nil {
			continue
		}
		filename := obj.Meta["filename"].(string)
		// Parse the added time from string
		var added time.Time
		if addedStr, ok := obj.Meta["added"].(string); ok {
			added, _ = time.Parse(time.RFC3339, addedStr)
		} else {
			added = time.Now() // fallback if no time found
		}
		fmt.Printf("  %x - %s (%s)\n", entry.ID[:4], filename, added.Format("02 Jan 06 15:04 MST"))
	}

	fmt.Printf("\nNotes (%d):\n", len(notes))
	for _, entry := range notes {
		obj, err := repo.GetNode(fmt.Sprintf("%x", entry.ID[:]))
		if err != nil {
			continue
		}
		// Parse the added time from string
		var added time.Time
		if addedStr, ok := obj.Meta["added"].(string); ok {
			added, _ = time.Parse(time.RFC3339, addedStr)
		} else {
			added = time.Now() // fallback if no time found
		}
		// Get first line of note as title
		if content, ok := obj.Meta["content"].(string); ok {
			title := content
			if len(title) > 50 {
				title = title[:47] + "..."
			}
			fmt.Printf("  %x - %s (%s)\n", entry.ID[:4], title, added.Format("02 Jan 06 15:04 MST"))
		}
	}

	fmt.Println()
	return nil
}

// GetRepository returns the current repository instance
func GetRepository() (*storage.MXStore, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository not initialized")
	}
	return repo, nil
}
