package test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex/storage"
)

func TestStorage(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "memex-test-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test repository
	repoPath := filepath.Join(tmpDir, "test.mx")
	repo, err := storage.CreateMX(repoPath)
	if err != nil {
		t.Fatalf("Error creating repository: %v", err)
	}

	// Test storing content
	content := []byte("Test content")
	meta := map[string]any{
		"filename": "test.txt",
		"added":    "2024-01-01T00:00:00Z",
	}
	id, err := repo.AddNode(content, "file", meta)
	if err != nil {
		t.Fatalf("Error storing content: %v", err)
	}

	// Test retrieving content
	node, err := repo.GetNode(id)
	if err != nil {
		t.Fatalf("Error loading content: %v", err)
	}

	// Get content from blob store
	contentHash := node.Meta["content"].(string)
	blob, err := repo.LoadBlob(contentHash)
	if err != nil {
		t.Fatalf("Error loading blob: %v", err)
	}

	if !bytes.Equal(content, blob) {
		t.Error("Content not preserved correctly")
	}

	// Test content exists
	_, err = repo.GetNode(id)
	if err != nil {
		t.Error("Content should exist")
	}

	// Test deleting content
	if err := repo.DeleteNode(id); err != nil {
		t.Fatalf("Error deleting content: %v", err)
	}

	// Verify content is deleted
	_, err = repo.GetNode(id)
	if err == nil {
		t.Error("Content should be deleted")
	}

	// Test storing duplicate content
	id1, err := repo.AddNode(content, "file", meta)
	if err != nil {
		t.Fatalf("Error storing content first time: %v", err)
	}

	id2, err := repo.AddNode(content, "file", meta)
	if err != nil {
		t.Fatalf("Error storing content second time: %v", err)
	}

	// Test content exists
	_, err = repo.GetNode(id1)
	if err != nil {
		t.Error("First copy should exist")
	}

	if err := repo.DeleteNode(id1); err != nil {
		t.Fatalf("Error deleting first copy: %v", err)
	}

	_, err = repo.GetNode(id2)
	if err != nil {
		t.Error("Second copy should still exist")
	}

	if err := repo.DeleteNode(id2); err != nil {
		t.Fatalf("Error deleting second copy: %v", err)
	}

	_, err = repo.GetNode(id2)
	if err == nil {
		t.Error("Content should be deleted after both copies removed")
	}
}
