package test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex/migration"
	"memex/internal/memex/storage"
)

func TestImport(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "memex-import-test-*")
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

	// Add test content to source
	content1 := []byte("node1")
	meta1 := map[string]any{
		"filename": "n1.txt",
		"type":     "file",
	}
	id1, err := srcRepo.AddNode(content1, "file", meta1)
	if err != nil {
		t.Fatalf("Error adding first node: %v", err)
	}

	content2 := []byte("node2")
	meta2 := map[string]any{
		"filename": "n2.txt",
		"type":     "file",
	}
	id2, err := srcRepo.AddNode(content2, "file", meta2)
	if err != nil {
		t.Fatalf("Error adding second node: %v", err)
	}

	// Create link
	err = srcRepo.AddLink(id1, id2, "references", map[string]any{"note": "test link"})
	if err != nil {
		t.Fatalf("Error creating link: %v", err)
	}

	// Export source repository
	var exportBuf bytes.Buffer
	exporter := migration.NewExporter(srcRepo, &exportBuf)
	if err := exporter.Export(); err != nil {
		t.Fatalf("Error exporting: %v", err)
	}

	// Create destination repository
	dstPath := filepath.Join(tmpDir, "dest.mx")
	dstRepo, err := storage.CreateMX(dstPath)
	if err != nil {
		t.Fatalf("Error creating destination repository: %v", err)
	}

	// Import into destination
	importBuf := bytes.NewReader(exportBuf.Bytes())
	importer := migration.NewImporter(dstRepo, importBuf, migration.ImportOptions{
		OnConflict: migration.Skip,
		Merge:      false,
	})
	if err := importer.Import(); err != nil {
		t.Fatalf("Error importing: %v", err)
	}

	// Verify imported content
	srcNodes := srcRepo.Nodes()
	dstNodes := dstRepo.Nodes()

	if len(srcNodes) != len(dstNodes) {
		t.Errorf("Node count mismatch: source has %d, destination has %d", len(srcNodes), len(dstNodes))
	}

	// Get source links
	srcLinks, err := srcRepo.GetLinks(id1)
	if err != nil {
		t.Fatalf("Error getting source links: %v", err)
	}

	// Find corresponding node in destination
	var dstID1 string
	for _, node := range dstNodes {
		if filename, ok := node.Meta["filename"].(string); ok && filename == "n1.txt" {
			dstID1 = node.ID
			break
		}
	}

	if dstID1 == "" {
		t.Fatal("Could not find imported node1")
	}

	// Get destination links
	dstLinks, err := dstRepo.GetLinks(dstID1)
	if err != nil {
		t.Fatalf("Error getting destination links: %v", err)
	}

	if len(srcLinks) != len(dstLinks) {
		t.Errorf("Link count mismatch: source has %d, destination has %d", len(srcLinks), len(dstLinks))
	}

	// Compare link metadata
	if len(srcLinks) > 0 && len(dstLinks) > 0 {
		srcNote, ok := srcLinks[0].Meta["note"].(string)
		if !ok {
			t.Error("Source link missing note")
		}
		dstNote, ok := dstLinks[0].Meta["note"].(string)
		if !ok {
			t.Error("Destination link missing note")
		}
		if srcNote != dstNote {
			t.Errorf("Link note mismatch: source has %q, destination has %q", srcNote, dstNote)
		}
	}

	// Verify content
	for _, node := range dstNodes {
		contentHash, ok := node.Meta["content"].(string)
		if !ok {
			t.Error("Node missing content hash")
			continue
		}

		content, err := dstRepo.ReconstructContent(contentHash)
		if err != nil {
			t.Errorf("Error reconstructing content: %v", err)
			continue
		}

		filename, ok := node.Meta["filename"].(string)
		if !ok {
			t.Error("Node missing filename")
			continue
		}

		var expectedContent []byte
		switch filename {
		case "n1.txt":
			expectedContent = content1
		case "n2.txt":
			expectedContent = content2
		default:
			t.Errorf("Unexpected filename: %s", filename)
			continue
		}

		if !bytes.Equal(content, expectedContent) {
			t.Errorf("Content mismatch for %s: expected %q, got %q", filename, expectedContent, content)
		}
	}
}
