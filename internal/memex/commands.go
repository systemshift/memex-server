package memex

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"memex/internal/memex/storage"
)

var repo *storage.MXStore

// GetConnectedRepo returns the path of the currently connected repository
func GetConnectedRepo() string {
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

// SaveConnectedRepo saves the path of the connected repository
func SaveConnectedRepo(path string) error {
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
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if _, err := os.Stat(absPath); err == nil {
		return fmt.Errorf("repository already exists at %s", absPath)
	}

	repo, err = storage.CreateMX(absPath)
	if err != nil {
		return fmt.Errorf("creating repository: %w", err)
	}

	if err := SaveConnectedRepo(absPath); err != nil {
		return fmt.Errorf("connecting to new repo: %w", err)
	}

	fmt.Printf("Created repository %s\n", absPath)
	return nil
}

// ConnectCommand connects to an existing repository
func ConnectCommand(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	if err := SaveConnectedRepo(absPath); err != nil {
		return fmt.Errorf("saving connection: %w", err)
	}

	fmt.Printf("Connected to %s\n", absPath)
	return nil
}

// OpenRepository opens and connects to a repository
func OpenRepository() error {
	repoPath := GetConnectedRepo()
	if repoPath == "" {
		return fmt.Errorf("no repository connected. Use 'init <name>' or 'connect <path>' first")
	}

	// Check if the connected repo exists
	if _, err := os.Stat(repoPath); err != nil {
		return fmt.Errorf("connected repository '%s' not found", repoPath)
	}

	var err error
	repo, err = storage.OpenMX(repoPath)
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}

	return nil
}

// GetRepository returns the current repository instance
func GetRepository() (*storage.MXStore, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository not initialized")
	}
	return repo, nil
}

// EditCommand opens the editor
func EditCommand() error {
	editor := NewEditor(repo.Path())
	content, err := editor.Run()
	if err != nil {
		return fmt.Errorf("running editor: %w", err)
	}

	if content == "" {
		return nil // User cancelled
	}

	meta := map[string]any{
		"added":   time.Now().Format(time.RFC3339),
		"content": content,
	}

	id, err := repo.AddNode([]byte(content), "note", meta)
	if err != nil {
		return fmt.Errorf("adding note: %w", err)
	}

	fmt.Printf("Added note (ID: %s)\n", id[:8])
	return nil
}

// AddCommand adds a file to the repository
func AddCommand(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("checking path: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("'%s' is a directory. Use 'add <file>' to add individual files", path)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	meta := map[string]any{
		"filename": filepath.Base(absPath),
		"added":    time.Now().Format(time.RFC3339),
	}

	id, err := repo.AddNode(content, "file", meta)
	if err != nil {
		return fmt.Errorf("adding to repository: %w", err)
	}

	fmt.Printf("Added %s (ID: %s)\n", filepath.Base(absPath), id[:8])
	return nil
}

// DeleteCommand deletes an object from the repository
func DeleteCommand(id string) error {
	node, err := repo.GetNode(id)
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

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
	node, err := repo.GetNode(id)
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

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

// CloseRepository closes the current repository
func CloseRepository() error {
	if repo != nil {
		return repo.Close()
	}
	return nil
}
