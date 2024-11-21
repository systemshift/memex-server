package test

import (
	"testing"
)

func TestIntegration(t *testing.T) {
	repoPath, store := CreateTestRepo(t)

	t.Run("Initialize Repository", func(t *testing.T) {
		AssertFileExists(t, repoPath)
	})

	var firstID, secondID string

	t.Run("Add First File", func(t *testing.T) {
		content := []byte("Test content 1")
		firstID = AddTestFile(t, store, "test1.txt", content)

		// Verify node exists
		node := AssertNodeExists(t, store, firstID)
		if filename, ok := node.Meta["filename"].(string); !ok || filename != "test1.txt" {
			t.Errorf("Wrong filename in metadata: %v", node.Meta)
		}
	})

	t.Run("Add Second File", func(t *testing.T) {
		content := []byte("Test content 2")
		secondID = AddTestFile(t, store, "test2.txt", content)

		// Verify node exists
		node := AssertNodeExists(t, store, secondID)
		if filename, ok := node.Meta["filename"].(string); !ok || filename != "test2.txt" {
			t.Errorf("Wrong filename in metadata: %v", node.Meta)
		}
	})

	t.Run("Create Link Between Files", func(t *testing.T) {
		// Create link
		CreateTestLink(t, store, firstID, secondID, "references", map[string]any{
			"note": "Test link",
		})

		// Verify link exists
		AssertLinkExists(t, store, firstID, secondID, "references")

		// Verify link metadata
		links, err := store.GetLinks(firstID)
		if err != nil {
			t.Fatalf("Failed to get links: %v", err)
		}
		if len(links) > 0 {
			link := links[0]
			if note, ok := link.Meta["note"].(string); !ok || note != "Test link" {
				t.Errorf("Wrong note in metadata: %v", link.Meta)
			}
		}
	})

	t.Run("Delete File", func(t *testing.T) {
		// Delete second file
		err := store.DeleteNode(secondID)
		if err != nil {
			t.Fatalf("Failed to delete node: %v", err)
		}

		// Verify node is gone
		AssertNodeNotExists(t, store, secondID)

		// Verify link is gone
		AssertLinkNotExists(t, store, firstID, secondID, "references")
	})

	t.Run("Reopen Repository", func(t *testing.T) {
		// Close and reopen repository
		store.Close()
		store = OpenTestRepo(t, repoPath)

		// Verify first file still exists
		node := AssertNodeExists(t, store, firstID)
		if filename, ok := node.Meta["filename"].(string); !ok || filename != "test1.txt" {
			t.Errorf("Wrong filename in metadata after reopen: %v", node.Meta)
		}

		// Verify second file is still gone
		AssertNodeNotExists(t, store, secondID)

		// Verify link is still gone
		AssertLinkNotExists(t, store, firstID, secondID, "references")
	})
}
