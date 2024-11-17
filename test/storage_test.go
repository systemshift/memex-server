package test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex/storage"
)

func TestChunking(t *testing.T) {
	content := []byte("This is a test content that should be split into multiple chunks based on content boundaries.")
	chunks, err := storage.ChunkContent(content)
	if err != nil {
		t.Fatalf("Error chunking content: %v", err)
	}

	// Should create at least one chunk
	if len(chunks) == 0 {
		t.Error("No chunks created")
	}

	// Each chunk should have a hash and content
	for i, chunk := range chunks {
		if chunk.Hash == "" {
			t.Errorf("Chunk %d has no hash", i)
		}
		if len(chunk.Content) == 0 {
			t.Errorf("Chunk %d has no content", i)
		}
		if !storage.VerifyChunk(chunk) {
			t.Errorf("Chunk %d failed verification", i)
		}
	}

	// Reassembling chunks should produce original content
	reassembled := storage.ReassembleContent(chunks)
	if !bytes.Equal(reassembled, content) {
		t.Error("Reassembled content does not match original")
	}
}

func TestRepository(t *testing.T) {
	// Create temporary directory for test repository
	tmpDir, err := os.MkdirTemp("", "memex-test-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create repository
	repo, err := storage.NewRepository(filepath.Join(tmpDir, ".memex"))
	if err != nil {
		t.Fatalf("Error creating repository: %v", err)
	}

	// Test adding content
	content := []byte("Test content")
	meta := map[string]any{
		"title": "Test Object",
	}

	id, err := repo.Add(content, "note", meta)
	if err != nil {
		t.Fatalf("Error adding content: %v", err)
	}

	// Test retrieving content
	obj, err := repo.Get(id)
	if err != nil {
		t.Fatalf("Error getting object: %v", err)
	}

	if !bytes.Equal(obj.Content, content) {
		t.Error("Retrieved content does not match original")
	}

	// Verify chunks were created
	if len(obj.Chunks) == 0 {
		t.Log("No chunks created - object stored as raw content")
	} else {
		t.Logf("Object split into %d chunks", len(obj.Chunks))
	}

	// Test updating content
	newContent := []byte("Updated content")
	err = repo.Update(id, newContent)
	if err != nil {
		t.Fatalf("Error updating content: %v", err)
	}

	// Test retrieving updated content
	updated, err := repo.Get(id)
	if err != nil {
		t.Fatalf("Error getting updated object: %v", err)
	}

	if !bytes.Equal(updated.Content, newContent) {
		t.Error("Updated content does not match")
	}

	// Test versions
	versions, err := repo.ListVersions(id)
	if err != nil {
		t.Fatalf("Error listing versions: %v", err)
	}

	if len(versions) != 2 { // Should have initial version and update
		t.Errorf("Expected 2 versions, got %d", len(versions))
	}

	// Test getting specific version
	v1, err := repo.GetVersion(id, 1)
	if err != nil {
		t.Fatalf("Error getting version 1: %v", err)
	}

	if !bytes.Equal(v1.Content, content) {
		t.Error("Version 1 content does not match original")
	}

	// Test chunk-level linking
	content2 := []byte("Another test content for linking")
	id2, err := repo.Add(content2, "note", nil)
	if err != nil {
		t.Fatalf("Error adding second object: %v", err)
	}

	// Get second object
	obj2, err := repo.Get(id2)
	if err != nil {
		t.Fatalf("Error getting second object: %v", err)
	}

	// Only create chunk-level link if both objects have chunks
	if len(obj.Chunks) > 0 && len(obj2.Chunks) > 0 {
		t.Logf("Creating chunk-level link between objects using chunks %s and %s",
			obj.Chunks[0], obj2.Chunks[0])

		// Create link between chunks
		err = repo.LinkChunks(id, obj.Chunks[0], id2, obj2.Chunks[0], "references", nil)
		if err != nil {
			t.Fatalf("Error creating chunk link: %v", err)
		}

		// Test retrieving links
		links, err := repo.GetLinks(id)
		if err != nil {
			t.Fatalf("Error getting links: %v", err)
		}

		if len(links) == 0 {
			t.Error("No links found")
		}

		// Verify chunk-level link
		found := false
		for _, link := range links {
			if link.Source == id && link.Target == id2 && link.SourceChunk == obj.Chunks[0] {
				found = true
				break
			}
		}
		if !found {
			t.Error("Chunk-level link not found")
		}
	} else {
		t.Log("Skipping chunk-level link test - one or both objects have no chunks")
	}
}

func TestRepositorySearch(t *testing.T) {
	// Create temporary directory for test repository
	tmpDir, err := os.MkdirTemp("", "memex-test-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create repository
	repo, err := storage.NewRepository(filepath.Join(tmpDir, ".memex"))
	if err != nil {
		t.Fatalf("Error creating repository: %v", err)
	}

	// Add test objects
	objects := []struct {
		content []byte
		typ     string
		meta    map[string]any
	}{
		{
			content: []byte("Test note 1"),
			typ:     "note",
			meta: map[string]any{
				"title": "Note 1",
				"tags":  []string{"test", "note"},
			},
		},
		{
			content: []byte("Test note 2"),
			typ:     "note",
			meta: map[string]any{
				"title": "Note 2",
				"tags":  []string{"test", "important"},
			},
		},
	}

	for _, obj := range objects {
		_, err := repo.Add(obj.content, obj.typ, obj.meta)
		if err != nil {
			t.Fatalf("Error adding test object: %v", err)
		}
	}

	// Test searching by type
	notes := repo.FindByType("note")
	if len(notes) != 2 {
		t.Errorf("Expected 2 notes, got %d", len(notes))
	}

	// Test searching by metadata
	query := map[string]any{
		"tags": []string{"important"},
	}
	results := repo.Search(query)
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}
