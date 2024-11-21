package test

import (
	"fmt"
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

	// Test finding file
	var found bool
	for _, entry := range repo.Nodes() {
		node, err := repo.GetNode(fmt.Sprintf("%x", entry.ID[:]))
		if err != nil {
			continue
		}
		if filename, ok := node.Meta["filename"].(string); ok && filename == "test.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find test.txt, but didn't")
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

	// Test finding second file
	found = false
	for _, entry := range repo.Nodes() {
		node, err := repo.GetNode(fmt.Sprintf("%x", entry.ID[:]))
		if err != nil {
			continue
		}
		if filename, ok := node.Meta["filename"].(string); ok && filename == "test2.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find test2.txt, but didn't")
	}

	// Test counting files
	fileCount := 0
	for _, entry := range repo.Nodes() {
		node, err := repo.GetNode(fmt.Sprintf("%x", entry.ID[:]))
		if err != nil {
			continue
		}
		if node.Type == "file" {
			fileCount++
		}
	}
	if fileCount != 2 {
		t.Errorf("Expected 2 files, got %d", fileCount)
	}
}
