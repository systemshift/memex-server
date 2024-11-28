package test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"memex/internal/memex/storage"
)

// TestPersistence tests data persistence and integrity
func TestPersistence(t *testing.T) {
	// Test basic persistence
	t.Run("Basic Persistence", func(t *testing.T) {
		// Create temporary directory for this subtest
		tmpDir := t.TempDir()
		repoPath := filepath.Join(tmpDir, "test.mx")

		// Create repository and add content
		repo, err := storage.CreateMX(repoPath)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}

		content := []byte("This content should persist across repository restarts.")
		meta := map[string]any{
			"filename": "test.txt",
			"created":  time.Now().Format(time.RFC3339),
		}

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
		defer repo.Close()

		// Verify content persisted
		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting node after reopen: %v", err)
		}

		// Check metadata
		if filename, ok := node.Meta["filename"].(string); !ok || filename != "test.txt" {
			t.Errorf("filename not persisted correctly: got %v", node.Meta["filename"])
		}

		// Check content
		contentHash := node.Meta["content"].(string)
		hash := sha256.Sum256(content)
		expected := hex.EncodeToString(hash[:])
		if contentHash != expected {
			t.Errorf("content hash mismatch: got %s, want %s", contentHash, expected)
		}

		// Verify content can be reconstructed
		reconstructed, err := repo.ReconstructContent(contentHash)
		if err != nil {
			t.Fatalf("reconstructing content after reopen: %v", err)
		}

		if !bytes.Equal(content, reconstructed) {
			t.Error("content not preserved correctly")
		}
	})

	// Test persistence of multiple nodes and links
	t.Run("Multiple Nodes and Links", func(t *testing.T) {
		// Create temporary directory for this subtest
		tmpDir := t.TempDir()
		repoPath := filepath.Join(tmpDir, "test.mx")

		// Create repository
		repo, err := storage.CreateMX(repoPath)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}

		// Add similar documents to create links
		content1 := []byte("The quick brown fox jumps over the lazy dog.")
		content2 := []byte("The quick brown fox jumps over the lazy cat.")

		id1, err := repo.AddNode(content1, "file", nil)
		if err != nil {
			t.Fatalf("adding first node: %v", err)
		}

		id2, err := repo.AddNode(content2, "file", nil)
		if err != nil {
			t.Fatalf("adding second node: %v", err)
		}

		// Wait for similarity links to be created
		time.Sleep(100 * time.Millisecond)

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

		// Verify nodes persisted
		if _, err := repo.GetNode(id1); err != nil {
			t.Fatalf("getting first node after reopen: %v", err)
		}

		if _, err := repo.GetNode(id2); err != nil {
			t.Fatalf("getting second node after reopen: %v", err)
		}

		// Verify links persisted
		links, err := repo.GetLinks(id1)
		if err != nil {
			t.Fatalf("getting links after reopen: %v", err)
		}

		var found bool
		for _, link := range links {
			if link.Target == id2 && link.Type == "similar" {
				found = true
				similarity := link.Meta["similarity"].(float64)
				if similarity < 0.8 {
					t.Errorf("similarity not preserved correctly: got %f", similarity)
				}
				break
			}
		}
		if !found {
			t.Error("similarity link not preserved")
		}
	})

	// Test persistence of large content
	t.Run("Large Content", func(t *testing.T) {
		// Create temporary directory for this subtest
		tmpDir := t.TempDir()
		repoPath := filepath.Join(tmpDir, "test.mx")

		// Create repository
		repo, err := storage.CreateMX(repoPath)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}

		// Create large content (1MB)
		content := make([]byte, 1024*1024)
		for i := range content {
			content[i] = byte(i % 256)
		}

		// Add large content
		id, err := repo.AddNode(content, "file", nil)
		if err != nil {
			t.Fatalf("adding large node: %v", err)
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

		// Verify large content persisted
		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting large node after reopen: %v", err)
		}

		contentHash := node.Meta["content"].(string)
		hash := sha256.Sum256(content)
		expected := hex.EncodeToString(hash[:])
		if contentHash != expected {
			t.Errorf("large content hash mismatch: got %s, want %s", contentHash, expected)
		}

		// Verify chunks persisted
		chunks, ok := node.Meta["chunks"].([]string)
		if !ok {
			t.Fatal("chunks metadata not preserved")
		}
		if len(chunks) < 100 {
			t.Error("large content should have many chunks")
		}

		// Verify content can be reconstructed
		reconstructed, err := repo.ReconstructContent(contentHash)
		if err != nil {
			t.Fatalf("reconstructing large content after reopen: %v", err)
		}

		if !bytes.Equal(content, reconstructed) {
			t.Error("large content not preserved correctly")
		}
	})
}
