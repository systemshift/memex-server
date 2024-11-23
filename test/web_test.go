package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex/storage"
)

func TestWebServer(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "memex-web-test-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test repository
	repoPath := filepath.Join(tmpDir, "test.mx")
	store, err := storage.CreateMX(repoPath)
	if err != nil {
		t.Fatalf("Error creating repository: %v", err)
	}

	// Add test content
	content1 := []byte("node1")
	meta1 := map[string]any{
		"filename": "n1.txt",
		"type":     "file",
	}
	id1, err := store.AddNode(content1, "file", meta1)
	if err != nil {
		t.Fatalf("Error adding first node: %v", err)
	}

	content2 := []byte("node2")
	meta2 := map[string]any{
		"filename": "n2.txt",
		"type":     "file",
	}
	id2, err := store.AddNode(content2, "file", meta2)
	if err != nil {
		t.Fatalf("Error adding second node: %v", err)
	}

	// Create link
	err = store.AddLink(id1, id2, "references", map[string]any{"note": "test link"})
	if err != nil {
		t.Fatalf("Error creating link: %v", err)
	}

	// Create server
	server := &Server{
		memex: store,
	}

	// Test /api/graph endpoint
	t.Run("Graph API", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/graph", nil)
		w := httptest.NewRecorder()
		server.handleGraph(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var graph GraphData
		if err := json.NewDecoder(w.Body).Decode(&graph); err != nil {
			t.Errorf("Error decoding response: %v", err)
		}

		if len(graph.Nodes) != 2 {
			t.Errorf("Expected 2 nodes, got %d", len(graph.Nodes))
		}

		if len(graph.Edges) != 1 {
			t.Errorf("Expected 1 edge, got %d", len(graph.Edges))
		}

		// Verify node data
		for _, node := range graph.Nodes {
			if node.Type != "file" {
				t.Errorf("Expected node type 'file', got %q", node.Type)
			}
			if filename, ok := node.Meta["filename"].(string); !ok {
				t.Error("Node missing filename")
			} else if filename != "n1.txt" && filename != "n2.txt" {
				t.Errorf("Unexpected filename: %s", filename)
			}
		}

		// Verify edge data
		edge := graph.Edges[0]
		if edge.Type != "references" {
			t.Errorf("Expected edge type 'references', got %q", edge.Type)
		}
		if note, ok := edge.Meta["note"].(string); !ok {
			t.Error("Edge missing note")
		} else if note != "test link" {
			t.Errorf("Expected note 'test link', got %q", note)
		}
	})

	// Test /api/content endpoint
	t.Run("Content API", func(t *testing.T) {
		// Get content hash from node metadata
		node, err := store.GetNode(id1)
		if err != nil {
			t.Fatalf("Error getting node: %v", err)
		}
		contentHash, ok := node.Meta["content"].(string)
		if !ok {
			t.Fatal("Node missing content hash")
		}

		req := httptest.NewRequest("GET", "/api/content/"+contentHash, nil)
		w := httptest.NewRecorder()
		server.handleContent(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		if w.Body.String() != "node1" {
			t.Errorf("Expected content %q, got %q", "node1", w.Body.String())
		}
	})

	// Test /node endpoint
	t.Run("Node API", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/node/"+id1, nil)
		w := httptest.NewRecorder()
		server.handleNode(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Should serve file content
		if w.Body.String() != "node1" {
			t.Errorf("Expected content %q, got %q", "node1", w.Body.String())
		}

		// Should set filename header
		filename := w.Header().Get("Content-Disposition")
		if filename != `attachment; filename="n1.txt"` {
			t.Errorf("Expected filename header %q, got %q", `attachment; filename="n1.txt"`, filename)
		}
	})

	// Test 404 responses
	t.Run("Not Found", func(t *testing.T) {
		// Test invalid content hash
		req := httptest.NewRequest("GET", "/api/content/invalid", nil)
		w := httptest.NewRecorder()
		server.handleContent(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
		}

		// Test invalid node ID
		req = httptest.NewRequest("GET", "/node/invalid", nil)
		w = httptest.NewRecorder()
		server.handleNode(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})
}
