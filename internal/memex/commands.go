package memex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GetFilesFromCommit extracts filenames from commit content
func GetFilesFromCommit(content string) map[string]struct{} {
	files := make(map[string]struct{})
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "--- ") && strings.HasSuffix(line, " ---") {
			filename := strings.TrimPrefix(line, "--- ")
			filename = strings.TrimSuffix(filename, " ---")
			files[filename] = struct{}{}
		}
	}
	return files
}

// InitCommand initializes a new memex repository
func InitCommand(dir string) error {
	// Convert to absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("getting absolute path: %w", err)
	}

	// Create notes directory
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return fmt.Errorf("creating notes directory: %w", err)
	}

	// Initialize repository
	repo := NewRepository(absDir)
	if err := repo.Initialize(); err != nil {
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
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("no memex directory configured, run 'memex init <directory>' first")
	}

	// Check if source file exists
	srcFile, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer srcFile.Close()

	// Get the base filename
	filename := filepath.Base(path)

	// Create destination path with timestamp prefix
	timestamp := time.Now().Unix() % 100000
	timeStr := time.Now().Format("1504") // HHMM format
	destPath := filepath.Join(config.NotesDirectory, fmt.Sprintf("%d_%s_%s", timestamp, timeStr, filename))

	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating destination file: %w", err)
	}
	defer destFile.Close()

	// Copy the file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading source file: %w", err)
	}

	if _, err := destFile.Write(data); err != nil {
		return fmt.Errorf("writing to destination file: %w", err)
	}

	fmt.Printf("Added %s to memex\n", filename)
	return nil
}

// EditCommand opens the editor for creating a new note
func EditCommand() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("no memex directory configured, run 'memex init <directory>' first")
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

	// Use shorter timestamp (last 5 digits) plus hour/minute for uniqueness
	timestamp := time.Now().Unix() % 100000
	timeStr := time.Now().Format("1504") // HHMM format
	filename := filepath.Join(config.NotesDirectory, fmt.Sprintf("%d_%s", timestamp, timeStr))

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing to file: %v", err)
	}

	fmt.Printf("\nSaved to %s\n", filename)
	return nil
}

// StatusCommand shows current repository status
func StatusCommand() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("no memex directory configured, run 'memex init <directory>' first")
	}

	repo := NewRepository(config.NotesDirectory)

	// Get last commit
	commits, err := repo.GetCommits()
	if err != nil {
		return fmt.Errorf("getting commits: %w", err)
	}

	// Print repository status
	fmt.Println("=== Memex Status ===")

	var committedFiles map[string]struct{}

	// Show last commit if exists and get its files
	if len(commits) > 0 {
		lastCommit := commits[len(commits)-1]
		fmt.Printf("\nLast commit: %s\n", lastCommit.Hash[:8])
		fmt.Printf("Message: %s\n", lastCommit.Message)
		fmt.Printf("Date: %s\n", lastCommit.Timestamp.Format(time.RFC822))

		// Get committed content
		content, err := repo.RestoreCommit(lastCommit.Hash)
		if err != nil {
			return fmt.Errorf("getting last commit content: %w", err)
		}
		committedFiles = GetFilesFromCommit(string(content))
	} else {
		fmt.Println("\nNo commits yet")
		committedFiles = make(map[string]struct{})
	}

	// List current files
	files, err := os.ReadDir(config.NotesDirectory)
	if err != nil {
		return fmt.Errorf("reading notes directory: %w", err)
	}

	var uncommittedFiles []string
	for _, file := range files {
		if !file.IsDir() && file.Name() != ".memex" {
			// Check if file was in last commit
			if _, exists := committedFiles[file.Name()]; !exists {
				uncommittedFiles = append(uncommittedFiles, file.Name())
			}
		}
	}

	if len(uncommittedFiles) > 0 {
		fmt.Println("\nUncommitted files:")
		for _, filename := range uncommittedFiles {
			info, err := os.Stat(filepath.Join(config.NotesDirectory, filename))
			if err != nil {
				continue
			}
			fmt.Printf("  %s (%s)\n", filename, info.ModTime().Format(time.RFC822))
		}
		fmt.Printf("\nTotal uncommitted files: %d\n", len(uncommittedFiles))
	} else {
		fmt.Println("\nNo uncommitted files")
	}

	return nil
}

// CommitCommand creates a new commit with the current state
func CommitCommand(message string) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("no memex directory configured, run 'memex init <directory>' first")
	}

	repo := NewRepository(config.NotesDirectory)

	// Read all files in notes directory
	files, err := os.ReadDir(config.NotesDirectory)
	if err != nil {
		return fmt.Errorf("reading notes directory: %w", err)
	}

	// Combine all files into one content block for the commit
	var content string
	for _, file := range files {
		if file.IsDir() || file.Name() == ".memex" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(config.NotesDirectory, file.Name()))
		if err != nil {
			return fmt.Errorf("reading file %s: %w", file.Name(), err)
		}
		content += fmt.Sprintf("--- %s ---\n", file.Name())
		content += string(data)
		content += "\n\n"
	}

	if err := repo.CreateCommit([]byte(content), message); err != nil {
		return fmt.Errorf("creating commit: %w", err)
	}

	fmt.Println("Created commit:", message)
	return nil
}

// LogCommand displays the commit history
func LogCommand() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("no memex directory configured, run 'memex init <directory>' first")
	}

	repo := NewRepository(config.NotesDirectory)
	commits, err := repo.GetCommits()
	if err != nil {
		return fmt.Errorf("getting commits: %w", err)
	}

	if len(commits) == 0 {
		fmt.Println("No commits yet")
		return nil
	}

	fmt.Println("Commit history:")
	for _, commit := range commits {
		fmt.Printf("Hash: %s\n", commit.Hash[:8])
		fmt.Printf("Date: %s\n", commit.Timestamp.Format(time.RFC822))
		fmt.Printf("Message: %s\n", commit.Message)
		fmt.Println("---")
	}

	return nil
}

// RestoreCommand restores content from a specific commit
func RestoreCommand(hash string) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("no memex directory configured, run 'memex init <directory>' first")
	}

	repo := NewRepository(config.NotesDirectory)
	content, err := repo.RestoreCommit(hash)
	if err != nil {
		return fmt.Errorf("restoring commit: %w", err)
	}

	fmt.Println("Restored content:")
	fmt.Println(string(content))
	return nil
}
