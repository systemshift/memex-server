package memex

import (
	"bytes"
	"os"
	"testing"
)

func TestMemex(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "memex-test-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create new memex instance
	mx, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Error opening memex: %v", err)
	}

	// Test adding content
	content := []byte("Test content")
	meta := map[string]any{
		"title": "Test Object",
	}

	id, err := mx.Add(content, "note", meta)
	if err != nil {
		t.Fatalf("Error adding content: %v", err)
	}

	// Test retrieving content
	obj, err := mx.Get(id)
	if err != nil {
		t.Fatalf("Error getting object: %v", err)
	}

	if !bytes.Equal(obj.Content, content) {
		t.Error("Retrieved content does not match original")
	}

	if obj.Type != "note" {
		t.Errorf("Expected type 'note', got '%s'", obj.Type)
	}

	if title, ok := obj.Meta["title"].(string); !ok || title != "Test Object" {
		t.Error("Metadata not preserved correctly")
	}

	// Test chunk-level operations
	if len(obj.Chunks) == 0 {
		t.Error("No chunks created")
	}

	chunks, err := mx.GetObjectChunks(id)
	if err != nil {
		t.Fatalf("Error getting chunks: %v", err)
	}

	if len(chunks) == 0 {
		t.Error("No chunks returned")
	}

	// Test adding another object for linking
	content2 := []byte("Another test content")
	id2, err := mx.Add(content2, "note", nil)
	if err != nil {
		t.Fatalf("Error adding second object: %v", err)
	}

	// Test file-level linking
	err = mx.Link(id, id2, "references", map[string]any{
		"note": "Test link",
	})
	if err != nil {
		t.Fatalf("Error creating link: %v", err)
	}

	// Test chunk-level linking
	obj1, _ := mx.Get(id)
	obj2, _ := mx.Get(id2)

	if len(obj1.Chunks) > 0 && len(obj2.Chunks) > 0 {
		err = mx.LinkChunks(id, obj1.Chunks[0], id2, obj2.Chunks[0], "references", map[string]any{
			"note": "Chunk link",
		})
		if err != nil {
			t.Fatalf("Error creating chunk link: %v", err)
		}
	}

	// Test retrieving links
	links, err := mx.GetLinks(id)
	if err != nil {
		t.Fatalf("Error getting links: %v", err)
	}

	if len(links) != 2 {
		t.Errorf("Expected 2 links (file-level and chunk-level), got %d", len(links))
	}

	// Test searching
	results := mx.Search(map[string]any{
		"title": "Test Object",
	})

	if len(results) != 1 {
		t.Errorf("Expected 1 search result, got %d", len(results))
	}

	// Test finding by type
	notes := mx.FindByType("note")
	if len(notes) != 2 {
		t.Errorf("Expected 2 notes, got %d", len(notes))
	}

	// Test listing all objects
	allObjects := mx.List()
	if len(allObjects) != 2 {
		t.Errorf("Expected 2 objects, got %d", len(allObjects))
	}

	// Test versioning
	newContent := []byte("Updated content")
	err = mx.Update(id, newContent)
	if err != nil {
		t.Fatalf("Error updating object: %v", err)
	}

	versions, err := mx.ListVersions(id)
	if err != nil {
		t.Fatalf("Error listing versions: %v", err)
	}

	if len(versions) != 2 { // Should have initial version and update
		t.Errorf("Expected 2 versions, got %d", len(versions))
	}

	// Test getting specific version
	v1, err := mx.GetVersion(id, 1)
	if err != nil {
		t.Fatalf("Error getting version 1: %v", err)
	}

	if !bytes.Equal(v1.Content, content) {
		t.Error("Version 1 content does not match original")
	}
}
