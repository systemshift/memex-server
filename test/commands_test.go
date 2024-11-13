package test

import (
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex"
)

func TestCommands(t *testing.T) {
	// Create temporary directory for testing
	tmpDir := CreateTempDir(t)

	// Set up home directory for config
	originalHome := os.Getenv("HOME")
	tmpHome := CreateTempDir(t)
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	t.Run("Init Command", func(t *testing.T) {
		notesDir := filepath.Join(tmpDir, "notes")

		// Run init command
		err := memex.InitCommand(notesDir)
		if err != nil {
			t.Fatalf("InitCommand failed: %v", err)
		}

		// Check if directories were created
		AssertFileExists(t, notesDir)
		AssertFileExists(t, filepath.Join(notesDir, ".memex", "objects"))
		AssertFileExists(t, filepath.Join(notesDir, ".memex", "meta"))
		AssertFileExists(t, filepath.Join(notesDir, ".memex", "links"))

		// Check if config was created
		config, err := memex.LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}
		if config.NotesDirectory != notesDir {
			t.Errorf("Config notes directory mismatch.\nGot: %q\nWant: %q",
				config.NotesDirectory, notesDir)
		}
	})

	t.Run("Add Command", func(t *testing.T) {
		// Create a test file to add
		testFile := CreateTestFile(t, tmpDir, "test.txt", "test content")

		// Run add command
		err := memex.AddCommand(testFile)
		if err != nil {
			t.Fatalf("AddCommand failed: %v", err)
		}

		// Check if file was added to repository
		repo, err := memex.GetRepository()
		if err != nil {
			t.Fatalf("GetRepository failed: %v", err)
		}

		// Search for added file
		results := repo.Search(map[string]any{
			"filename": "test.txt",
		})

		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}

		if string(results[0].Content) != "test content" {
			t.Errorf("Content mismatch.\nGot: %q\nWant: %q",
				string(results[0].Content), "test content")
		}
	})

	t.Run("Status Command", func(t *testing.T) {
		err := memex.StatusCommand()
		if err != nil {
			t.Errorf("StatusCommand failed: %v", err)
		}
	})

	t.Run("Show Command", func(t *testing.T) {
		// Create test content
		repo, err := memex.GetRepository()
		if err != nil {
			t.Fatalf("GetRepository failed: %v", err)
		}

		// Add test object
		id, err := repo.Add([]byte("test content"), "note", map[string]any{
			"title": "Test Note",
			"tags":  []string{"test"},
		})
		if err != nil {
			t.Fatalf("Adding test content failed: %v", err)
		}

		// Test show command
		err = memex.ShowCommand(id)
		if err != nil {
			t.Errorf("ShowCommand failed: %v", err)
		}
	})

	t.Run("Link Command", func(t *testing.T) {
		// Create two test objects
		repo, err := memex.GetRepository()
		if err != nil {
			t.Fatalf("GetRepository failed: %v", err)
		}

		id1, err := repo.Add([]byte("content 1"), "note", nil)
		if err != nil {
			t.Fatalf("Adding first test object failed: %v", err)
		}

		id2, err := repo.Add([]byte("content 2"), "note", nil)
		if err != nil {
			t.Fatalf("Adding second test object failed: %v", err)
		}

		// Test linking
		err = memex.LinkCommand(id1, id2, "references", "test link")
		if err != nil {
			t.Errorf("LinkCommand failed: %v", err)
		}

		// Verify link exists
		links, err := repo.GetLinks(id1)
		if err != nil {
			t.Fatalf("GetLinks failed: %v", err)
		}

		if len(links) != 1 {
			t.Errorf("Expected 1 link, got %d", len(links))
		}
	})

	t.Run("Search Command", func(t *testing.T) {
		// Add test objects with metadata
		repo, err := memex.GetRepository()
		if err != nil {
			t.Fatalf("GetRepository failed: %v", err)
		}

		_, err = repo.Add([]byte("content"), "note", map[string]any{
			"tags": []string{"test"},
		})
		if err != nil {
			t.Fatalf("Adding first test object failed: %v", err)
		}

		_, err = repo.Add([]byte("content"), "note", map[string]any{
			"tags": []string{"other"},
		})
		if err != nil {
			t.Fatalf("Adding second test object failed: %v", err)
		}

		// Test search
		query := map[string]any{
			"tags": []string{"test"},
		}
		err = memex.SearchCommand(query)
		if err != nil {
			t.Errorf("SearchCommand failed: %v", err)
		}
	})
}
