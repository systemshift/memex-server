package test

import (
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex"
)

func TestConfig(t *testing.T) {
	// Create a temporary home directory for testing
	originalHome := os.Getenv("HOME")
	tmpHome := CreateTempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	t.Run("Save and Load Config", func(t *testing.T) {
		// Create test config
		config := &memex.Config{
			NotesDirectory: "/test/notes/dir",
		}

		// Save config
		if err := memex.SaveConfig(config); err != nil {
			t.Fatalf("SaveConfig failed: %v", err)
		}

		// Check if config file exists
		configPath := filepath.Join(tmpHome, ".config", "memex", "config.json")
		AssertFileExists(t, configPath)

		// Load config
		loaded, err := memex.LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		// Verify loaded config
		if loaded.NotesDirectory != config.NotesDirectory {
			t.Errorf("Config mismatch.\nGot: %q\nWant: %q",
				loaded.NotesDirectory, config.NotesDirectory)
		}
	})

	t.Run("Config File Not Found", func(t *testing.T) {
		// Remove config directory
		configDir := filepath.Join(tmpHome, ".config", "memex")
		os.RemoveAll(configDir)

		// Try to load config
		_, err := memex.LoadConfig()
		if err == nil {
			t.Error("Expected error when config file doesn't exist")
		}
	})

	t.Run("Invalid Config JSON", func(t *testing.T) {
		// Create invalid config file
		configPath := filepath.Join(tmpHome, ".config", "memex", "config.json")
		err := os.MkdirAll(filepath.Dir(configPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		err = os.WriteFile(configPath, []byte("invalid json"), 0644)
		if err != nil {
			t.Fatalf("Failed to write invalid config: %v", err)
		}

		// Try to load config
		_, err = memex.LoadConfig()
		if err == nil {
			t.Error("Expected error when loading invalid config")
		}
	})
}
