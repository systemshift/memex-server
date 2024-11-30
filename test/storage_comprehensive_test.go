package test

import (
	"bytes"
	"path/filepath"
	"testing"

	"memex/internal/memex/storage"
)

// TestComprehensiveStorage tests all storage functionality together
func TestComprehensiveStorage(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test.mx")

	// Create repository
	repo, err := storage.CreateMX(repoPath)
	if err != nil {
		t.Fatalf("creating repository: %v", err)
	}
	defer repo.Close()

	// Test basic node operations
	t.Run("Basic Node Operations", func(t *testing.T) {
		content := []byte("Test content.")
		meta := map[string]any{"filename": "test.txt"}

		// Add node
		id, err := repo.AddNode(content, "file", meta)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		// Get node
		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting node: %v", err)
		}

		// Check metadata
		if filename, ok := node.Meta["filename"].(string); !ok || filename != "test.txt" {
			t.Errorf("filename not preserved: got %v", node.Meta["filename"])
		}

		// Check content
		contentHash := node.Meta["content"].(string)
		reconstructed, err := repo.ReconstructContent(contentHash)
		if err != nil {
			t.Fatalf("reconstructing content: %v", err)
		}

		if !bytes.Equal(content, reconstructed) {
			t.Error("content not preserved correctly")
		}
	})

	// Test multiple nodes
	t.Run("Multiple Nodes", func(t *testing.T) {
		// Add several nodes
		for i := 0; i < 3; i++ {
			content := []byte("Node content")
			if _, err := repo.AddNode(content, "file", nil); err != nil {
				t.Fatalf("adding node %d: %v", i, err)
			}
		}
	})

	// Test node deletion
	t.Run("Node Deletion", func(t *testing.T) {
		content := []byte("Delete me")
		id, err := repo.AddNode(content, "file", nil)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		if err := repo.DeleteNode(id); err != nil {
			t.Fatalf("deleting node: %v", err)
		}

		if _, err := repo.GetNode(id); err == nil {
			t.Error("deleted node should not be retrievable")
		}
	})

	// Test chunk reuse
	t.Run("Chunk Reuse", func(t *testing.T) {
		content := []byte("This content will be stored twice to test chunk reuse.")

		// Add first copy
		id1, err := repo.AddNode(content, "file", nil)
		if err != nil {
			t.Fatalf("adding first copy: %v", err)
		}

		// Add second copy
		id2, err := repo.AddNode(content, "file", nil)
		if err != nil {
			t.Fatalf("adding second copy: %v", err)
		}

		// Get nodes
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

	// Test file persistence
	t.Run("File Persistence", func(t *testing.T) {
		content := []byte("This content should persist.")
		meta := map[string]any{"filename": "persist.txt"}

		// Add node
		id, err := repo.AddNode(content, "file", meta)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		// Close repository
		if err := repo.Close(); err != nil {
			t.Fatalf("closing repository: %v", err)
		}

		// Reopen repository
		repo, err = storage.OpenMX(repoPath)
		if err != nil {
			t.Fatalf("reopening repository: %v", err)
		}

		// Get node
		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting node after reopen: %v", err)
		}

		// Check content
		contentHash := node.Meta["content"].(string)
		reconstructed, err := repo.ReconstructContent(contentHash)
		if err != nil {
			t.Fatalf("reconstructing content: %v", err)
		}

		if !bytes.Equal(content, reconstructed) {
			t.Error("content not preserved correctly")
		}
	})

	// Test large content
	t.Run("Large Content", func(t *testing.T) {
		// Create 1MB of content
		content := make([]byte, 1024*1024)
		for i := range content {
			content[i] = byte(i % 256)
		}

		// Add node
		id, err := repo.AddNode(content, "file", nil)
		if err != nil {
			t.Fatalf("adding large node: %v", err)
		}

		// Get node
		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting large node: %v", err)
		}

		// Check content
		contentHash := node.Meta["content"].(string)
		reconstructed, err := repo.ReconstructContent(contentHash)
		if err != nil {
			t.Fatalf("reconstructing large content: %v", err)
		}

		if !bytes.Equal(content, reconstructed) {
			t.Error("large content not preserved correctly")
		}
	})

	// Test error cases
	t.Run("Error Cases", func(t *testing.T) {
		// Test invalid node ID
		if _, err := repo.GetNode("invalid"); err == nil {
			t.Error("getting invalid node should fail")
		}

		// Test nil content
		if _, err := repo.AddNode(nil, "file", nil); err == nil {
			t.Error("adding nil content should fail")
		}

		// Test empty type
		if _, err := repo.AddNode([]byte("content"), "", nil); err == nil {
			t.Error("adding empty type should fail")
		}
	})
}
