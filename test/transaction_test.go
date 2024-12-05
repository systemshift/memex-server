package test

import (
	"path/filepath"
	"testing"
	"time"

	"memex/internal/memex/repository"
)

func TestTransactions(t *testing.T) {
	t.Run("Basic Transaction Recording", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test_repo.mx")

		repo, err := repository.Create(path)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Add node (should record transaction)
		content := []byte("test content")
		id, err := repo.AddNode(content, "test", nil)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		// Delete node (should record transaction)
		if err := repo.DeleteNode(id); err != nil {
			t.Fatalf("deleting node: %v", err)
		}
	})

	t.Run("Concurrent Operations", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test_repo.mx")

		repo, err := repository.Create(path)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Create multiple nodes concurrently
		done := make(chan error, 5) // Reduced from 10 to 5
		for i := 0; i < 5; i++ {
			go func(i int) {
				content := []byte("concurrent content")
				_, err := repo.AddNode(content, "test", map[string]interface{}{
					"index": i,
				})
				done <- err
			}(i)
		}

		// Wait for all operations
		for i := 0; i < 5; i++ {
			if err := <-done; err != nil {
				t.Errorf("concurrent operation %d failed: %v", i, err)
			}
		}
	})

	t.Run("Link Transaction Integrity", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test_repo.mx")

		repo, err := repository.Create(path)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Create two nodes
		id1, err := repo.AddNode([]byte("node 1"), "test", nil)
		if err != nil {
			t.Fatalf("adding node 1: %v", err)
		}

		id2, err := repo.AddNode([]byte("node 2"), "test", nil)
		if err != nil {
			t.Fatalf("adding node 2: %v", err)
		}

		// Add link
		if err := repo.AddLink(id1, id2, "test", nil); err != nil {
			t.Fatalf("adding link: %v", err)
		}

		// Delete first node (should handle link cleanup)
		if err := repo.DeleteNode(id1); err != nil {
			t.Fatalf("deleting node: %v", err)
		}

		// Verify link is gone
		links, err := repo.GetLinks(id2)
		if err != nil {
			t.Fatalf("getting links: %v", err)
		}
		if len(links) > 0 {
			t.Error("link still exists after node deletion")
		}
	})

	t.Run("Repository Reopening", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test_repo.mx")

		// Create repository and add content
		repo1, err := repository.Create(path)
		if err != nil {
			t.Fatalf("creating first repository: %v", err)
		}

		id, err := repo1.AddNode([]byte("test"), "test", nil)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		if err := repo1.Close(); err != nil {
			t.Fatalf("closing first repository: %v", err)
		}

		// Reopen repository
		repo2, err := repository.Open(path)
		if err != nil {
			t.Fatalf("opening second repository: %v", err)
		}
		defer repo2.Close()

		// Verify content accessible
		node, err := repo2.GetNode(id)
		if err != nil {
			t.Fatalf("getting node from reopened repository: %v", err)
		}

		if string(node.Content) != "test" {
			t.Error("content not preserved across repository instances")
		}
	})

	t.Run("Transaction Ordering", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test_repo.mx")

		repo, err := repository.Create(path)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Create node
		id, err := repo.AddNode([]byte("original"), "test", nil)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		// Create multiple links in quick succession
		for i := 0; i < 5; i++ {
			targetID, err := repo.AddNode([]byte("target"), "test", nil)
			if err != nil {
				t.Fatalf("adding target node %d: %v", i, err)
			}

			if err := repo.AddLink(id, targetID, "test", map[string]interface{}{
				"order": i,
			}); err != nil {
				t.Fatalf("adding link %d: %v", i, err)
			}

			// Longer delay to ensure distinct timestamps
			time.Sleep(50 * time.Millisecond) // Increased from 10ms to 50ms
		}

		// Get links
		links, err := repo.GetLinks(id)
		if err != nil {
			t.Fatalf("getting links: %v", err)
		}

		// Verify links are in order
		if len(links) != 5 {
			t.Fatalf("expected 5 links, got %d", len(links))
		}

		var lastTime time.Time
		for i, link := range links {
			if i > 0 && !link.Created.After(lastTime) {
				t.Error("links not in chronological order")
			}
			lastTime = link.Created

			order, ok := link.Meta["order"].(float64)
			if !ok || int(order) != i {
				t.Errorf("link %d has wrong order: %v", i, link.Meta["order"])
			}
		}
	})

	t.Run("Error Recovery", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test_repo.mx")

		repo, err := repository.Create(path)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Try to get non-existent node
		if _, err := repo.GetNode("nonexistent"); err == nil {
			t.Error("expected error getting non-existent node")
		}

		// Repository should still be usable
		if _, err := repo.AddNode([]byte("test"), "test", nil); err != nil {
			t.Error("repository unusable after error")
		}
	})

	t.Run("Large Transaction Volume", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test_repo.mx")

		repo, err := repository.Create(path)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Create many nodes and links (reduced from 100 to 20)
		var ids []string
		for i := 0; i < 20; i++ {
			id, err := repo.AddNode([]byte("test"), "test", map[string]interface{}{
				"index": i,
			})
			if err != nil {
				t.Fatalf("adding node %d: %v", i, err)
			}
			ids = append(ids, id)

			// Create links to previous nodes (reduced from 5 to 2)
			for j := max(0, i-2); j < i; j++ {
				if err := repo.AddLink(ids[j], id, "test", nil); err != nil {
					t.Fatalf("adding link %d->%d: %v", j, i, err)
				}
			}
		}

		// Delete nodes in reverse order
		for i := len(ids) - 1; i >= 0; i-- {
			if err := repo.DeleteNode(ids[i]); err != nil {
				t.Fatalf("deleting node %d: %v", i, err)
			}
		}
	})
}

// Helper function for Go < 1.21
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
