package test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	"memex/internal/memex/storage"
)

// TestBasicNodeOperations tests fundamental node operations
func TestBasicNodeOperations(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test.mx")

	// Create repository
	repo, err := storage.CreateMX(repoPath)
	if err != nil {
		t.Fatalf("creating repository: %v", err)
	}
	defer repo.Close()

	// Test adding a node
	t.Run("Add Node", func(t *testing.T) {
		content := []byte("test content 1")
		meta := map[string]any{
			"filename": "test1.txt",
			"type":     "file",
		}
		id, err := repo.AddNode(content, "file", meta)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		// Get node
		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting node: %v", err)
		}

		// Verify node data
		if node.Type != "file" {
			t.Errorf("wrong type: got %s, want file", node.Type)
		}
		if filename, ok := node.Meta["filename"].(string); !ok || filename != "test1.txt" {
			t.Errorf("wrong filename: got %v, want test1.txt", node.Meta["filename"])
		}
		if contentHash, ok := node.Meta["content"].(string); !ok {
			t.Error("content hash not found in metadata")
		} else {
			hash := sha256.Sum256(content)
			expected := hex.EncodeToString(hash[:])
			if contentHash != expected {
				t.Errorf("wrong content hash: got %s, want %s", contentHash, expected)
			}
		}
	})

	// Test retrieving content
	t.Run("Get Content", func(t *testing.T) {
		content := []byte("test content 2")
		meta := map[string]any{"filename": "test2.txt"}
		id, err := repo.AddNode(content, "file", meta)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting node: %v", err)
		}

		contentHash := node.Meta["content"].(string)
		blob, err := repo.LoadBlob(contentHash)
		if err != nil {
			t.Fatalf("loading blob: %v", err)
		}

		if !bytes.Equal(content, blob) {
			t.Error("content not preserved correctly")
		}
	})

	// Test deleting content
	t.Run("Delete Node", func(t *testing.T) {
		content := []byte("test content 3")
		id, err := repo.AddNode(content, "file", nil)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		// Verify node exists
		if _, err := repo.GetNode(id); err != nil {
			t.Error("node should exist")
		}

		// Delete node
		if err := repo.DeleteNode(id); err != nil {
			t.Fatalf("deleting node: %v", err)
		}

		// Verify node is gone
		if _, err := repo.GetNode(id); err == nil {
			t.Error("node should be deleted")
		}
	})

	// Test duplicate content handling
	t.Run("Duplicate Content", func(t *testing.T) {
		content := []byte("duplicate content")
		meta := map[string]any{"filename": "dup.txt"}

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

		// Verify both copies exist
		if _, err := repo.GetNode(id1); err != nil {
			t.Error("first copy should exist")
		}
		if _, err := repo.GetNode(id2); err != nil {
			t.Error("second copy should exist")
		}

		// Delete first copy
		if err := repo.DeleteNode(id1); err != nil {
			t.Fatalf("deleting first copy: %v", err)
		}

		// Verify second copy still exists
		if _, err := repo.GetNode(id2); err != nil {
			t.Error("second copy should still exist")
		}

		// Delete second copy
		if err := repo.DeleteNode(id2); err != nil {
			t.Fatalf("deleting second copy: %v", err)
		}

		// Verify both copies are gone
		if _, err := repo.GetNode(id1); err == nil {
			t.Error("first copy should be deleted")
		}
		if _, err := repo.GetNode(id2); err == nil {
			t.Error("second copy should be deleted")
		}
	})
}

// TestErrorCases tests various error conditions
func TestErrorCases(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test.mx")

	// Create repository
	repo, err := storage.CreateMX(repoPath)
	if err != nil {
		t.Fatalf("creating repository: %v", err)
	}
	defer repo.Close()

	// Test invalid node ID
	t.Run("Invalid Node ID", func(t *testing.T) {
		if _, err := repo.GetNode("invalid"); err == nil {
			t.Error("expected error for invalid node ID")
		}
	})

	// Test deleting non-existent node
	t.Run("Delete Non-existent Node", func(t *testing.T) {
		if err := repo.DeleteNode("nonexistent"); err == nil {
			t.Error("expected error for deleting non-existent node")
		}
	})

	// Test adding node with nil content
	t.Run("Nil Content", func(t *testing.T) {
		if _, err := repo.AddNode(nil, "test", nil); err == nil {
			t.Error("expected error for nil content")
		}
	})

	// Test adding node with empty type
	t.Run("Empty Type", func(t *testing.T) {
		if _, err := repo.AddNode([]byte("test"), "", nil); err == nil {
			t.Error("expected error for empty type")
		}
	})
}
