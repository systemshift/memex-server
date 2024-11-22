package test

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex/migration"
	"memex/internal/memex/storage"
)

func TestImportConflicts(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "memex-import-conflict-test-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source repository
	srcPath := filepath.Join(tmpDir, "source.mx")
	srcRepo, err := storage.CreateMX(srcPath)
	if err != nil {
		t.Fatalf("Error creating source repository: %v", err)
	}

	// Add test content
	content := []byte("test content")
	meta := map[string]any{
		"filename": "test.txt",
		"type":     "file",
	}
	if _, err := srcRepo.AddNode(content, "file", meta); err != nil {
		t.Fatalf("Error adding node to source: %v", err)
	}

	// Export source repository
	var exportBuf bytes.Buffer
	exporter := migration.NewExporter(srcRepo, &exportBuf)
	if err := exporter.Export(); err != nil {
		t.Fatalf("Error exporting: %v", err)
	}

	// Test different conflict strategies
	tests := []struct {
		name          string
		strategy      migration.ConflictStrategy
		prefix        string
		wantNodeCount int
	}{
		{
			name:          "Skip",
			strategy:      migration.Skip,
			wantNodeCount: 1, // Only keeps existing
		},
		{
			name:          "Replace",
			strategy:      migration.Replace,
			wantNodeCount: 1, // Replaces existing
		},
		{
			name:          "Rename",
			strategy:      migration.Rename,
			prefix:        "imported_",
			wantNodeCount: 2, // Keeps both
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create destination repository
			dstPath := filepath.Join(tmpDir, "dest.mx")
			dstRepo, err := storage.CreateMX(dstPath)
			if err != nil {
				t.Fatalf("Error creating destination repository: %v", err)
			}

			// Add same content to destination
			dstID, err := dstRepo.AddNode(content, "file", meta)
			if err != nil {
				t.Fatalf("Error adding node to destination: %v", err)
			}

			// Import with strategy
			importBuf := bytes.NewReader(exportBuf.Bytes())
			importer := migration.NewImporter(dstRepo, importBuf, migration.ImportOptions{
				OnConflict: tt.strategy,
				Prefix:     tt.prefix,
			})
			if err := importer.Import(); err != nil {
				t.Fatalf("Error importing: %v", err)
			}

			// Verify node count
			if len(dstRepo.Nodes()) != tt.wantNodeCount {
				t.Errorf("Node count = %d, want %d", len(dstRepo.Nodes()), tt.wantNodeCount)
			}

			// Verify content
			for _, entry := range dstRepo.Nodes() {
				nodeID := hex.EncodeToString(entry.ID[:])
				node, err := dstRepo.GetNode(nodeID)
				if err != nil {
					t.Errorf("Error getting node: %v", err)
					continue
				}

				contentHash, ok := node.Meta["content"].(string)
				if !ok {
					t.Error("Node missing content hash")
					continue
				}

				nodeContent, err := dstRepo.LoadBlob(contentHash)
				if err != nil {
					t.Errorf("Error loading content: %v", err)
					continue
				}

				if !bytes.Equal(nodeContent, content) {
					t.Errorf("Content mismatch: got %q, want %q", nodeContent, content)
				}
			}

			// Strategy-specific checks
			switch tt.strategy {
			case migration.Skip:
				// Should keep original ID
				if len(dstRepo.Nodes()) > 0 {
					nodeID := hex.EncodeToString(dstRepo.Nodes()[0].ID[:])
					node, err := dstRepo.GetNode(nodeID)
					if err != nil {
						t.Errorf("Error getting node: %v", err)
					} else if node.ID != dstID {
						t.Errorf("Node ID = %s, want %s", node.ID, dstID)
					}
				}

			case migration.Replace:
				// Should have new ID
				if len(dstRepo.Nodes()) > 0 {
					nodeID := hex.EncodeToString(dstRepo.Nodes()[0].ID[:])
					node, err := dstRepo.GetNode(nodeID)
					if err != nil {
						t.Errorf("Error getting node: %v", err)
					} else if node.ID == dstID {
						t.Error("Node ID should have changed")
					}
				}

			case migration.Rename:
				// Should have both nodes with correct content
				foundOriginal := false
				foundImported := false
				for _, entry := range dstRepo.Nodes() {
					nodeID := hex.EncodeToString(entry.ID[:])
					node, err := dstRepo.GetNode(nodeID)
					if err != nil {
						t.Errorf("Error getting node: %v", err)
						continue
					}
					if node.Meta["filename"] == "test.txt" {
						if node.ID == dstID {
							foundOriginal = true
						} else {
							foundImported = true
						}
					}
				}
				if !foundOriginal {
					t.Error("Original node not found")
				}
				if !foundImported {
					t.Error("Imported node not found")
				}
			}
		})
	}
}
