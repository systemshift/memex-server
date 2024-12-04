package test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex"
)

func TestCommands(t *testing.T) {
	// Create temporary directory for tests
	tmpDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to temp directory: %v", err)
	}
	defer func() {
		os.Chdir(originalWd)
		memex.CloseRepository()
	}()

	t.Run("Repository Management", func(t *testing.T) {
		// Test init command
		if err := memex.InitCommand("test.mx"); err != nil {
			t.Fatalf("init command: %v", err)
		}

		// Verify repository file exists
		if _, err := os.Stat("test.mx"); os.IsNotExist(err) {
			t.Error("repository file not created")
		}

		// Test status command
		if err := memex.StatusCommand(); err != nil {
			t.Errorf("status command: %v", err)
		}

		// Test close and connect
		if err := memex.CloseRepository(); err != nil {
			t.Errorf("closing repository: %v", err)
		}

		if err := memex.ConnectCommand("test.mx"); err != nil {
			t.Errorf("connect command: %v", err)
		}
	})

	t.Run("Node Operations", func(t *testing.T) {
		// Create test file
		testFile := filepath.Join(tmpDir, "test.txt")
		content := []byte("test content")
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("creating test file: %v", err)
		}

		// Test add command
		if err := memex.AddCommand(testFile); err != nil {
			t.Errorf("add command: %v", err)
		}

		// Test delete command (need to get node ID first)
		repo, err := memex.GetRepository()
		if err != nil {
			t.Fatalf("getting repository: %v", err)
		}

		// Add a node to delete
		id, err := repo.AddNode([]byte("delete me"), "test", nil)
		if err != nil {
			t.Fatalf("adding node: %v", err)
		}

		if err := memex.DeleteCommand(id); err != nil {
			t.Errorf("delete command: %v", err)
		}

		// Verify node was deleted
		if _, err := repo.GetNode(id); err == nil {
			t.Error("node not deleted")
		}
	})

	t.Run("Link Operations", func(t *testing.T) {
		repo, err := memex.GetRepository()
		if err != nil {
			t.Fatalf("getting repository: %v", err)
		}

		// Create two nodes to link
		id1, err := repo.AddNode([]byte("node1"), "test", nil)
		if err != nil {
			t.Fatalf("adding node 1: %v", err)
		}

		id2, err := repo.AddNode([]byte("node2"), "test", nil)
		if err != nil {
			t.Fatalf("adding node 2: %v", err)
		}

		// Test link command
		if err := memex.LinkCommand(id1, id2, "test", "test note"); err != nil {
			t.Errorf("link command: %v", err)
		}

		// Test links command
		if err := memex.LinksCommand(id1); err != nil {
			t.Errorf("links command: %v", err)
		}
	})

	t.Run("Import Export", func(t *testing.T) {
		// Create some content to export
		repo, err := memex.GetRepository()
		if err != nil {
			t.Fatalf("getting repository: %v", err)
		}

		// Create nodes with valid hex IDs
		content1 := []byte("export test 1")
		hash1 := sha256.Sum256(content1)
		id1 := hex.EncodeToString(hash1[:])

		if err := repo.AddNodeWithID(id1, content1, "test", map[string]interface{}{
			"title": "Node 1",
		}); err != nil {
			t.Fatalf("adding node 1: %v", err)
		}

		content2 := []byte("export test 2")
		hash2 := sha256.Sum256(content2)
		id2 := hex.EncodeToString(hash2[:])

		if err := repo.AddNodeWithID(id2, content2, "test", map[string]interface{}{
			"title": "Node 2",
		}); err != nil {
			t.Fatalf("adding node 2: %v", err)
		}

		repo.AddLink(id1, id2, "test", nil)

		// Test export command
		exportPath := filepath.Join(tmpDir, "export.tar")
		if err := memex.ExportCommand(exportPath); err != nil {
			t.Errorf("export command: %v", err)
		}

		// Verify export file exists
		if _, err := os.Stat(exportPath); os.IsNotExist(err) {
			t.Error("export file not created")
		}

		// Create new repository for import
		if err := memex.CloseRepository(); err != nil {
			t.Fatalf("closing repository: %v", err)
		}

		if err := memex.InitCommand("import.mx"); err != nil {
			t.Fatalf("creating import repository: %v", err)
		}

		// Test import command with prefix
		if err := memex.ImportCommand(exportPath, "--prefix", "imported-"); err != nil {
			t.Errorf("import command: %v", err)
		}

		// Verify imported content
		repo, err = memex.GetRepository()
		if err != nil {
			t.Fatalf("getting repository: %v", err)
		}

		// Check for imported nodes with prefix
		importedId1 := "imported-" + id1
		if _, err := repo.GetNode(importedId1); err != nil {
			t.Errorf("imported node not found: %v", err)
		}

		// Check for imported links
		links, err := repo.GetLinks(importedId1)
		if err != nil {
			t.Errorf("getting imported links: %v", err)
		}
		if len(links) != 1 {
			t.Errorf("expected 1 imported link got %d", len(links))
		}
	})

	t.Run("Auto Repository Detection", func(t *testing.T) {
		// Close current repository
		if err := memex.CloseRepository(); err != nil {
			t.Fatalf("closing repository: %v", err)
		}

		// Create repository file
		repoName := "auto.mx"
		if err := memex.InitCommand(repoName); err != nil {
			t.Fatalf("creating repository: %v", err)
		}

		// Close and try auto-detection
		if err := memex.CloseRepository(); err != nil {
			t.Fatalf("closing repository: %v", err)
		}

		if err := memex.OpenRepository(); err != nil {
			t.Errorf("auto-detecting repository: %v", err)
		}

		// Verify correct repository was opened
		repo, err := memex.GetRepository()
		if err != nil {
			t.Fatalf("getting repository: %v", err)
		}

		// Add a node to verify repository is working
		if _, err := repo.AddNode([]byte("test"), "test", nil); err != nil {
			t.Errorf("adding node to auto-detected repository: %v", err)
		}
	})

	t.Run("Error Cases", func(t *testing.T) {
		// Test init without name
		if err := memex.InitCommand(); err == nil {
			t.Error("init without name should fail")
		}

		// Test connect without path
		if err := memex.ConnectCommand(); err == nil {
			t.Error("connect without path should fail")
		}

		// Test add without path
		if err := memex.AddCommand(); err == nil {
			t.Error("add without path should fail")
		}

		// Test delete without ID
		if err := memex.DeleteCommand(); err == nil {
			t.Error("delete without ID should fail")
		}

		// Test link with insufficient arguments
		if err := memex.LinkCommand("source"); err == nil {
			t.Error("link with only source should fail")
		}

		// Test links without ID
		if err := memex.LinksCommand(); err == nil {
			t.Error("links without ID should fail")
		}

		// Test export without path
		if err := memex.ExportCommand(); err == nil {
			t.Error("export without path should fail")
		}

		// Test import without path
		if err := memex.ImportCommand(); err == nil {
			t.Error("import without path should fail")
		}

		// Test operations without repository
		if err := memex.CloseRepository(); err != nil {
			t.Fatalf("closing repository: %v", err)
		}

		if err := memex.StatusCommand(); err == nil {
			t.Error("status without repository should fail")
		}

		if err := memex.AddCommand("test.txt"); err == nil {
			t.Error("add without repository should fail")
		}
	})
}
