package test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex/storage"
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

	// Create repository
	repo, err := storage.CreateMX(repoPath)
	if err != nil {
		t.Fatalf("Error creating repository: %v", err)
	}
	defer repo.Close()

	// Test adding content
	content := []byte("Test content")
	meta := map[string]any{
		"title": "Test Node",
		"tags":  []string{"test", "example"},
	}

	id, err := repo.AddNode(content, "note", meta)
	if err != nil {
		t.Fatalf("Error adding content: %v", err)
	}

	// Test retrieving content
	node, err := repo.GetNode(id)
	if err != nil {
		t.Fatalf("Error getting node: %v", err)
	}

	contentHash, ok := node.Meta["content"].(string)
	if !ok {
		t.Fatal("Node missing content hash")
	}

	reconstructedContent, err := repo.ReconstructContent(contentHash)
	if err != nil {
		t.Fatalf("Error reconstructing content: %v", err)
	}

	if !bytes.Equal(reconstructedContent, content) {
		t.Errorf("Content not preserved correctly\nExpected: %q\nGot: %q", content, reconstructedContent)
	}

	if title, ok := node.Meta["title"].(string); !ok || title != "Test Node" {
		t.Error("Metadata not preserved correctly")
	}

	// Test updating content
	newContent := []byte("Updated content")
	newMeta := map[string]any{
		"title": "Test Node",
		"tags":  []string{"test", "example", "updated"},
	}
	newID, err := repo.AddNode(newContent, "note", newMeta)
	if err != nil {
		t.Fatalf("Error updating content: %v", err)
	}

	// Delete old node
	if err := repo.DeleteNode(id); err != nil {
		t.Fatalf("Error deleting old node: %v", err)
	}

	// Get updated node
	updatedNode, err := repo.GetNode(newID)
	if err != nil {
		t.Fatalf("Error getting updated node: %v", err)
	}

	contentHash, ok = updatedNode.Meta["content"].(string)
	if !ok {
		t.Fatal("Updated node missing content hash")
	}

	reconstructedContent, err = repo.ReconstructContent(contentHash)
	if err != nil {
		t.Fatalf("Error reconstructing updated content: %v", err)
	}

	if !bytes.Equal(reconstructedContent, newContent) {
		t.Errorf("Updated content not preserved correctly\nExpected: %q\nGot: %q", newContent, reconstructedContent)
	}

	// Test linking objects
	content2 := []byte("Another test")
	meta2 := map[string]any{
		"title": "Another Node",
	}

	id2, err := repo.AddNode(content2, "note", meta2)
	if err != nil {
		t.Fatalf("Error adding second node: %v", err)
	}

	linkMeta := map[string]any{
		"note": "Test link",
	}

	if err := repo.AddLink(newID, id2, "references", linkMeta); err != nil {
		t.Fatalf("Error creating link: %v", err)
	}

	// Test getting links
	links, err := repo.GetLinks(newID)
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
	if err := repo.DeleteNode(newID); err != nil {
		t.Fatalf("Error deleting node: %v", err)
	}

	// Verify object is deleted
	if _, err := repo.GetNode(newID); err == nil {
		t.Error("Node should be deleted")
	}
}
