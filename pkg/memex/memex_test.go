package memex

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestMemex(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "memex-test-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test repository path
	repoPath := filepath.Join(tmpDir, "test.mx")

	// Create memex
	mx, err := Open(repoPath)
	if err != nil {
		t.Fatalf("Error creating memex: %v", err)
	}

	// Test adding content
	content := []byte("Test content")
	meta := map[string]any{
		"title": "Test Node",
		"tags":  []string{"test", "example"},
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
		t.Error("Content not preserved correctly")
	}

	if title, ok := obj.Meta["title"].(string); !ok || title != "Test Node" {
		t.Error("Metadata not preserved correctly")
	}

	// Test updating content
	newContent := []byte("Updated content")
	if err := mx.Update(id, newContent); err != nil {
		t.Fatalf("Error updating object: %v", err)
	}

	// Get updated object
	updated, err := mx.Get(id)
	if err != nil {
		t.Fatalf("Error getting updated object: %v", err)
	}

	if !bytes.Equal(updated.Content, newContent) {
		t.Error("Updated content not preserved correctly")
	}

	// Test searching
	query := map[string]any{
		"title": "Test Node",
	}

	results, err := mx.Search(query)
	if err != nil {
		t.Fatalf("Error searching: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 search result got %d", len(results))
	}

	// Test finding by type
	notes, err := mx.FindByType("note")
	if err != nil {
		t.Fatalf("Error finding by type: %v", err)
	}

	if len(notes) != 1 {
		t.Errorf("Expected 1 note got %d", len(notes))
	}

	// Test linking objects
	content2 := []byte("Another test")
	meta2 := map[string]any{
		"title": "Another Node",
	}

	id2, err := mx.Add(content2, "note", meta2)
	if err != nil {
		t.Fatalf("Error adding second object: %v", err)
	}

	linkMeta := map[string]any{
		"note": "Test link",
	}

	if err := mx.Link(id, id2, "references", linkMeta); err != nil {
		t.Fatalf("Error creating link: %v", err)
	}

	// Test getting links
	links, err := mx.GetLinks(id)
	if err != nil {
		t.Fatalf("Error getting links: %v", err)
	}

	if len(links) != 1 {
		t.Errorf("Expected 1 link got %d", len(links))
	}

	if links[0].Source != id || links[0].Target != id2 {
		t.Error("Link source/target not preserved correctly")
	}

	if note, ok := links[0].Meta["note"].(string); !ok || note != "Test link" {
		t.Error("Link metadata not preserved correctly")
	}

	// Test deleting object
	if err := mx.Delete(id); err != nil {
		t.Fatalf("Error deleting object: %v", err)
	}

	// Verify object is deleted
	if _, err := mx.Get(id); err == nil {
		t.Error("Object should be deleted")
	}

	// Test listing objects
	objects := mx.List()
	if len(objects) != 1 { // Should only have id2 left
		t.Errorf("Expected 1 object after delete got %d", len(objects))
	}
}
