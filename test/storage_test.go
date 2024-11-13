package test

import (
	"bytes"
	"testing"

	"memex/internal/memex/storage"
)

func TestStorage(t *testing.T) {
	// Create temporary directory for testing
	tmpDir := CreateTempDir(t)

	t.Run("Repository Operations", func(t *testing.T) {
		// Initialize repository
		repo, err := storage.NewRepository(tmpDir)
		if err != nil {
			t.Fatalf("NewRepository failed: %v", err)
		}

		// Test adding content
		content := []byte("test content")
		meta := map[string]any{
			"title": "Test Object",
			"tags":  []string{"test", "example"},
		}

		id, err := repo.Add(content, "text", meta)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		// Test retrieving content
		obj, err := repo.Get(id)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if !bytes.Equal(obj.Content, content) {
			t.Errorf("Content mismatch.\nGot: %q\nWant: %q", obj.Content, content)
		}

		if obj.Type != "text" {
			t.Errorf("Type mismatch.\nGot: %q\nWant: %q", obj.Type, "text")
		}

		if obj.Meta["title"] != "Test Object" {
			t.Errorf("Metadata mismatch.\nGot: %v\nWant: %v", obj.Meta["title"], "Test Object")
		}

		// Test updating content
		newContent := []byte("updated content")
		err = repo.Update(id, newContent)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		// Verify update
		obj, err = repo.Get(id)
		if err != nil {
			t.Fatalf("Get after update failed: %v", err)
		}

		if !bytes.Equal(obj.Content, newContent) {
			t.Errorf("Updated content mismatch.\nGot: %q\nWant: %q", obj.Content, newContent)
		}

		if obj.Version != 2 {
			t.Errorf("Version not incremented.\nGot: %d\nWant: %d", obj.Version, 2)
		}
	})

	t.Run("Version Operations", func(t *testing.T) {
		repo, _ := storage.NewRepository(tmpDir)

		// Add content with multiple versions
		id, _ := repo.Add([]byte("version 1"), "text", nil)
		repo.Update(id, []byte("version 2"))
		repo.Update(id, []byte("version 3"))

		// List versions
		versions, err := repo.ListVersions(id)
		if err != nil {
			t.Fatalf("ListVersions failed: %v", err)
		}

		if len(versions) != 3 {
			t.Errorf("Expected 3 versions, got %d", len(versions))
		}

		// Get specific version
		v1, err := repo.GetVersion(id, 1)
		if err != nil {
			t.Fatalf("GetVersion failed: %v", err)
		}

		if !bytes.Equal(v1.Content, []byte("version 1")) {
			t.Errorf("Version 1 content mismatch.\nGot: %q\nWant: %q", v1.Content, "version 1")
		}
	})

	t.Run("Link Operations", func(t *testing.T) {
		repo, _ := storage.NewRepository(tmpDir)

		// Create two objects to link
		id1, _ := repo.Add([]byte("object 1"), "text", nil)
		id2, _ := repo.Add([]byte("object 2"), "text", nil)

		// Create link
		linkMeta := map[string]any{"note": "test link"}
		err := repo.Link(id1, id2, "references", linkMeta)
		if err != nil {
			t.Fatalf("Link failed: %v", err)
		}

		// Get links
		links, err := repo.GetLinks(id1)
		if err != nil {
			t.Fatalf("GetLinks failed: %v", err)
		}

		if len(links) != 1 {
			t.Errorf("Expected 1 link, got %d", len(links))
		}

		if links[0].Source != id1 || links[0].Target != id2 {
			t.Errorf("Link mismatch.\nGot: %s -> %s\nWant: %s -> %s",
				links[0].Source, links[0].Target, id1, id2)
		}

		// Test unlinking
		err = repo.Unlink(id1, id2)
		if err != nil {
			t.Fatalf("Unlink failed: %v", err)
		}

		links, _ = repo.GetLinks(id1)
		if len(links) != 0 {
			t.Errorf("Expected no links after unlink, got %d", len(links))
		}
	})

	t.Run("Search Operations", func(t *testing.T) {
		repo, _ := storage.NewRepository(tmpDir)

		// Add objects with different types and metadata
		repo.Add([]byte("doc 1"), "document", map[string]any{
			"tags": []string{"work"},
		})
		repo.Add([]byte("doc 2"), "document", map[string]any{
			"tags": []string{"personal"},
		})
		repo.Add([]byte("note 1"), "note", map[string]any{
			"tags": []string{"work"},
		})

		// Test finding by type
		docs := repo.FindByType("document")
		if len(docs) != 2 {
			t.Errorf("Expected 2 documents, got %d", len(docs))
		}

		// Test searching by metadata
		results := repo.Search(map[string]any{
			"tags": []string{"work"},
		})
		if len(results) != 2 {
			t.Errorf("Expected 2 objects with 'work' tag, got %d", len(results))
		}
	})

	t.Run("Delete Operations", func(t *testing.T) {
		repo, _ := storage.NewRepository(tmpDir)

		// Create object and link
		id1, _ := repo.Add([]byte("object 1"), "text", nil)
		id2, _ := repo.Add([]byte("object 2"), "text", nil)
		repo.Link(id1, id2, "references", nil)

		// Delete object
		err := repo.Delete(id1)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify object is gone
		_, err = repo.Get(id1)
		if err == nil {
			t.Error("Expected error getting deleted object")
		}

		// Verify links are gone
		links, _ := repo.GetLinks(id2)
		if len(links) != 0 {
			t.Errorf("Expected no links after delete, got %d", len(links))
		}
	})
}
