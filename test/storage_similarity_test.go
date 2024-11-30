package test

import (
	"path/filepath"
	"testing"

	"memex/internal/memex/storage"
)

// TestSimilarityDetection tests content deduplication through chunk reuse
func TestSimilarityDetection(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test.mx")

	// Create repository
	repo, err := storage.CreateMX(repoPath)
	if err != nil {
		t.Fatalf("creating repository: %v", err)
	}
	defer repo.Close()

	// Test exact duplicates
	t.Run("Exact Duplicates", func(t *testing.T) {
		content := []byte("This is a test document that will be duplicated exactly.")
		meta := map[string]any{}

		// Add first copy
		id1, err := repo.AddNode(content, "file", meta)
		if err != nil {
			t.Fatalf("adding first copy: %v", err)
		}

		// Add second copy
		id2, err := repo.AddNode(content, "file", meta)
		if err != nil {
			t.Fatalf("adding second copy: %v", err)
		}

		// Get nodes to check chunk reuse
		node1, err := repo.GetNode(id1)
		if err != nil {
			t.Fatalf("getting first node: %v", err)
		}

		node2, err := repo.GetNode(id2)
		if err != nil {
			t.Fatalf("getting second node: %v", err)
		}

		// Verify chunks are reused
		chunks1 := node1.Meta["chunks"].([]string)
		chunks2 := node2.Meta["chunks"].([]string)

		if len(chunks1) != len(chunks2) {
			t.Errorf("chunk counts differ: %d != %d", len(chunks1), len(chunks2))
		}

		for i := range chunks1 {
			if chunks1[i] != chunks2[i] {
				t.Errorf("chunk %d differs: %s != %s", i, chunks1[i], chunks2[i])
			}
		}
	})

	// Test partial chunk reuse
	t.Run("Partial Reuse", func(t *testing.T) {
		// Create two files that share a 4KB block
		sharedBlock := make([]byte, 4096)
		for i := range sharedBlock {
			sharedBlock[i] = byte(i % 256)
		}

		content1 := append(sharedBlock, []byte("unique content 1")...)
		content2 := append(sharedBlock, []byte("unique content 2")...)

		// Add first file
		id1, err := repo.AddNode(content1, "file", nil)
		if err != nil {
			t.Fatalf("adding first file: %v", err)
		}

		// Add second file
		id2, err := repo.AddNode(content2, "file", nil)
		if err != nil {
			t.Fatalf("adding second file: %v", err)
		}

		// Get nodes to check chunk reuse
		node1, err := repo.GetNode(id1)
		if err != nil {
			t.Fatalf("getting first node: %v", err)
		}

		node2, err := repo.GetNode(id2)
		if err != nil {
			t.Fatalf("getting second node: %v", err)
		}

		// Verify at least one chunk is shared
		chunks1 := node1.Meta["chunks"].([]string)
		chunks2 := node2.Meta["chunks"].([]string)

		var sharedChunks int
		for _, c1 := range chunks1 {
			for _, c2 := range chunks2 {
				if c1 == c2 {
					sharedChunks++
				}
			}
		}

		if sharedChunks == 0 {
			t.Error("no chunks shared between files with identical blocks")
		}
	})

	// Test unique content
	t.Run("Unique Content", func(t *testing.T) {
		content1 := make([]byte, 4096)
		content2 := make([]byte, 4096)

		// Fill with different data
		for i := range content1 {
			content1[i] = byte(i % 256)
			content2[i] = byte((i + 128) % 256)
		}

		// Add first file
		id1, err := repo.AddNode(content1, "file", nil)
		if err != nil {
			t.Fatalf("adding first file: %v", err)
		}

		// Add second file
		id2, err := repo.AddNode(content2, "file", nil)
		if err != nil {
			t.Fatalf("adding second file: %v", err)
		}

		// Get nodes to check chunks
		node1, err := repo.GetNode(id1)
		if err != nil {
			t.Fatalf("getting first node: %v", err)
		}

		node2, err := repo.GetNode(id2)
		if err != nil {
			t.Fatalf("getting second node: %v", err)
		}

		// Verify no chunks are shared
		chunks1 := node1.Meta["chunks"].([]string)
		chunks2 := node2.Meta["chunks"].([]string)

		for _, c1 := range chunks1 {
			for _, c2 := range chunks2 {
				if c1 == c2 {
					t.Error("unique content should not share any chunks")
				}
			}
		}
	})
}
