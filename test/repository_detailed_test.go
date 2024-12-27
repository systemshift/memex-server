package test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/systemshift/memex/internal/memex/repository"
)

// Helper function to compare maps recursively
func mapsEqual(t *testing.T, a, b map[string]interface{}) bool {
	t.Helper()

	if len(a) != len(b) {
		t.Logf("map length mismatch: %d != %d", len(a), len(b))
		return false
	}

	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			t.Logf("key %q missing from second map", k)
			return false
		}

		// Convert both values to JSON for comparison
		ja, err := json.Marshal(va)
		if err != nil {
			t.Logf("error marshaling first value: %v", err)
			return false
		}
		jb, err := json.Marshal(vb)
		if err != nil {
			t.Logf("error marshaling second value: %v", err)
			return false
		}

		// Compare JSON strings
		if !bytes.Equal(ja, jb) {
			t.Logf("value mismatch at key %q: %s != %s", k, ja, jb)
			return false
		}
	}

	return true
}

func TestRepositoryDetailed(t *testing.T) {
	t.Run("Repository Creation", func(t *testing.T) {
		// Use t.TempDir() for test isolation
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test_repo.mx")

		// Create new repository
		repo, err := repository.Create(path)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		repo.Close()
	})

	t.Run("Node Metadata", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test_repo.mx")

		repo, err := repository.Create(path)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Add node with metadata
		content := []byte("test content")
		nodeType := "test"
		meta := map[string]interface{}{
			"title":  "Test Node",
			"tags":   []string{"test", "metadata"},
			"count":  42.0, // Use float64 for numbers
			"active": true,
		}

		id, err := repo.AddNode(content, nodeType, meta)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		// Get node
		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting node: %v", err)
		}

		// Verify metadata
		if node.Type != nodeType {
			t.Errorf("type mismatch: got %q, want %q", node.Type, nodeType)
		}
		if title, ok := node.Meta["title"].(string); !ok || title != "Test Node" {
			t.Errorf("title mismatch: got %v", node.Meta["title"])
		}
		if tags, ok := node.Meta["tags"].([]interface{}); !ok || !reflect.DeepEqual(tags, []interface{}{"test", "metadata"}) {
			t.Errorf("tags mismatch: got %v", node.Meta["tags"])
		}
		if count, ok := node.Meta["count"].(float64); !ok || count != 42.0 {
			t.Errorf("count mismatch: got %v", node.Meta["count"])
		}
		if active, ok := node.Meta["active"].(bool); !ok || !active {
			t.Errorf("active mismatch: got %v", node.Meta["active"])
		}
	})

	t.Run("Node Timestamps", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test_repo.mx")

		repo, err := repository.Create(path)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Add node
		before := time.Now().Add(-time.Second) // Allow 1 second buffer
		id, err := repo.AddNode([]byte("test"), "test", nil)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}
		after := time.Now().Add(time.Second) // Allow 1 second buffer

		// Get node
		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting node: %v", err)
		}

		// Verify timestamps
		if node.Created.Before(before) || node.Created.After(after) {
			t.Error("created timestamp out of range")
		}
		if node.Modified.Before(before) || node.Modified.After(after) {
			t.Error("modified timestamp out of range")
		}
	})

	t.Run("Complex Link Operations", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test_repo.mx")

		repo, err := repository.Create(path)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Create multiple nodes
		ids := make([]string, 3)
		for i := range ids {
			id, err := repo.AddNode([]byte("node content"), "test", nil)
			if err != nil {
				t.Fatalf("adding node %d: %v", i, err)
			}
			ids[i] = id
		}

		// Create links between nodes
		linkTypes := []string{"parent", "child", "related"}
		linkMetas := []map[string]interface{}{
			{"weight": 1.0, "notes": "first link"},
			{"weight": 2.0, "notes": "second link"},
			{"weight": 3.0, "notes": "third link"},
		}

		for i := 0; i < len(ids)-1; i++ {
			err := repo.AddLink(ids[i], ids[i+1], linkTypes[i], linkMetas[i])
			if err != nil {
				t.Fatalf("adding link %d: %v", i, err)
			}
			// Add small delay to ensure distinct timestamps
			time.Sleep(10 * time.Millisecond)
		}

		// Get links for middle node
		links, err := repo.GetLinks(ids[1])
		if err != nil {
			t.Fatalf("getting links: %v", err)
		}

		// Middle node should have two links (one incoming, one outgoing)
		if len(links) != 2 {
			t.Errorf("expected 2 links for middle node, got %d", len(links))
		}

		// Verify link metadata
		for _, link := range links {
			var expectedType string
			var expectedWeight float64
			if link.Source == ids[0] {
				expectedType = linkTypes[0]
				expectedWeight = 1.0
			} else if link.Target == ids[2] {
				expectedType = linkTypes[1]
				expectedWeight = 2.0
			} else {
				t.Error("unexpected link found")
				continue
			}

			if link.Type != expectedType {
				t.Errorf("link type mismatch: got %q, want %q", link.Type, expectedType)
			}
			if weight, ok := link.Meta["weight"].(float64); !ok || weight != expectedWeight {
				t.Errorf("link weight mismatch: got %v, want %v", link.Meta["weight"], expectedWeight)
			}
		}
	})

	t.Run("JSON Metadata Integrity", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test_repo.mx")

		repo, err := repository.Create(path)
		if err != nil {
			t.Fatalf("creating repository: %v", err)
		}
		defer repo.Close()

		// Create complex metadata
		meta := map[string]interface{}{
			"string": "value",
			"number": 42.5,
			"bool":   true,
			"array":  []interface{}{1.0, "two", 3.0},
			"object": map[string]interface{}{
				"nested": "value",
				"deep": map[string]interface{}{
					"deeper": []interface{}{
						map[string]interface{}{
							"deepest": true,
						},
					},
				},
			},
		}

		// Add node with metadata
		id, err := repo.AddNode([]byte("test"), "test", meta)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		// Get node
		node, err := repo.GetNode(id)
		if err != nil {
			t.Fatalf("getting node: %v", err)
		}

		// Convert both to JSON for comparison
		originalJSON, err := json.Marshal(meta)
		if err != nil {
			t.Fatalf("marshaling original metadata: %v", err)
		}

		retrievedJSON, err := json.Marshal(node.Meta)
		if err != nil {
			t.Fatalf("marshaling retrieved metadata: %v", err)
		}

		// Compare JSON (ignoring system fields)
		var originalMap, retrievedMap map[string]interface{}
		if err := json.Unmarshal(originalJSON, &originalMap); err != nil {
			t.Fatalf("unmarshaling original JSON: %v", err)
		}
		if err := json.Unmarshal(retrievedJSON, &retrievedMap); err != nil {
			t.Fatalf("unmarshaling retrieved JSON: %v", err)
		}

		// Remove system fields from retrieved metadata
		delete(retrievedMap, "type")
		delete(retrievedMap, "created")
		delete(retrievedMap, "modified")
		delete(retrievedMap, "chunks")

		// Compare maps
		if !mapsEqual(t, originalMap, retrievedMap) {
			t.Error("metadata not preserved correctly")
		}
	})
}
