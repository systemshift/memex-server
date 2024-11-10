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
		AssertFileExists(t, filepath.Join(notesDir, ".memex", "commits"))

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

		// Check if file was copied to notes directory
		config, _ := memex.LoadConfig()
		files, err := os.ReadDir(config.NotesDirectory)
		if err != nil {
			t.Fatalf("Reading notes directory failed: %v", err)
		}

		found := false
		for _, file := range files {
			if filepath.Ext(file.Name()) == ".txt" {
				found = true
				// Verify content
				content, err := os.ReadFile(filepath.Join(config.NotesDirectory, file.Name()))
				if err != nil {
					t.Fatalf("Reading added file failed: %v", err)
				}
				if string(content) != "test content" {
					t.Errorf("Added file content mismatch.\nGot: %q\nWant: %q",
						string(content), "test content")
				}
				break
			}
		}
		if !found {
			t.Error("Added file not found in notes directory")
		}
	})

	t.Run("Commit and Log Commands", func(t *testing.T) {
		// Create a test note
		config, _ := memex.LoadConfig()
		testNote := filepath.Join(config.NotesDirectory, "test_note")
		os.WriteFile(testNote, []byte("test note content"), 0644)

		// Create a commit
		message := "Test commit"
		err := memex.CommitCommand(message)
		if err != nil {
			t.Fatalf("CommitCommand failed: %v", err)
		}

		// Check log
		err = memex.LogCommand()
		if err != nil {
			t.Fatalf("LogCommand failed: %v", err)
		}

		// Verify commit exists
		repo := memex.NewRepository(config.NotesDirectory)
		commits, err := repo.GetCommits()
		if err != nil {
			t.Fatalf("GetCommits failed: %v", err)
		}

		if len(commits) == 0 {
			t.Fatal("No commits found after CommitCommand")
		}

		lastCommit := commits[len(commits)-1]
		if lastCommit.Message != message {
			t.Errorf("Commit message mismatch.\nGot: %q\nWant: %q",
				lastCommit.Message, message)
		}
	})

	t.Run("Restore Command", func(t *testing.T) {
		config, _ := memex.LoadConfig()
		repo := memex.NewRepository(config.NotesDirectory)
		commits, _ := repo.GetCommits()
		lastCommit := commits[len(commits)-1]

		// Test restore
		err := memex.RestoreCommand(lastCommit.Hash)
		if err != nil {
			t.Fatalf("RestoreCommand failed: %v", err)
		}
	})
}
