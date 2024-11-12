package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memex/internal/memex"
	"memex/internal/memex/mx"
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
			if strings.HasSuffix(file.Name(), ".mx") {
				found = true
				// Verify content
				content, err := os.ReadFile(filepath.Join(config.NotesDirectory, file.Name()))
				if err != nil {
					t.Fatalf("Reading added file failed: %v", err)
				}
				if !strings.Contains(string(content), "test content") {
					t.Errorf("Added file content mismatch.\nGot: %q\nWant to contain: %q",
						string(content), "test content")
				}
				break
			}
		}
		if !found {
			t.Error("Added file not found in notes directory")
		}
	})

	t.Run("Status Command Tests", func(t *testing.T) {
		config, _ := memex.LoadConfig()

		// Test: Status with no files
		t.Run("Empty Repository", func(t *testing.T) {
			// Clear directory
			files, _ := os.ReadDir(config.NotesDirectory)
			for _, file := range files {
				if !file.IsDir() && file.Name() != ".memex" {
					os.Remove(filepath.Join(config.NotesDirectory, file.Name()))
				}
			}

			err := memex.StatusCommand()
			if err != nil {
				t.Errorf("StatusCommand failed: %v", err)
			}
		})

		// Test: Status with uncommitted files
		t.Run("Uncommitted Files", func(t *testing.T) {
			// Create test files
			file1 := CreateTestFile(t, config.NotesDirectory, "note1", "content 1")
			file2 := CreateTestFile(t, config.NotesDirectory, "note2", "content 2")

			err := memex.StatusCommand()
			if err != nil {
				t.Errorf("StatusCommand failed: %v", err)
			}

			// Verify files exist
			AssertFileExists(t, file1)
			AssertFileExists(t, file2)
		})

		// Test: Status after commit
		t.Run("After Commit", func(t *testing.T) {
			// Commit current files
			err := memex.CommitCommand("Test commit")
			if err != nil {
				t.Fatalf("CommitCommand failed: %v", err)
			}

			// Check status
			err = memex.StatusCommand()
			if err != nil {
				t.Errorf("StatusCommand failed: %v", err)
			}

			// Add new file after commit
			CreateTestFile(t, config.NotesDirectory, "note3", "content 3")

			// Check status again - should show only the new file
			err = memex.StatusCommand()
			if err != nil {
				t.Errorf("StatusCommand failed: %v", err)
			}
		})

		// Test: Status with mixed committed/uncommitted files
		t.Run("Mixed Files", func(t *testing.T) {
			// Create more test files
			CreateTestFile(t, config.NotesDirectory, "committed1", "content")
			CreateTestFile(t, config.NotesDirectory, "committed2", "content")

			// Commit these files
			err := memex.CommitCommand("Commit some files")
			if err != nil {
				t.Fatalf("CommitCommand failed: %v", err)
			}

			// Create more files without committing
			CreateTestFile(t, config.NotesDirectory, "uncommitted1", "content")
			CreateTestFile(t, config.NotesDirectory, "uncommitted2", "content")

			// Check status - should only show uncommitted files
			err = memex.StatusCommand()
			if err != nil {
				t.Errorf("StatusCommand failed: %v", err)
			}
		})
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

// TestGetFilesFromCommit tests the helper function that extracts filenames from commit content
func TestGetFilesFromCommit(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "Empty content",
			content:  "",
			expected: []string{},
		},
		{
			name:     "Single file",
			content:  "--- file1.txt ---\ncontent\n",
			expected: []string{"file1.txt"},
		},
		{
			name:     "Multiple files",
			content:  "--- file1.txt ---\ncontent1\n\n--- file2.txt ---\ncontent2",
			expected: []string{"file1.txt", "file2.txt"},
		},
		{
			name:     "With timestamps",
			content:  "--- 12345_1234_file.txt ---\ncontent\n",
			expected: []string{"12345_1234_file.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := mx.GetFilesFromCommit(tt.content)

			// Convert map to slice for comparison
			var got []string
			for file := range files {
				got = append(got, file)
			}

			// Check if all expected files are present
			for _, want := range tt.expected {
				found := false
				for _, g := range got {
					if g == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Missing expected file %q", want)
				}
			}

			// Check if there are any unexpected files
			if len(got) != len(tt.expected) {
				t.Errorf("Got %d files, want %d", len(got), len(tt.expected))
			}
		})
	}
}
