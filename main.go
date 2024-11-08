package main

import (
	"encoding/json"
	"flag"
	"fmt"
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

func main() {
	// Subcommands
	initCmd := flag.NewFlagSet("init", flag.ExitOnError)

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

	default:
		// Unknown command, default to edit
		if err := editCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}
