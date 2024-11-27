package test

import (
	"bytes"
	"path/filepath"
	"testing"

	"memex/internal/memex/storage"
)

// TestChunking tests content chunking functionality
func TestChunking(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test.mx")

	// Create repository
	repo, err := storage.CreateMX(repoPath)
	if err != nil {
		t.Fatalf("creating repository: %v", err)
	}
	defer repo.Close()

	// Test word-based chunking
	t.Run("Word Based Chunking", func(t *testing.T) {
		// Create content under 1KB to trigger word-based chunking
		content := []byte("The quick brown fox jumps over the lazy dog. This is a test document.")
		meta := map[string]any{"filename": "small.txt"}

		id, err := repo.AddNode(content, "file", meta)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		// Verify content is split into word-based chunks
		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting node: %v", err)
		}

		chunks, ok := node.Meta["chunks"].([]string)
		if !ok {
			t.Fatal("node should have chunks in metadata")
		}

		// Should have multiple chunks based on word boundaries
		if len(chunks) <= 1 {
			t.Error("content should be split into multiple chunks")
		}
	})

	// Test fixed-size chunking
	t.Run("Fixed Size Chunking", func(t *testing.T) {
		// Create content larger than 1KB to trigger fixed-size chunking
		content := bytes.Repeat([]byte("This is test content that will be split into fixed-size chunks. "), 20)
		meta := map[string]any{"filename": "large.txt"}

		id, err := repo.AddNode(content, "file", meta)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		// Verify content is split into fixed-size chunks
		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting node: %v", err)
		}

		chunks, ok := node.Meta["chunks"].([]string)
		if !ok {
			t.Fatal("node should have chunks in metadata")
		}

		if len(chunks) <= 1 {
			t.Error("large content should be split into multiple chunks")
		}
	})

	// Test chunk deduplication
	t.Run("Chunk Deduplication", func(t *testing.T) {
		content := []byte("This content will be stored twice to test chunk deduplication.")
		meta := map[string]any{"filename": "dup.txt"}

		// Store first copy
		id1, err := repo.AddNode(content, "file", meta)
		if err != nil {
			t.Fatalf("storing first copy: %v", err)
		}

		// Store second copy
		id2, err := repo.AddNode(content, "file", meta)
		if err != nil {
			t.Fatalf("storing second copy: %v", err)
		}

		// Get chunks for both copies
		node1, err := repo.GetNode(id1)
		if err != nil {
			t.Fatalf("getting first node: %v", err)
		}

		node2, err := repo.GetNode(id2)
		if err != nil {
			t.Fatalf("getting second node: %v", err)
		}

		chunks1, ok := node1.Meta["chunks"].([]string)
		if !ok {
			t.Fatal("first node should have chunks in metadata")
		}

		chunks2, ok := node2.Meta["chunks"].([]string)
		if !ok {
			t.Fatal("second node should have chunks in metadata")
		}

		// Verify chunks are identical
		if len(chunks1) != len(chunks2) {
			t.Errorf("chunk counts differ: %d != %d", len(chunks1), len(chunks2))
		}

		for i := range chunks1 {
			if chunks1[i] != chunks2[i] {
				t.Errorf("chunk %d differs: %s != %s", i, chunks1[i], chunks2[i])
			}
		}
	})

	// Test very large content
	t.Run("Very Large Content", func(t *testing.T) {
		// Create 1MB of content
		content := make([]byte, 1024*1024)
		for i := range content {
			content[i] = byte(i % 256)
		}

		meta := map[string]any{"filename": "verylarge.txt"}
		id, err := repo.AddNode(content, "file", meta)
		if err != nil {
			t.Fatalf("adding large node: %v", err)
		}

		// Verify content is split into many chunks
		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting large node: %v", err)
		}

		chunks, ok := node.Meta["chunks"].([]string)
		if !ok {
			t.Fatal("large node should have chunks in metadata")
		}

		if len(chunks) < 100 {
			t.Error("very large content should be split into many chunks")
		}

		// Verify content can be reconstructed
		contentHash := node.Meta["content"].(string)
		reconstructed, err := repo.LoadBlob(contentHash)
		if err != nil {
			t.Fatalf("loading large content: %v", err)
		}

		if !bytes.Equal(content, reconstructed) {
			t.Error("large content not preserved correctly")
		}
	})
}
