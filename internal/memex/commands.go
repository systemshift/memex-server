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

	fmt.Println("=== Memex Status ===")

	// List all objects
	objects := repo.FindByType("note")
	if len(objects) == 0 {
		fmt.Println("\nNo notes found")
		return nil
	}

	fmt.Printf("\nNotes (%d):\n", len(objects))
	for _, obj := range objects {
		title := "Untitled"
		if t, ok := obj.Meta["title"].(string); ok {
			title = t
		}
		fmt.Printf("  %s - %s (%s)\n", obj.ID[:8], title, obj.Modified.Format(time.RFC822))
	}

	return nil
}

// ShowCommand displays an object's content and metadata
func ShowCommand(id string) error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

	// Get object
	obj, err := repo.Get(id)
	if err != nil {
		return fmt.Errorf("getting object: %w", err)
	}

	// Print metadata
	fmt.Printf("ID: %s\n", obj.ID[:8])
	fmt.Printf("Type: %s\n", obj.Type)
	fmt.Printf("Created: %s\n", obj.Created.Format(time.RFC822))
	fmt.Printf("Modified: %s\n", obj.Modified.Format(time.RFC822))
	fmt.Printf("Version: %d\n", obj.Version)

	if len(obj.Meta) > 0 {
		fmt.Println("\nMetadata:")
		for k, v := range obj.Meta {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}

	// Get links
	links, err := repo.GetLinks(id)
	if err != nil {
		return fmt.Errorf("getting links: %w", err)
	}

	if len(links) > 0 {
		fmt.Println("\nLinks:")
		for _, link := range links {
			if link.Source == id {
				fmt.Printf("  -> %s (%s)\n", link.Target[:8], link.Type)
			} else {
				fmt.Printf("  <- %s (%s)\n", link.Source[:8], link.Type)
			}
		}
	}

	// Print content
	fmt.Println("\nContent:")
	fmt.Println(string(obj.Content))

	return nil
}

// LinkCommand creates a link between objects
func LinkCommand(sourceID, targetID, linkType string, note string) error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

	meta := map[string]any{}
	if note != "" {
		meta["note"] = note
	}

	if err := repo.Link(sourceID, targetID, linkType, meta); err != nil {
		return fmt.Errorf("creating link: %w", err)
	}

	fmt.Printf("Created %s link: %s -> %s\n", linkType, sourceID[:8], targetID[:8])
	return nil
}

// SearchCommand searches for objects by metadata
func SearchCommand(query map[string]any) error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

	results := repo.Search(query)
	if len(results) == 0 {
		fmt.Println("No matches found")
		return nil
	}

	fmt.Printf("Found %d matches:\n", len(results))
	for _, obj := range results {
		title := "Untitled"
		if t, ok := obj.Meta["title"].(string); ok {
			title = t
		}
		fmt.Printf("  %s - %s (%s)\n", obj.ID[:8], title, obj.Modified.Format(time.RFC822))
	}

	return nil
}

// CommitCommand creates a new commit
func CommitCommand(message string) error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

	// Get all objects
	objects := repo.List()
	if len(objects) == 0 {
		return fmt.Errorf("no objects to commit")
	}

	// Create commit object
	meta := map[string]any{
		"message": message,
		"date":    time.Now(),
	}

	// Store commit
	commitID, err := repo.Add([]byte(message), "commit", meta)
	if err != nil {
		return fmt.Errorf("creating commit: %w", err)
	}

	// Link commit to all objects
	for _, objID := range objects {
		if err := repo.Link(commitID, objID, "contains", nil); err != nil {
			return fmt.Errorf("linking commit: %w", err)
		}
	}

	fmt.Printf("Created commit: %s\n", message)
	return nil
}

// LogCommand displays commit history
func LogCommand() error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

	// Get all commits
	commits := repo.FindByType("commit")
	if len(commits) == 0 {
		fmt.Println("No commits yet")
		return nil
	}

	fmt.Println("Commit history:")
	for _, commit := range commits {
		fmt.Printf("Hash: %s\n", commit.ID[:8])
		if date, ok := commit.Meta["date"].(time.Time); ok {
			fmt.Printf("Date: %s\n", date.Format(time.RFC822))
		}
		if msg, ok := commit.Meta["message"].(string); ok {
			fmt.Printf("Message: %s\n", msg)
		}
		fmt.Println("---")
	}

	return nil
}

// RestoreCommand restores content from a specific commit
func RestoreCommand(commitID string) error {
	repo, err := GetRepository()
	if err != nil {
		return err
	}

	// Get commit
	commit, err := repo.Get(commitID)
	if err != nil {
		return fmt.Errorf("getting commit: %w", err)
	}

	if commit.Type != "commit" {
		return fmt.Errorf("object %s is not a commit", commitID[:8])
	}

	// Get linked objects
	links, err := repo.GetLinks(commitID)
	if err != nil {
		return fmt.Errorf("getting commit contents: %w", err)
	}

	fmt.Println("Restored content:")
	for _, link := range links {
		if link.Type == "contains" {
			obj, err := repo.Get(link.Target)
			if err != nil {
				continue
			}
			filename := "unknown"
			if name, ok := obj.Meta["filename"].(string); ok {
				filename = name
			}
			fmt.Printf("--- %s ---\n", filename)
			fmt.Println(string(obj.Content))
			fmt.Println()
		}
	}

	return nil
}
