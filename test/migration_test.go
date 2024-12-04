package test

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"memex/internal/memex/core"
	"memex/internal/memex/migration"
)

func TestMigration(t *testing.T) {
	t.Run("Export Empty Repository", func(t *testing.T) {
		repo := NewMockRepository()
		var buf bytes.Buffer
		exporter := migration.NewExporter(repo, &buf)

		if err := exporter.Export(); err != nil {
			t.Fatalf("exporting empty repository: %v", err)
		}

		// Read tar archive
		tr := tar.NewReader(&buf)

		// Should contain only manifest
		header, err := tr.Next()
		if err != nil {
			t.Fatalf("reading tar: %v", err)
		}
		if header.Name != "manifest.json" {
			t.Errorf("expected manifest.json, got %s", header.Name)
		}

		// Parse manifest
		data := make([]byte, header.Size)
		if _, err := io.ReadFull(tr, data); err != nil {
			t.Fatalf("reading manifest: %v", err)
		}

		var manifest migration.ExportManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatalf("parsing manifest: %v", err)
		}

		if manifest.Version != migration.Version {
			t.Errorf("wrong version: got %d, want %d", manifest.Version, migration.Version)
		}
		if manifest.Nodes != 0 {
			t.Errorf("expected 0 nodes, got %d", manifest.Nodes)
		}
		if manifest.Edges != 0 {
			t.Errorf("expected 0 edges, got %d", manifest.Edges)
		}
	})

	t.Run("Export With Content", func(t *testing.T) {
		repo := NewMockRepository()

		// Add some nodes and links
		id1, err := repo.AddNode([]byte("node1"), "test", map[string]interface{}{
			"title": "Node 1",
		})
		if err != nil {
			t.Fatalf("adding node 1: %v", err)
		}

		id2, err := repo.AddNode([]byte("node2"), "test", map[string]interface{}{
			"title": "Node 2",
		})
		if err != nil {
			t.Fatalf("adding node 2: %v", err)
		}

		if err := repo.AddLink(id1, id2, "test", map[string]interface{}{
			"weight": 1.0,
		}); err != nil {
			t.Fatalf("adding link: %v", err)
		}

		var buf bytes.Buffer
		exporter := migration.NewExporter(repo, &buf)

		if err := exporter.Export(); err != nil {
			t.Fatalf("exporting repository: %v", err)
		}

		// Read tar archive
		tr := tar.NewReader(&buf)
		files := make(map[string][]byte)

		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("reading tar: %v", err)
			}

			data := make([]byte, header.Size)
			if _, err := io.ReadFull(tr, data); err != nil {
				t.Fatalf("reading file data: %v", err)
			}

			files[header.Name] = data
		}

		// Check manifest
		manifestData, ok := files["manifest.json"]
		if !ok {
			t.Fatal("manifest.json not found")
		}

		var manifest migration.ExportManifest
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			t.Fatalf("parsing manifest: %v", err)
		}

		if manifest.Nodes != 2 {
			t.Errorf("expected 2 nodes, got %d", manifest.Nodes)
		}
		if manifest.Edges != 1 {
			t.Errorf("expected 1 edge, got %d", manifest.Edges)
		}

		// Check nodes
		nodeCount := 0
		for name := range files {
			if strings.HasPrefix(name, "nodes/") {
				nodeCount++
				var node core.Node
				if err := json.Unmarshal(files[name], &node); err != nil {
					t.Errorf("parsing node %s: %v", name, err)
				}
			}
		}
		if nodeCount != 2 {
			t.Errorf("expected 2 node files, got %d", nodeCount)
		}

		// Check edges
		edgeCount := 0
		for name := range files {
			if strings.HasPrefix(name, "edges/") {
				edgeCount++
				var link core.Link
				if err := json.Unmarshal(files[name], &link); err != nil {
					t.Errorf("parsing edge %s: %v", name, err)
				}
			}
		}
		if edgeCount != 1 {
			t.Errorf("expected 1 edge file, got %d", edgeCount)
		}
	})

	t.Run("Import Export Roundtrip", func(t *testing.T) {
		// Create source repository with content
		sourceRepo := NewMockRepository()
		id1, _ := sourceRepo.AddNode([]byte("node1"), "test", map[string]interface{}{
			"title": "Node 1",
			"tags":  []string{"test", "import"},
		})
		id2, _ := sourceRepo.AddNode([]byte("node2"), "test", map[string]interface{}{
			"title":  "Node 2",
			"weight": 42.0,
		})
		sourceRepo.AddLink(id1, id2, "test", map[string]interface{}{
			"weight": 1.0,
		})

		// Export to buffer
		var buf bytes.Buffer
		exporter := migration.NewExporter(sourceRepo, &buf)
		if err := exporter.Export(); err != nil {
			t.Fatalf("exporting: %v", err)
		}

		// Create destination repository
		destRepo := NewMockRepository()

		// Import from buffer
		importer := migration.NewImporter(destRepo, &buf, migration.ImportOptions{
			OnConflict: migration.Skip,
			Merge:      false,
		})
		if err := importer.Import(); err != nil {
			t.Fatalf("importing: %v", err)
		}

		// Verify nodes were imported
		for id, sourceNode := range sourceRepo.nodes {
			destNode, err := destRepo.GetNode(id)
			if err != nil {
				t.Errorf("getting imported node %s: %v", id, err)
				continue
			}

			if !bytes.Equal(destNode.Content, sourceNode.Content) {
				t.Errorf("node %s content mismatch", id)
			}
			if destNode.Type != sourceNode.Type {
				t.Errorf("node %s type mismatch: got %s, want %s", id, destNode.Type, sourceNode.Type)
			}

			// Compare metadata
			sourceJSON, _ := json.Marshal(sourceNode.Meta)
			destJSON, _ := json.Marshal(destNode.Meta)
			if !bytes.Equal(sourceJSON, destJSON) {
				t.Errorf("node %s metadata mismatch:\ngot: %s\nwant: %s", id, destJSON, sourceJSON)
			}
		}

		// Verify links were imported
		for id, sourceLinks := range sourceRepo.links {
			destLinks, err := destRepo.GetLinks(id)
			if err != nil {
				t.Errorf("getting imported links for %s: %v", id, err)
				continue
			}

			if len(destLinks) != len(sourceLinks) {
				t.Errorf("wrong number of links for node %s: got %d, want %d", id, len(destLinks), len(sourceLinks))
				continue
			}

			for i, sourceLink := range sourceLinks {
				destLink := destLinks[i]
				if destLink.Source != sourceLink.Source {
					t.Errorf("link source mismatch: got %s, want %s", destLink.Source, sourceLink.Source)
				}
				if destLink.Target != sourceLink.Target {
					t.Errorf("link target mismatch: got %s, want %s", destLink.Target, sourceLink.Target)
				}
				if destLink.Type != sourceLink.Type {
					t.Errorf("link type mismatch: got %s, want %s", destLink.Type, sourceLink.Type)
				}

				// Compare metadata
				sourceJSON, _ := json.Marshal(sourceLink.Meta)
				destJSON, _ := json.Marshal(destLink.Meta)
				if !bytes.Equal(sourceJSON, destJSON) {
					t.Errorf("link metadata mismatch:\ngot: %s\nwant: %s", destJSON, sourceJSON)
				}
			}
		}
	})

	t.Run("Import With Prefix", func(t *testing.T) {
		// Create source repository
		sourceRepo := NewMockRepository()
		id1, _ := sourceRepo.AddNode([]byte("node1"), "test", nil)
		id2, _ := sourceRepo.AddNode([]byte("node2"), "test", nil)
		sourceRepo.AddLink(id1, id2, "test", nil)

		// Export to buffer
		var buf bytes.Buffer
		exporter := migration.NewExporter(sourceRepo, &buf)
		if err := exporter.Export(); err != nil {
			t.Fatalf("exporting: %v", err)
		}

		// Import with prefix
		destRepo := NewMockRepository()
		importer := migration.NewImporter(destRepo, &buf, migration.ImportOptions{
			Prefix: "imported-",
		})
		if err := importer.Import(); err != nil {
			t.Fatalf("importing: %v", err)
		}

		// Verify node IDs have prefix
		for sourceID := range sourceRepo.nodes {
			destID := "imported-" + sourceID
			if _, err := destRepo.GetNode(destID); err != nil {
				t.Errorf("node %s not found", destID)
			}
		}

		// Verify link IDs have prefix
		for sourceID, sourceLinks := range sourceRepo.links {
			destID := "imported-" + sourceID
			destLinks, err := destRepo.GetLinks(destID)
			if err != nil {
				t.Errorf("getting links for %s: %v", destID, err)
				continue
			}

			if len(destLinks) == 0 {
				t.Errorf("no links found for node %s", destID)
				continue
			}

			for i, sourceLink := range sourceLinks {
				destLink := destLinks[i]
				expectedSource := "imported-" + sourceLink.Source
				expectedTarget := "imported-" + sourceLink.Target

				if destLink.Source != expectedSource {
					t.Errorf("link source mismatch: got %s, want %s", destLink.Source, expectedSource)
				}
				if destLink.Target != expectedTarget {
					t.Errorf("link target mismatch: got %s, want %s", destLink.Target, expectedTarget)
				}
			}
		}
	})
}
