package memex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memex/internal/memex/storage"
)

var repo *storage.Repository

// GetRepository returns the current repository instance
func GetRepository() (*storage.Repository, error) {
	if repo != nil {
		return repo, nil
	}

	config, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("no memex directory configured, run 'memex init <directory>' first")
	}

	repo, err = storage.NewRepository(filepath.Join(config.NotesDirectory, ".memex"))
	if err != nil {
		return nil, fmt.Errorf("opening repository: %w", err)
	}

	return repo, nil
}

// InitCommand initializes a new memex repository
func InitCommand(dir string) error {
	// Convert to absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("getting absolute path: %w", err)
	}

	// Create repository directory
	repoDir := filepath.Join(absDir, ".memex")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return fmt.Errorf("creating repository directory: %w", err)
	}

	// Initialize repository
	repo, err = storage.NewRepository(repoDir)
	if err != nil {
		return fmt.Errorf("initializing repository: %w", err)
	}

	// Save config
	config := &Config{
		NotesDirectory: absDir,
	}
	if err := SaveConfig(config); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Initialized memex in %s\n", absDir)
	return nil
}

// AddCommand adds a file to memex
func AddCommand(path string) error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

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
	id, err := repo.Add(content, "file", meta)
	if err != nil {
		return fmt.Errorf("adding to repository: %w", err)
	}

	fmt.Printf("Added %s (ID: %s)\n", filepath.Base(path), id[:8])
	return nil
}

// EditCommand opens the editor for creating a new note
func EditCommand() error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

	// Create a new editor instance
	editor := NewEditor()

	// Run the editor and get content
	content, err := editor.Run()
	if err != nil {
		return fmt.Errorf("editor error: %v", err)
	}

	// If no content or user quit, exit
	if content == "" {
		fmt.Println("\nNo content saved")
		return nil
	}

	// Extract title from first line
	lines := strings.Split(content, "\n")
	title := "Untitled"
	if len(lines) > 0 && lines[0] != "" {
		title = lines[0]
	}

	// Create metadata
	meta := map[string]any{
		"title": title,
		"type":  "note",
	}

	// Add to repository
	id, err := repo.Add([]byte(content), "note", meta)
	if err != nil {
		return fmt.Errorf("adding to repository: %w", err)
	}

	fmt.Printf("\nSaved note (ID: %s)\n", id[:8])
	return nil
}

// StatusCommand shows current repository status
func StatusCommand() error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

	fmt.Println("Memex Status ===\n")

	// List notes
	notes := repo.FindByType("note")
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
	files := repo.FindByType("file")
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

// ShowCommand displays an object's content
func ShowCommand(id string) error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

	obj, err := repo.Get(id)
	if err != nil {
		return fmt.Errorf("getting object: %w", err)
	}

	// Print basic info
	fmt.Printf("ID: %s\n", obj.ID)
	fmt.Printf("Type: %s\n", obj.Type)
	fmt.Printf("Created: %s\n", obj.Created.UTC().Format("02 Jan 06 15:04 MST"))
	fmt.Printf("Modified: %s\n", obj.Modified.UTC().Format("02 Jan 06 15:04 MST"))

	// Print metadata
	fmt.Println("\nMetadata:")
	for k, v := range obj.Meta {
		fmt.Printf("  %s: %v\n", k, v)
	}

	// Print content for text-based objects only
	if obj.Type == "note" {
		fmt.Printf("\nContent:\n%s\n", string(obj.Content))
	} else {
		fmt.Printf("\nContent: [Binary data - %d bytes]\n", len(obj.Content))
	}

	return nil
}

// LinkCommand creates a link between two objects
func LinkCommand(sourceID, targetID, linkType, note string) error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

	meta := map[string]any{}
	if note != "" {
		meta["note"] = note
	}

	err = repo.Link(sourceID, targetID, linkType, meta)
	if err != nil {
		return fmt.Errorf("creating link: %w", err)
	}

	fmt.Printf("Created %s link from %s to %s\n", linkType, sourceID[:8], targetID[:8])
	return nil
}

// SearchCommand searches for objects
func SearchCommand(query map[string]any) error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

	results := repo.Search(query)
	if len(results) == 0 {
		fmt.Println("No results found")
		return nil
	}

	fmt.Printf("Found %d results:\n\n", len(results))
	for _, obj := range results {
		fmt.Printf("ID: %s\n", obj.ID[:8])
		fmt.Printf("Type: %s\n", obj.Type)
		if title, ok := obj.Meta["title"].(string); ok {
			fmt.Printf("Title: %s\n", title)
		}
		fmt.Printf("Created: %s\n", obj.Created.UTC().Format("02 Jan 06 15:04 MST"))
		fmt.Printf("\n")
	}

	return nil
}

// CommitCommand creates a new version of all changes
func CommitCommand(message string) error {
	// TODO: Implement versioning
	fmt.Println("Commit functionality not yet implemented")
	return nil
}

// LogCommand shows version history
func LogCommand() error {
	// TODO: Implement version history
	fmt.Println("Log functionality not yet implemented")
	return nil
}

// RestoreCommand restores content to a specific version
func RestoreCommand(commitHash string) error {
	// TODO: Implement version restore
	fmt.Println("Restore functionality not yet implemented")
	return nil
}
