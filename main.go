package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	NotesDirectory string `json:"notes_directory"`
}

func getConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting home directory: %v\n", err)
		os.Exit(1)
	}
	return filepath.Join(homeDir, ".config", "memex", "config.json")
}

func loadConfig() (*Config, error) {
	configPath := getConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func saveConfig(config *Config) error {
	configPath := getConfigPath()
	configDir := filepath.Dir(configPath)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

func initCommand(dir string) error {
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
	if err := saveConfig(config); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Initialized memex in %s\n", absDir)
	return nil
}

func addCommand(path string) error {
	config, err := loadConfig()
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
	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}

	fmt.Printf("Added %s to memex\n", filename)
	return nil
}

func editCommand() error {
	config, err := loadConfig()
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

func commitCommand(message string) error {
	config, err := loadConfig()
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

func logCommand() error {
	config, err := loadConfig()
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

func restoreCommand(hash string) error {
	config, err := loadConfig()
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

func main() {
	// Subcommands
	initCmd := flag.NewFlagSet("init", flag.ExitOnError)
	commitCmd := flag.NewFlagSet("commit", flag.ExitOnError)
	commitMsg := commitCmd.String("m", "", "Commit message")
	restoreCmd := flag.NewFlagSet("restore", flag.ExitOnError)
	addCmd := flag.NewFlagSet("add", flag.ExitOnError)

	// Parse command
	if len(os.Args) < 2 {
		// No command provided, default to edit
		if err := editCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	switch os.Args[1] {
	case "init":
		initCmd.Parse(os.Args[2:])
		if initCmd.NArg() != 1 {
			fmt.Fprintf(os.Stderr, "Usage: memex init <directory>\n")
			os.Exit(1)
		}
		if err := initCommand(initCmd.Arg(0)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "add":
		addCmd.Parse(os.Args[2:])
		if addCmd.NArg() != 1 {
			fmt.Fprintf(os.Stderr, "Usage: memex add <file>\n")
			os.Exit(1)
		}
		if err := addCommand(addCmd.Arg(0)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "commit":
		commitCmd.Parse(os.Args[2:])
		if *commitMsg == "" {
			fmt.Fprintf(os.Stderr, "Usage: memex commit -m \"commit message\"\n")
			os.Exit(1)
		}
		if err := commitCommand(*commitMsg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "log":
		if err := logCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "restore":
		restoreCmd.Parse(os.Args[2:])
		if restoreCmd.NArg() != 1 {
			fmt.Fprintf(os.Stderr, "Usage: memex restore <commit-hash>\n")
			os.Exit(1)
		}
		if err := restoreCommand(restoreCmd.Arg(0)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	default:
		// Unknown command, default to edit
		if err := editCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}
