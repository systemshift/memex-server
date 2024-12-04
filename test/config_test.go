package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex"
)

func TestConfig(t *testing.T) {
	// Create temporary home directory for tests
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer func() {
		os.Setenv("HOME", originalHome)
	}()

	t.Run("Config Path", func(t *testing.T) {
		path := memex.GetConfigPath()
		expected := filepath.Join(tmpHome, ".config", "memex", "config.json")
		if path != expected {
			t.Errorf("wrong config path: got %q, want %q", path, expected)
		}
	})

	t.Run("Save and Load Config", func(t *testing.T) {
		config := &memex.Config{
			NotesDirectory: "/path/to/notes",
		}

		// Save config
		if err := memex.SaveConfig(config); err != nil {
			t.Fatalf("saving config: %v", err)
		}

		// Load config
		loaded, err := memex.LoadConfig()
		if err != nil {
			t.Fatalf("loading config: %v", err)
		}

		if loaded.NotesDirectory != config.NotesDirectory {
			t.Errorf("wrong notes directory: got %q, want %q",
				loaded.NotesDirectory, config.NotesDirectory)
		}

		// Verify JSON formatting
		data, err := os.ReadFile(memex.GetConfigPath())
		if err != nil {
			t.Fatalf("reading config file: %v", err)
		}

		var formatted map[string]interface{}
		if err := json.Unmarshal(data, &formatted); err != nil {
			t.Fatalf("parsing config JSON: %v", err)
		}

		if formatted["notes_directory"] != config.NotesDirectory {
			t.Error("JSON field name mismatch")
		}
	})

	t.Run("Invalid Config File", func(t *testing.T) {
		// Write invalid JSON
		configPath := memex.GetConfigPath()
		if err := os.WriteFile(configPath, []byte("invalid json"), 0644); err != nil {
			t.Fatalf("writing invalid config: %v", err)
		}

		// Try to load config
		_, err := memex.LoadConfig()
		if err == nil {
			t.Error("expected error loading invalid config")
		}
	})

	t.Run("Missing Config File", func(t *testing.T) {
		// Remove config file
		configPath := memex.GetConfigPath()
		if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
			t.Fatalf("removing config file: %v", err)
		}

		// Try to load config
		_, err := memex.LoadConfig()
		if err == nil {
			t.Error("expected error loading missing config")
		}
	})

	t.Run("Create Config Directory", func(t *testing.T) {
		// Remove config directory
		configDir := filepath.Dir(memex.GetConfigPath())
		if err := os.RemoveAll(configDir); err != nil {
			t.Fatalf("removing config directory: %v", err)
		}

		// Save config should create directory
		config := &memex.Config{
			NotesDirectory: "/test/path",
		}
		if err := memex.SaveConfig(config); err != nil {
			t.Fatalf("saving config: %v", err)
		}

		// Verify directory was created
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			t.Error("config directory was not created")
		}
	})

	t.Run("Config Permissions", func(t *testing.T) {
		config := &memex.Config{
			NotesDirectory: "/test/permissions",
		}

		// Save config
		if err := memex.SaveConfig(config); err != nil {
			t.Fatalf("saving config: %v", err)
		}

		// Check file permissions
		info, err := os.Stat(memex.GetConfigPath())
		if err != nil {
			t.Fatalf("getting config file info: %v", err)
		}

		mode := info.Mode().Perm()
		if mode != 0644 {
			t.Errorf("wrong file permissions: got %o, want %o", mode, 0644)
		}

		// Check directory permissions
		dirInfo, err := os.Stat(filepath.Dir(memex.GetConfigPath()))
		if err != nil {
			t.Fatalf("getting config directory info: %v", err)
		}

		dirMode := dirInfo.Mode().Perm()
		if dirMode != 0755 {
			t.Errorf("wrong directory permissions: got %o, want %o", dirMode, 0755)
		}
	})
}
