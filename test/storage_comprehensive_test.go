package test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"memex/internal/memex/storage"
)

func TestComprehensiveStorage(t *testing.T) {
	// Test basic node operations
	t.Run("Basic Node Operations", func(t *testing.T) {
		// Create test directory
		testDir := t.TempDir()
		repoPath := filepath.Join(testDir, "test.mx")

		// Create repository
		repo, err := storage.CreateMX(repoPath)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Add node
		content := []byte("test content 1")
		meta := map[string]any{
			"filename": "test1.txt",
			"type":     "file",
		}
		id1, err := repo.AddNode(content, "file", meta)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		// Get node
		node, err := repo.GetNode(id1)
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

	// Test multiple nodes
	t.Run("Multiple Nodes", func(t *testing.T) {
		// Create test directory
		testDir := t.TempDir()
		repoPath := filepath.Join(testDir, "test.mx")

		// Create repository
		repo, err := storage.CreateMX(repoPath)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Add multiple nodes
		nodes := []struct {
			content []byte
			meta    map[string]any
		}{
			{[]byte("content 1"), map[string]any{"name": "node1"}},
			{[]byte("content 2"), map[string]any{"name": "node2"}},
			{[]byte("content 3"), map[string]any{"name": "node3"}},
		}

		var ids []string
		for _, n := range nodes {
			id, err := repo.AddNode(n.content, "test", n.meta)
			if err != nil {
				t.Fatalf("adding node: %v", err)
			}
			ids = append(ids, id)
		}

		// Verify each node
		for i, id := range ids {
			node, err := repo.GetNode(id)
			if err != nil {
				t.Fatalf("getting node %s: %v", id, err)
			}
			if name, ok := node.Meta["name"].(string); !ok || name != fmt.Sprintf("node%d", i+1) {
				t.Errorf("wrong name for node %s: got %v, want node%d", id, name, i+1)
			}
		}
	})

	// Test node deletion
	t.Run("Node Deletion", func(t *testing.T) {
		// Create test directory
		testDir := t.TempDir()
		repoPath := filepath.Join(testDir, "test.mx")

		// Create repository
		repo, err := storage.CreateMX(repoPath)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Add node
		content := []byte("delete me")
		id, err := repo.AddNode(content, "test", nil)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		// Delete node
		if err := repo.DeleteNode(id); err != nil {
			t.Fatalf("deleting node: %v", err)
		}

		// Verify node is gone
		if _, err := repo.GetNode(id); err == nil {
			t.Error("node still exists after deletion")
		}
	})

	// Test similar content detection
	t.Run("Similar Content", func(t *testing.T) {
		// Create test directory
		testDir := t.TempDir()
		repoPath := filepath.Join(testDir, "test.mx")

		// Create repository
		repo, err := storage.CreateMX(repoPath)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Add two nodes with similar content (under 1KB to use word-based chunking)
		content1 := []byte("The quick brown fox jumps over the lazy dog. This is a test document with some shared content.")
		content2 := []byte("The quick brown fox jumps over the lazy cat. This is a test document with some shared content.")

		id1, err := repo.AddNode(content1, "test", nil)
		if err != nil {
			t.Fatalf("adding first node: %v", err)
		}

		id2, err := repo.AddNode(content2, "test", nil)
		if err != nil {
			t.Fatalf("adding second node: %v", err)
		}

		// Wait for similarity links to be created
		time.Sleep(100 * time.Millisecond)

		// Get links
		links, err := repo.GetLinks(id1)
		if err != nil {
			t.Fatalf("getting links: %v", err)
		}

		t.Logf("Found %d links for node %s", len(links), id1)
		for _, link := range links {
			t.Logf("Link: %s -> %s [%s] meta: %v", link.Source, link.Target, link.Type, link.Meta)
		}

		// Verify similarity link exists
		var found bool
		for _, link := range links {
			if link.Target == id2 && link.Type == "similar" {
				found = true
				if similarity, ok := link.Meta["similarity"].(float64); !ok {
					t.Error("similarity not found in link metadata")
				} else if similarity < 0.3 {
					t.Errorf("similarity too low: got %f, want >= 0.3", similarity)
				}
				if shared, ok := link.Meta["shared"].(int); !ok {
					t.Error("shared chunks not found in link metadata")
				} else if shared == 0 {
					t.Error("no shared chunks found")
				}
				break
			}
		}
		if !found {
			t.Error("similarity link not found")
		}
	})

	// Test file persistence
	t.Run("File Persistence", func(t *testing.T) {
		// Create test directory
		testDir := t.TempDir()
		repoPath := filepath.Join(testDir, "test.mx")

		// Create repository
		repo, err := storage.CreateMX(repoPath)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}

		// Add node
		content := []byte("persistence test")
		id, err := repo.AddNode(content, "test", nil)
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
		defer repo.Close()

		// Get node
		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting node after reopen: %v", err)
		}

		// Verify content hash
		hash := sha256.Sum256(content)
		expected := hex.EncodeToString(hash[:])
		if contentHash, ok := node.Meta["content"].(string); !ok || contentHash != expected {
			t.Errorf("wrong content hash after reopen: got %s, want %s", contentHash, expected)
		}
	})

	// Test large content
	t.Run("Large Content", func(t *testing.T) {
		// Create test directory
		testDir := t.TempDir()
		repoPath := filepath.Join(testDir, "test.mx")

		// Create repository
		repo, err := storage.CreateMX(repoPath)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Create large content (1MB)
		content := make([]byte, 1024*1024)
		for i := range content {
			content[i] = byte(i % 256)
		}

		// Add node with large content
		id, err := repo.AddNode(content, "test", nil)
		if err != nil {
			t.Fatalf("adding large node: %v", err)
		}

		// Get node
		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting large node: %v", err)
		}

		// Verify content hash
		hash := sha256.Sum256(content)
		expected := hex.EncodeToString(hash[:])
		if contentHash, ok := node.Meta["content"].(string); !ok || contentHash != expected {
			t.Errorf("wrong content hash for large node: got %s, want %s", contentHash, expected)
		}

		// Verify chunks
		chunks, ok := node.Meta["chunks"].([]string)
		if !ok {
			t.Fatal("chunks not found in large node metadata")
		}
		if len(chunks) < 2 {
			t.Error("large content should be split into multiple chunks")
		}
	})

	// Test error cases
	t.Run("Error Cases", func(t *testing.T) {
		// Create test directory
		testDir := t.TempDir()
		repoPath := filepath.Join(testDir, "test.mx")

		// Create repository
		repo, err := storage.CreateMX(repoPath)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Test invalid node ID
		if _, err := repo.GetNode("invalid"); err == nil {
			t.Error("expected error for invalid node ID")
		}

		// Test deleting non-existent node
		if err := repo.DeleteNode("nonexistent"); err == nil {
			t.Error("expected error for deleting non-existent node")
		}

		// Test adding node with nil content
		if _, err := repo.AddNode(nil, "test", nil); err == nil {
			t.Error("expected error for nil content")
		}

		// Test adding node with empty type
		if _, err := repo.AddNode([]byte("test"), "", nil); err == nil {
			t.Error("expected error for empty type")
		}
	})
}
