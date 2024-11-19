package test

import (
	"bytes"
	"os"
	"testing"

	"memex/internal/memex/storage"
)

func TestStorage(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "memex-test-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create repository
	store, err := storage.NewDAGStore(tmpDir)
	if err != nil {
		t.Fatalf("Error creating repository: %v", err)
	}

	// Test adding content
	content := []byte("Test content")
	meta := map[string]any{
		"title": "Test Node",
		"tags":  []string{"test", "example"},
	}

	id, err := store.AddNode(content, "note", meta)
	if err != nil {
		t.Fatalf("Error adding content: %v", err)
	}

	// Test retrieving content
	node, err := store.GetNode(id)
	if err != nil {
		t.Fatalf("Error getting node: %v", err)
	}

	// Get current version content
	version, err := store.GetVersion(id, node.Current)
	if err != nil {
		t.Fatalf("Error getting version: %v", err)
	}

	// Verify version
	if !version.Available {
		t.Error("Version should be available")
	}

	// Reconstruct content
	var reconstructed []byte
	for _, hash := range version.Chunks {
		chunk, err := store.GetChunk(hash)
		if err != nil {
			t.Fatalf("Error getting chunk: %v", err)
		}
		reconstructed = append(reconstructed, chunk...)
	}

	if !bytes.Equal(reconstructed, content) {
		t.Error("Content not preserved correctly")
	}

	// Test updating content
	newContent := []byte("Updated content")
	err = store.UpdateNode(id, newContent, nil)
	if err != nil {
		t.Fatalf("Error updating node: %v", err)
	}

	// Get updated node
	updated, err := store.GetNode(id)
	if err != nil {
		t.Fatalf("Error getting updated node: %v", err)
	}

	// Verify update
	if len(updated.Versions) != 2 {
		t.Errorf("Expected 2 versions, got %d", len(updated.Versions))
	}

	// Test root hash updates
	root, err := store.GetRoot()
	if err != nil {
		t.Fatalf("Error getting root: %v", err)
	}

	if len(root.Nodes) != 1 {
		t.Errorf("Expected 1 node in root, got %d", len(root.Nodes))
	}

	if root.Hash == "" {
		t.Error("Root hash should not be empty")
	}

	// Test content deduplication
	sameContent := []byte("Updated content") // Same as newContent
	meta2 := map[string]any{
		"title": "Another Node",
	}

	id2, err := store.AddNode(sameContent, "note", meta2)
	if err != nil {
		t.Fatalf("Error adding second node: %v", err)
	}

	// Get both nodes
	node1, err := store.GetNode(id)
	if err != nil {
		t.Fatalf("Error getting first node: %v", err)
	}

	node2, err := store.GetNode(id2)
	if err != nil {
		t.Fatalf("Error getting second node: %v", err)
	}

	// Verify they share the same content hash
	if node1.Current != node2.Current {
		t.Error("Content deduplication not working")
	}

	// Test pruning an old version
	oldHash := updated.Versions[0].Hash
	err = store.PruneVersion(id, oldHash)
	if err != nil {
		t.Fatalf("Error pruning version: %v", err)
	}

	// Verify version is marked unavailable
	prunedNode, err := store.GetNode(id)
	if err != nil {
		t.Fatalf("Error getting pruned node: %v", err)
	}

	var found bool
	for _, v := range prunedNode.Versions {
		if v.Hash == oldHash {
			if v.Available {
				t.Error("Pruned version should be marked unavailable")
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("Pruned version not found")
	}

	// Test deleting a node
	err = store.DeleteNode(id)
	if err != nil {
		t.Fatalf("Error deleting node: %v", err)
	}

	// Verify node is deleted
	_, err = store.GetNode(id)
	if err == nil {
		t.Error("Node should be deleted")
	}

	// Verify root is updated
	root, err = store.GetRoot()
	if err != nil {
		t.Fatalf("Error getting root: %v", err)
	}

	if len(root.Nodes) != 1 { // Should only have id2 left
		t.Errorf("Expected 1 node in root after delete, got %d", len(root.Nodes))
	}
}
