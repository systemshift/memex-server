package test

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"memex/pkg/memex"
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
	mx, err := memex.Open(repoPath)
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
		t.Errorf("Content not preserved correctly\nExpected: %q\nGot: %q", content, obj.Content)
	}

	if title, ok := obj.Meta["title"].(string); !ok || title != "Test Node" {
		t.Error("Metadata not preserved correctly")
	}

	// Test updating content
	newContent := []byte("Updated content")
	if err := mx.Update(id, newContent); err != nil {
		t.Fatalf("Error updating object: %v", err)
	}

	// Old ID should be gone
	if _, err := mx.Get(id); err == nil {
		t.Error("Old node should be deleted after update")
	}

	// Get all nodes to find updated one
	repo, err := mx.GetRepository()
	if err != nil {
		t.Fatalf("Error getting repository: %v", err)
	}

	var updatedID string
	for _, entry := range repo.Nodes() {
		node, err := repo.GetNode(hex.EncodeToString(entry.ID[:]))
		if err != nil {
			continue
		}
		if node.Type == "note" {
			updatedID = node.ID
			break
		}
	}

	if updatedID == "" {
		t.Fatal("Could not find updated node")
	}

	// Get updated object
	updated, err := mx.Get(updatedID)
	if err != nil {
		t.Fatalf("Error getting updated object: %v", err)
	}

	if !bytes.Equal(updated.Content, newContent) {
		t.Errorf("Updated content not preserved correctly\nExpected: %q\nGot: %q", newContent, updated.Content)
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

	if err := mx.Link(updatedID, id2, "references", linkMeta); err != nil {
		t.Fatalf("Error creating link: %v", err)
	}

	// Test getting links
	links, err := mx.GetLinks(updatedID)
	if err != nil {
		t.Fatalf("Error getting links: %v", err)
	}

	if len(links) != 1 {
		t.Errorf("Expected 1 link got %d", len(links))
	}

	if links[0].Type != "references" {
		t.Error("Link type not preserved correctly")
	}

	if note, ok := links[0].Meta["note"].(string); !ok || note != "Test link" {
		t.Error("Link metadata not preserved correctly")
	}

	// Test deleting object
	if err := mx.Delete(updatedID); err != nil {
		t.Fatalf("Error deleting object: %v", err)
	}

	// Verify object is deleted
	if _, err := mx.Get(updatedID); err == nil {
		t.Error("Object should be deleted")
	}
}
