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

	// Test adding another object for linking
	content2 := []byte("Another test content")
	id2, err := mx.Add(content2, "note", nil)
	if err != nil {
		t.Fatalf("Error adding second object: %v", err)
	}

	// Test linking
	err = mx.Link(id, id2, "references", map[string]any{
		"note": "Test link",
	})
	if err != nil {
		t.Fatalf("Error creating link: %v", err)
	}

	// Test retrieving links
	links, err := mx.GetLinks(id)
	if err != nil {
		t.Fatalf("Error getting links: %v", err)
	}

	if len(links) != 1 {
		t.Errorf("Expected 1 link, got %d", len(links))
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

	// Test updating content
	newContent := []byte("Updated content")
	err = mx.Update(id, newContent)
	if err != nil {
		t.Fatalf("Error updating object: %v", err)
	}

	// Verify update
	updated, err := mx.Get(id)
	if err != nil {
		t.Fatalf("Error getting updated object: %v", err)
	}

	if !bytes.Equal(updated.Content, newContent) {
		t.Error("Updated content does not match")
	}

	// Test deleting an object
	err = mx.Delete(id)
	if err != nil {
		t.Fatalf("Error deleting object: %v", err)
	}

	// Verify object is deleted
	_, err = mx.Get(id)
	if err == nil {
		t.Error("Expected error getting deleted object")
	}

	// Verify object count is updated
	allObjects = mx.List()
	if len(allObjects) != 1 {
		t.Errorf("Expected 1 object after delete, got %d", len(allObjects))
	}
}
