package test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

func TestChunking(t *testing.T) {
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

	// Create test content larger than chunk size
	content := bytes.Repeat([]byte("This is test content that will be split into multiple chunks. "), 100)

	// Store content
	meta := map[string]any{"filename": "large.txt"}
	id, err := repo.AddNode(content, "file", meta)
	if err != nil {
		t.Fatalf("Error storing content: %v", err)
	}

	// Verify content is split into chunks
	node, err := repo.GetNode(id)
	if err != nil {
		t.Fatalf("Error getting node: %v", err)
	}

	chunks, ok := node.Meta["chunks"].([]string)
	if !ok {
		t.Fatal("Node should have chunks in metadata")
	}

	if len(chunks) <= 1 {
		t.Error("Content should be split into multiple chunks")
	}

	// Test chunk deduplication
	id2, err := repo.AddNode(content, "file", meta)
	if err != nil {
		t.Fatalf("Error storing duplicate content: %v", err)
	}

	node2, err := repo.GetNode(id2)
	if err != nil {
		t.Fatalf("Error getting second node: %v", err)
	}

	chunks2, ok := node2.Meta["chunks"].([]string)
	if !ok {
		t.Fatal("Second node should have chunks in metadata")
	}

	// Verify chunks are reused
	if !equalStringSlices(chunks, chunks2) {
		t.Error("Duplicate content should reuse same chunks")
	}
}

func TestSimilarity(t *testing.T) {
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

	// Create two similar documents
	content1 := strings.Repeat("This is the first document with some shared content. ", 100)
	content2 := strings.Repeat("This is the second document with some shared content. ", 100)

	// Store both documents
	meta := map[string]any{}
	id1, err := repo.AddNode([]byte(content1), "file", meta)
	if err != nil {
		t.Fatalf("Error storing first document: %v", err)
	}

	id2, err := repo.AddNode([]byte(content2), "file", meta)
	if err != nil {
		t.Fatalf("Error storing second document: %v", err)
	}

	// Get links for first document
	links, err := repo.GetLinks(id1)
	if err != nil {
		t.Fatalf("Error getting links: %v", err)
	}

	// Verify similarity link exists
	found := false
	for _, link := range links {
		if link.Target == id2 && link.Type == "similar" {
			found = true
			// Verify similarity metadata
			similarity, ok := link.Meta["similarity"].(float64)
			if !ok {
				t.Error("Similarity link should have similarity score")
			}
			if similarity < 0.3 {
				t.Error("Documents should have significant similarity")
			}
			shared, ok := link.Meta["shared"].(int)
			if !ok {
				t.Error("Similarity link should have shared chunk count")
			}
			if shared == 0 {
				t.Error("Documents should have shared chunks")
			}
			break
		}
	}

	if !found {
		t.Error("Similar documents should be linked")
	}
}

// Helper function to compare string slices
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
