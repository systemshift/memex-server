package test

import (
	"os"
	"testing"

	"memex/internal/memex/storage"
)

func TestDAGStore(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "memex-test-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create DAG store
	store, err := storage.NewDAGStore(tmpDir)
	if err != nil {
		t.Fatalf("Error creating DAG store: %v", err)
	}

	// Test adding a node
	content := []byte("Test content")
	meta := map[string]any{
		"title": "Test Node",
		"tags":  []string{"test", "example"},
	}

	id, err := store.AddNode(content, "note", meta)
	if err != nil {
		t.Fatalf("Error adding node: %v", err)
	}

	// Test retrieving the node
	node, err := store.GetNode(id)
	if err != nil {
		t.Fatalf("Error getting node: %v", err)
	}

	// Verify node metadata
	if node.Type != "note" {
		t.Errorf("Expected type 'note', got '%s'", node.Type)
	}

	if title, ok := node.Meta["title"].(string); !ok || title != "Test Node" {
		t.Error("Title metadata not preserved correctly")
	}

	// Test getting current version content
	version, err := store.GetVersion(id, node.Current)
	if err != nil {
		t.Fatalf("Error getting version: %v", err)
	}

	// Verify version
	if !version.Available {
		t.Error("Version should be available")
	}

	// Reconstruct content from chunks
	var reconstructed []byte
	for _, hash := range version.Chunks {
		chunk, err := store.GetChunk(hash)
		if err != nil {
			t.Fatalf("Error getting chunk: %v", err)
		}
		reconstructed = append(reconstructed, chunk...)
	}

	if string(reconstructed) != string(content) {
		t.Errorf("Content mismatch. Expected '%s', got '%s'", content, reconstructed)
	}

	// Test updating the node
	newContent := []byte("Updated content")
	newMeta := map[string]any{
		"title": "Updated Node",
		"tags":  []string{"test", "updated"},
	}

	err = store.UpdateNode(id, newContent, newMeta)
	if err != nil {
		t.Fatalf("Error updating node: %v", err)
	}

	// Get updated node
	updated, err := store.GetNode(id)
	if err != nil {
		t.Fatalf("Error getting updated node: %v", err)
	}

	// Verify update
	if title, ok := updated.Meta["title"].(string); !ok || title != "Updated Node" {
		t.Error("Updated title not preserved correctly")
	}

	if len(updated.Versions) != 2 {
		t.Errorf("Expected 2 versions, got %d", len(updated.Versions))
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

	// Test linking nodes
	err = store.AddLink(id, id2, "references", map[string]any{
		"note": "Test link",
	})
	if err != nil {
		t.Fatalf("Error creating link: %v", err)
	}

	// Test getting links
	links, err := store.GetLinks(id)
	if err != nil {
		t.Fatalf("Error getting links: %v", err)
	}

	if len(links) != 1 {
		t.Errorf("Expected 1 link, got %d", len(links))
	}

	if links[0].Source != id || links[0].Target != id2 {
		t.Error("Link source/target not preserved correctly")
	}

	if note, ok := links[0].Meta["note"].(string); !ok || note != "Test link" {
		t.Error("Link metadata not preserved correctly")
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
	root, err := store.GetRoot()
	if err != nil {
		t.Fatalf("Error getting root: %v", err)
	}

	if len(root.Nodes) != 1 { // Should only have id2 left
		t.Errorf("Expected 1 node in root after delete, got %d", len(root.Nodes))
	}

	// Test searching
	nodes, err := store.Search(map[string]any{
		"title": "Another Node",
	})
	if err != nil {
		t.Fatalf("Error searching: %v", err)
	}

	if len(nodes) != 1 {
		t.Errorf("Expected 1 search result, got %d", len(nodes))
	}

	// Test finding by type
	notes, err := store.FindByType("note")
	if err != nil {
		t.Fatalf("Error finding by type: %v", err)
	}

	if len(notes) != 1 {
		t.Errorf("Expected 1 note, got %d", len(notes))
	}
}
