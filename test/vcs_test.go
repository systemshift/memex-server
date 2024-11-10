package test

import (
	"path/filepath"
	"testing"
	"time"

	"memex/internal/memex"
)

// TestRepository tests the core functionality of the version control system
func TestRepository(t *testing.T) {
	tmpDir := CreateTempDir(t)

	// Initialize repository
	t.Run("Initialize Repository", func(t *testing.T) {
		repo := memex.NewRepository(tmpDir)
		if err := repo.Initialize(); err != nil {
			t.Errorf("Initialize failed: %v", err)
		}

		// Check if required directories were created
		dirs := []string{
			filepath.Join(tmpDir, ".memex", "objects"),
			filepath.Join(tmpDir, ".memex", "commits"),
		}
		for _, dir := range dirs {
			AssertFileExists(t, dir)
		}
	})

	// Test content storage
	t.Run("Store and Retrieve Content", func(t *testing.T) {
		repo := memex.NewRepository(tmpDir)
		content := []byte("test content")

		// Store content
		hash, err := repo.StoreObject(content)
		if err != nil {
			t.Fatalf("StoreObject failed: %v", err)
		}

		// Check if object file exists
		objPath := filepath.Join(tmpDir, ".memex", "objects", hash[:2], hash[2:])
		AssertFileExists(t, objPath)
		AssertFileContent(t, objPath, string(content))
	})

	// Test commit functionality
	t.Run("Create and List Commits", func(t *testing.T) {
		repo := memex.NewRepository(tmpDir)
		content := []byte("test commit content")
		message := "test commit message"

		// Create commit
		if err := repo.CreateCommit(content, message); err != nil {
			t.Fatalf("CreateCommit failed: %v", err)
		}

		// Get commits
		commits, err := repo.GetCommits()
		if err != nil {
			t.Fatalf("GetCommits failed: %v", err)
		}

		// Verify commit
		if len(commits) == 0 {
			t.Fatal("No commits found")
		}

		lastCommit := commits[len(commits)-1]
		if lastCommit.Message != message {
			t.Errorf("Commit message mismatch.\nGot: %q\nWant: %q", lastCommit.Message, message)
		}

		// Verify commit timestamp
		if time.Since(lastCommit.Timestamp) > time.Minute {
			t.Errorf("Commit timestamp too old: %v", lastCommit.Timestamp)
		}
	})

	// Test restore functionality
	t.Run("Restore Commit", func(t *testing.T) {
		repo := memex.NewRepository(tmpDir)
		content := []byte("test restore content")
		message := "test restore commit"

		// Create a commit to restore
		if err := repo.CreateCommit(content, message); err != nil {
			t.Fatalf("CreateCommit failed: %v", err)
		}

		// Get the commit hash
		commits, err := repo.GetCommits()
		if err != nil {
			t.Fatalf("GetCommits failed: %v", err)
		}

		lastCommit := commits[len(commits)-1]

		// Restore the commit
		restored, err := repo.RestoreCommit(lastCommit.Hash)
		if err != nil {
			t.Fatalf("RestoreCommit failed: %v", err)
		}

		// Verify restored content
		if string(restored) != string(content) {
			t.Errorf("Restored content mismatch.\nGot: %q\nWant: %q", restored, content)
		}
	})
}

// TestHashContent tests the content hashing functionality
func TestHashContent(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    string // First few chars of expected hash
	}{
		{
			name:    "Empty content",
			content: []byte(""),
			want:    "e3b0c4", // First 6 chars of SHA256 of empty string
		},
		{
			name:    "Simple content",
			content: []byte("test"),
			want:    "9f86d0", // First 6 chars of SHA256 of "test"
		},
		{
			name:    "Multi-line content",
			content: []byte("line1\nline2\n"),
			want:    "2751a3", // First 6 chars of SHA256
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := memex.HashContent(tt.content)
			if got[:6] != tt.want {
				t.Errorf("hashContent() = %v, want %v", got[:6], tt.want)
			}
		})
	}
}
