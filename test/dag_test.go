package test

import (
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex/storage"
)

func TestDAG(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "memex-test-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test repository
	repoPath := filepath.Join(tmpDir, "test.mx")
	repo, err := storage.CreateRepository(repoPath, "test")
	if err != nil {
		t.Fatalf("Error creating repository: %v", err)
	}

	// Test adding content
	content := []byte("Test content")
	meta := map[string]any{
		"title": "Test Node",
		"tags":  []string{"test", "example"},
	}

	id, err := repo.AddNode(content, "note", meta)
	if err != nil {
		t.Fatalf("Error adding node: %v", err)
	}

	// Test retrieving content
	node, err := repo.GetNode(id)
	if err != nil {
		t.Fatalf("Error getting node: %v", err)
	}

	if node.Type != "note" {
		t.Error("Node type not preserved correctly")
	}

	if title, ok := node.Meta["title"].(string); !ok || title != "Test Node" {
		t.Error("Node metadata not preserved correctly")
	}

	// Test updating content
	newContent := []byte("Updated content")
	if err := repo.UpdateNode(id, newContent, nil); err != nil {
		t.Fatalf("Error updating node: %v", err)
	}

	// Get updated node
	updated, err := repo.GetNode(id)
	if err != nil {
		t.Fatalf("Error getting updated node: %v", err)
	}

	if updated.Current == node.Current {
		t.Error("Node version not updated")
	}

	// Test linking nodes
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

	if err := repo.AddLink(id, id2, "references", linkMeta); err != nil {
		t.Fatalf("Error creating link: %v", err)
	}

	// Test getting links
	links, err := repo.GetLinks(id)
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

	// Test deleting node
	if err := repo.DeleteNode(id); err != nil {
		t.Fatalf("Error deleting node: %v", err)
	}

	// Verify node is deleted
	if _, err := repo.GetNode(id); err == nil {
		t.Error("Node should be deleted")
	}

	// Test root state
	root, err := repo.GetRoot()
	if err != nil {
		t.Fatalf("Error getting root: %v", err)
	}

	if len(root.Nodes) != 1 { // Should only have id2 left
		t.Errorf("Expected 1 node in root got %d", len(root.Nodes))
	}
}
