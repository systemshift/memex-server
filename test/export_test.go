package test

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex/migration"
	"memex/internal/memex/storage"
)

func TestExport(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "memex-export-test-*")
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

	content3 := []byte("node3")
	meta3 := map[string]any{
		"filename": "n3.txt",
		"type":     "file",
	}
	id3, err := store.AddNode(content3, "file", meta3)
	if err != nil {
		t.Fatalf("Error adding third node: %v", err)
	}

	// Create links
	err = store.AddLink(id1, id2, "references", map[string]any{"note": "link1"})
	if err != nil {
		t.Fatalf("Error creating first link: %v", err)
	}

	err = store.AddLink(id2, id3, "references", map[string]any{"note": "link2"})
	if err != nil {
		t.Fatalf("Error creating second link: %v", err)
	}

	// Test different export depths
	depths := []struct {
		depth       int
		wantNodes   int
		wantEdges   int
		wantChunks  int
		wantContent [][]byte
	}{
		{0, 1, 1, 1, [][]byte{content1}},
		{1, 2, 2, 2, [][]byte{content1, content2}},
		{2, 3, 2, 3, [][]byte{content1, content2, content3}},
	}

	for _, tt := range depths {
		t.Run(filepath.Join("depth", string(rune('0'+tt.depth))), func(t *testing.T) {
			var buf bytes.Buffer

			// Create exporter
			exporter := migration.NewExporter(store, &buf)

			// Export subgraph
			if err := exporter.ExportSubgraph([]string{id1}, tt.depth); err != nil {
				t.Fatalf("Error exporting subgraph: %v", err)
			}

			// Read the tar archive
			tr := tar.NewReader(&buf)

			// Track what we've found
			foundManifest := false
			foundNodes := 0
			foundEdges := 0
			foundChunks := 0
			foundContent := make([][]byte, 0)

			// Read each entry
			for {
				header, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("Error reading tar: %v", err)
				}

				// Read the file content
				content := make([]byte, header.Size)
				if _, err := io.ReadFull(tr, content); err != nil {
					t.Fatalf("Error reading content: %v", err)
				}

				switch {
				case header.Name == "manifest.json":
					foundManifest = true
					var manifest migration.ExportManifest
					if err := json.Unmarshal(content, &manifest); err != nil {
						t.Errorf("Error parsing manifest: %v", err)
					}
					if manifest.Nodes != tt.wantNodes {
						t.Errorf("Expected %d nodes in manifest, got %d", tt.wantNodes, manifest.Nodes)
					}
					if manifest.Edges != tt.wantEdges {
						t.Errorf("Expected %d edges in manifest, got %d", tt.wantEdges, manifest.Edges)
					}
					if manifest.Chunks != tt.wantChunks {
						t.Errorf("Expected %d chunks in manifest, got %d", tt.wantChunks, manifest.Chunks)
					}

				case filepath.Dir(header.Name) == "nodes":
					foundNodes++

				case filepath.Dir(header.Name) == "edges":
					foundEdges++

				case filepath.Dir(header.Name) == "chunks":
					foundChunks++
					foundContent = append(foundContent, content)
				}
			}

			// Verify we found everything
			if !foundManifest {
				t.Error("Manifest not found in export")
			}
			if foundNodes != tt.wantNodes {
				t.Errorf("Expected %d nodes in export, found %d", tt.wantNodes, foundNodes)
			}
			if foundEdges != tt.wantEdges {
				t.Errorf("Expected %d edges in export, found %d", tt.wantEdges, foundEdges)
			}
			if foundChunks != tt.wantChunks {
				t.Errorf("Expected %d chunks in export, found %d", tt.wantChunks, foundChunks)
			}

			// Verify chunk content
			for _, want := range tt.wantContent {
				found := false
				for _, got := range foundContent {
					if bytes.Equal(want, got) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Content %q not found in export", want)
				}
			}
		})
	}
}
