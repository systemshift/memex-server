package test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"memex/internal/memex/repository"
)

func TestRepository(t *testing.T) {
	// Create test file
	path := "test.mx"
	defer os.Remove(path)

	// Create repository
	repo, err := repository.Create(path)
	if err != nil {
		t.Fatalf("creating repository: %v", err)
	}
	defer repo.Close()

	t.Run("Basic Node Operations", func(t *testing.T) {
		// Add node
		content := []byte("test content")
		nodeType := "test"
		meta := map[string]interface{}{
			"note": "test note",
		}

		id, err := repo.AddNode(content, nodeType, meta)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}
		t.Logf("Created node with ID: %s", id)

		// Get node
		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting node: %v", err)
		}

		// Verify content
		if !bytes.Equal(node.Content, content) {
			t.Errorf("content mismatch: got %q, want %q", node.Content, content)
		}

		// Verify metadata
		t.Logf("Node metadata: %v", node.Meta)
		if node.Type != nodeType {
			t.Errorf("type mismatch: got %q, want %q", node.Type, nodeType)
		}
		if note, ok := node.Meta["note"].(string); !ok || note != "test note" {
			t.Errorf("note mismatch: got %v", node.Meta["note"])
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

	t.Run("Link Operations", func(t *testing.T) {
		// Create two nodes
		content1 := []byte("node 1")
		id1, err := repo.AddNode(content1, "test", nil)
		if err != nil {
			t.Fatalf("adding node 1: %v", err)
		}
		t.Logf("Created node 1 with ID: %s", id1)

		content2 := []byte("node 2")
		id2, err := repo.AddNode(content2, "test", nil)
		if err != nil {
			t.Fatalf("adding node 2: %v", err)
		}
		t.Logf("Created node 2 with ID: %s", id2)

		// Add link
		linkType := "test"
		linkMeta := map[string]interface{}{
			"note": "test link",
		}
		if err := repo.AddLink(id1, id2, linkType, linkMeta); err != nil {
			t.Fatalf("adding link: %v", err)
		}

		// Get links
		links, err := repo.GetLinks(id1)
		if err != nil {
			t.Fatalf("getting links: %v", err)
		}

		// Verify link
		if len(links) != 1 {
			t.Errorf("link count mismatch: got %d, want 1", len(links))
		} else {
			link := links[0]
			if link.Source != id1 {
				t.Errorf("source mismatch: got %q, want %q", link.Source, id1)
			}
			if link.Target != id2 {
				t.Errorf("target mismatch: got %q, want %q", link.Target, id2)
			}
			if link.Type != linkType {
				t.Errorf("type mismatch: got %q, want %q", link.Type, linkType)
			}
			if note, ok := link.Meta["note"].(string); !ok || note != "test link" {
				t.Errorf("note mismatch: got %v", link.Meta["note"])
			}
		}

		// Delete link
		if err := repo.DeleteLink(id1, id2, linkType); err != nil {
			t.Fatalf("deleting link: %v", err)
		}

		// Verify link is gone
		links, err = repo.GetLinks(id1)
		if err != nil {
			t.Fatalf("getting links after deletion: %v", err)
		}
		if len(links) != 0 {
			t.Error("link still exists after deletion")
		}
	})

	t.Run("Content Deduplication", func(t *testing.T) {
		// Create two documents with similar content
		prefix := strings.Repeat("This is a test document with some shared content. ", 10)
		content1 := []byte(prefix + "This is unique to doc1.")
		id1, err := repo.AddNode(content1, "test", nil)
		if err != nil {
			t.Fatalf("adding doc1: %v", err)
		}
		t.Logf("Created doc1 with ID: %s", id1)

		content2 := []byte(prefix + "This is unique to doc2.")
		id2, err := repo.AddNode(content2, "test", nil)
		if err != nil {
			t.Fatalf("adding doc2: %v", err)
		}
		t.Logf("Created doc2 with ID: %s", id2)

		// Get nodes
		node1, err := repo.GetNode(id1)
		if err != nil {
			t.Fatalf("getting node1: %v", err)
		}
		node2, err := repo.GetNode(id2)
		if err != nil {
			t.Fatalf("getting node2: %v", err)
		}

		// Log metadata for debugging
		t.Logf("Node 1 metadata: %v", node1.Meta)
		t.Logf("Node 2 metadata: %v", node2.Meta)

		// Get chunk IDs
		chunks1, ok := node1.Meta["chunks"].([]interface{})
		if !ok {
			t.Fatal("chunks1 not found in metadata")
		}
		chunks2, ok := node2.Meta["chunks"].([]interface{})
		if !ok {
			t.Fatal("chunks2 not found in metadata")
		}

		t.Logf("Node 1 chunks: %v", chunks1)
		t.Logf("Node 2 chunks: %v", chunks2)

		// Check for shared chunks
		shared := false
		for _, c1 := range chunks1 {
			for _, c2 := range chunks2 {
				if c1 == c2 {
					shared = true
					break
				}
			}
		}

		// Documents should share at least one chunk
		if !shared {
			t.Error("no chunks shared between similar documents")
		}
	})
}
