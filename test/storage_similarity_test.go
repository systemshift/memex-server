package test

import (
	"path/filepath"
	"testing"
	"time"

	"memex/internal/memex/storage"
)

// TestSimilarityDetection tests content similarity detection and linking
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

		// Wait for similarity links to be created
		time.Sleep(100 * time.Millisecond)

		// Check links from first node
		links, err := repo.GetLinks(id1)
		if err != nil {
			t.Fatalf("getting links for first node: %v", err)
		}

		var found bool
		for _, link := range links {
			if link.Target == id2 && link.Type == "similar" {
				found = true
				similarity := link.Meta["similarity"].(float64)
				if similarity != 1.0 {
					t.Errorf("exact duplicates should have similarity 1.0, got %f", similarity)
				}
				shared := link.Meta["shared"].(int)
				if shared == 0 {
					t.Error("exact duplicates should have shared chunks")
				}
				break
			}
		}
		if !found {
			t.Error("similarity link not found between exact duplicates")
		}
	})

	// Test similar content
	t.Run("Similar Content", func(t *testing.T) {
		// Create two similar documents under 1KB to use word-based chunking
		content1 := []byte("The quick brown fox jumps over the lazy dog. This is a test document with some shared content.")
		content2 := []byte("The quick brown fox jumps over the lazy cat. This is a test document with some shared content.")

		// Add first document
		id1, err := repo.AddNode(content1, "file", nil)
		if err != nil {
			t.Fatalf("adding first document: %v", err)
		}

		// Add second document
		id2, err := repo.AddNode(content2, "file", nil)
		if err != nil {
			t.Fatalf("adding second document: %v", err)
		}

		// Wait for similarity links to be created
		time.Sleep(100 * time.Millisecond)

		// Check links
		links, err := repo.GetLinks(id1)
		if err != nil {
			t.Fatalf("getting links: %v", err)
		}

		var found bool
		for _, link := range links {
			if link.Target == id2 && link.Type == "similar" {
				found = true
				similarity := link.Meta["similarity"].(float64)
				if similarity < 0.8 {
					t.Errorf("similar documents should have high similarity, got %f", similarity)
				}
				shared := link.Meta["shared"].(int)
				if shared == 0 {
					t.Error("similar documents should have shared chunks")
				}
				break
			}
		}
		if !found {
			t.Error("similarity link not found between similar documents")
		}
	})

	// Test dissimilar content
	t.Run("Dissimilar Content", func(t *testing.T) {
		content1 := []byte("This is a completely different document with unique content.")
		content2 := []byte("Another document with entirely different text and no shared phrases.")

		// Add first document
		id1, err := repo.AddNode(content1, "file", nil)
		if err != nil {
			t.Fatalf("adding first document: %v", err)
		}

		// Add second document
		id2, err := repo.AddNode(content2, "file", nil)
		if err != nil {
			t.Fatalf("adding second document: %v", err)
		}

		// Wait for similarity links to be created
		time.Sleep(100 * time.Millisecond)

		// Check links
		links, err := repo.GetLinks(id1)
		if err != nil {
			t.Fatalf("getting links: %v", err)
		}

		// Verify no similarity link exists
		for _, link := range links {
			if link.Target == id2 && link.Type == "similar" {
				similarity := link.Meta["similarity"].(float64)
				t.Errorf("dissimilar documents should not be linked (got similarity %f)", similarity)
			}
		}
	})

	// Test similarity threshold
	t.Run("Similarity Threshold", func(t *testing.T) {
		// Create documents with minimal similarity
		content1 := []byte("Document one has some words that are shared.")
		content2 := []byte("Document two has some words but is mostly different content entirely.")

		// Add first document
		id1, err := repo.AddNode(content1, "file", nil)
		if err != nil {
			t.Fatalf("adding first document: %v", err)
		}

		// Add second document
		id2, err := repo.AddNode(content2, "file", nil)
		if err != nil {
			t.Fatalf("adding second document: %v", err)
		}

		// Wait for similarity links to be created
		time.Sleep(100 * time.Millisecond)

		// Check links
		links, err := repo.GetLinks(id1)
		if err != nil {
			t.Fatalf("getting links: %v", err)
		}

		// Verify links respect similarity threshold
		for _, link := range links {
			if link.Target == id2 && link.Type == "similar" {
				similarity := link.Meta["similarity"].(float64)
				if similarity < 0.3 {
					t.Error("links should only be created for similarity >= 0.3")
				}
			}
		}
	})
}
