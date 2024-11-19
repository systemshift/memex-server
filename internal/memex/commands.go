package memex

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"memex/internal/memex/storage"
)

var repo *storage.DAGStore

// InitCommand initializes a new repository
func InitCommand(path string) error {
	// Always use ~/.memex for repository
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}
	repoPath := filepath.Join(homeDir, ".memex")

	repo, err = storage.NewDAGStore(repoPath)
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
	} else if title, ok := node.Meta["title"].(string); ok {
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

	err := repo.AddLink(source, target, linkType, meta)
	if err != nil {
		return fmt.Errorf("error creating link: %w", err)
	}

	fmt.Printf("Created %s link from %s to %s\n", linkType, source[:8], target[:8])
	return nil
}

// StatusCommand shows repository status
func StatusCommand() error {
	fmt.Println("Memex Status ===")
	fmt.Println()

	// List notes
	notes, err := repo.FindByType("note")
	if err != nil {
		return fmt.Errorf("finding notes: %w", err)
	}

	if len(notes) > 0 {
		fmt.Printf("Notes (%d):\n", len(notes))
		for _, node := range notes {
			title := "Untitled"
			if t, ok := node.Meta["title"].(string); ok {
				title = t
			}
			fmt.Printf("  %s - %s (%s)\n", node.ID[:8], title, node.Created.UTC().Format("02 Jan 06 15:04 MST"))
		}
		fmt.Println()
	}

	// List files
	files, err := repo.FindByType("file")
	if err != nil {
		return fmt.Errorf("finding files: %w", err)
	}

	if len(files) > 0 {
		fmt.Printf("Files (%d):\n", len(files))
		for _, node := range files {
			filename := "unknown"
			if f, ok := node.Meta["filename"].(string); ok {
				filename = f
			}
			fmt.Printf("  %s - %s (%s)\n", node.ID[:8], filename, node.Created.UTC().Format("02 Jan 06 15:04 MST"))
		}
		fmt.Println()
	}

	if len(notes) == 0 && len(files) == 0 {
		fmt.Println("No content found")
	}

	return nil
}

// GetRepository returns the current repository instance
func GetRepository() (*storage.DAGStore, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository not initialized")
	}
	return repo, nil
}
