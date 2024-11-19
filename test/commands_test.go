package test

import (
	"os"
	"testing"

	"memex/internal/memex"
)

func TestCommands(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "memex-test-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize repository
	if err := memex.InitCommand(tmpDir); err != nil {
		t.Fatalf("Error initializing repository: %v", err)
	}

	// Create test file
	testFile := tmpDir + "/test.txt"
	content := []byte("Test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Error creating test file: %v", err)
	}

	// Test add command
	if err := memex.AddCommand(testFile); err != nil {
		t.Fatalf("Error adding file: %v", err)
	}

	// Get repository
	repo, err := memex.GetRepository()
	if err != nil {
		t.Fatalf("Error getting repository: %v", err)
	}

	// Test search
	nodes, err := repo.Search(map[string]any{
		"filename": "test.txt",
	})
	if err != nil {
		t.Fatalf("Error searching: %v", err)
	}
	if len(nodes) != 1 {
		t.Errorf("Expected 1 search result, got %d", len(nodes))
	}

	// Test status command
	if err := memex.StatusCommand(); err != nil {
		t.Fatalf("Error showing status: %v", err)
	}

	// Create another test file
	testFile2 := tmpDir + "/test2.txt"
	content2 := []byte("Another test content")
	if err := os.WriteFile(testFile2, content2, 0644); err != nil {
		t.Fatalf("Error creating second test file: %v", err)
	}

	// Add second file
	if err := memex.AddCommand(testFile2); err != nil {
		t.Fatalf("Error adding second file: %v", err)
	}

	// Test search again
	nodes, err = repo.Search(map[string]any{
		"filename": "test2.txt",
	})
	if err != nil {
		t.Fatalf("Error searching: %v", err)
	}
	if len(nodes) != 1 {
		t.Errorf("Expected 1 search result, got %d", len(nodes))
	}

	// Test finding by type
	files, err := repo.FindByType("file")
	if err != nil {
		t.Fatalf("Error finding by type: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}
}
