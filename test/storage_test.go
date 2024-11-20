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
	repo, err := storage.CreateRepository(repoPath, "test")
	if err != nil {
		t.Fatalf("Error creating repository: %v", err)
	}

	// Test storing content
	content := []byte("Test content")
	hash, err := repo.GetChunkStore().Store(content)
	if err != nil {
		t.Fatalf("Error storing content: %v", err)
	}

	// Test retrieving content
	retrieved, err := repo.GetChunkStore().Load(hash)
	if err != nil {
		t.Fatalf("Error loading content: %v", err)
	}

	if !bytes.Equal(retrieved, content) {
		t.Error("Content not preserved correctly")
	}

	// Test content exists
	if !repo.GetChunkStore().Has(hash) {
		t.Error("Content should exist")
	}

	// Test deleting content
	if err := repo.GetChunkStore().Delete(hash); err != nil {
		t.Fatalf("Error deleting content: %v", err)
	}

	// Verify content is deleted
	if repo.GetChunkStore().Has(hash) {
		t.Error("Content should be deleted")
	}

	// Test storing duplicate content
	hash1, err := repo.GetChunkStore().Store(content)
	if err != nil {
		t.Fatalf("Error storing content first time: %v", err)
	}

	hash2, err := repo.GetChunkStore().Store(content)
	if err != nil {
		t.Fatalf("Error storing content second time: %v", err)
	}

	if hash1 != hash2 {
		t.Error("Duplicate content should have same hash")
	}

	// Test content deduplication
	if !repo.GetChunkStore().Has(hash1) {
		t.Error("First copy should exist")
	}

	if err := repo.GetChunkStore().Delete(hash1); err != nil {
		t.Fatalf("Error deleting first copy: %v", err)
	}

	if !repo.GetChunkStore().Has(hash2) {
		t.Error("Second copy should still exist")
	}

	if err := repo.GetChunkStore().Delete(hash2); err != nil {
		t.Fatalf("Error deleting second copy: %v", err)
	}

	if repo.GetChunkStore().Has(hash2) {
		t.Error("Content should be deleted after both copies removed")
	}
}
