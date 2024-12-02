package test

import (
	"testing"
)

func TestIntegration(t *testing.T) {
	// Create test repository
	repo, tmpDir := CreateTestRepo(t)
	defer CleanupTestRepo(t, repo, tmpDir)

	t.Run("Initialize Repository", func(t *testing.T) {
		AssertFileExists(t, repo.Path())
	})

	t.Run("Add First File", func(t *testing.T) {
		// Add first file
		content := []byte("Test content 1")
		firstID := AddTestFile(t, repo, "test1.txt", content)

		// Verify file was added
		AssertNodeExists(t, repo, firstID)
	})

	t.Run("Add Second File", func(t *testing.T) {
		// Add second file
		content := []byte("Test content 2")
		secondID := AddTestFile(t, repo, "test2.txt", content)

		// Verify file was added
		AssertNodeExists(t, repo, secondID)
	})

	t.Run("Create Link Between Files", func(t *testing.T) {
		// Create link
		sourceID := AddTestFile(t, repo, "source.txt", []byte("Source"))
		targetID := AddTestFile(t, repo, "target.txt", []byte("Target"))
		CreateTestLink(t, repo, sourceID, targetID, "references")

		// Verify link exists
		AssertLinkExists(t, repo, sourceID, targetID, "references")

		// Get links
		links, err := repo.GetLinks(sourceID)
		if err != nil {
			t.Fatalf("Error getting links: %v", err)
		}

		// Verify link count
		if len(links) != 1 {
			t.Errorf("Expected 1 link got %d", len(links))
		}
	})

	t.Run("Delete File", func(t *testing.T) {
		// Add file to delete
		content := []byte("Delete me")
		id := AddTestFile(t, repo, "delete.txt", content)

		// Create self-referential link
		CreateTestLink(t, repo, id, id, "references")

		// Delete file
		if err := repo.DeleteNode(id); err != nil {
			t.Fatalf("Error deleting node: %v", err)
		}

		// Delete associated links
		if err := repo.DeleteLink(id, id, "references"); err != nil {
			t.Fatalf("Error deleting link: %v", err)
		}

		// Verify file and links are gone
		AssertNodeNotExists(t, repo, id)
		AssertLinkNotExists(t, repo, id, id, "references")
	})

	t.Run("Reopen Repository", func(t *testing.T) {
		// Close repository
		if err := repo.Close(); err != nil {
			t.Fatalf("Error closing repository: %v", err)
		}

		// Reopen repository
		repo = OpenTestRepo(t, repo.Path())

		// Add file after reopening
		content := []byte("New content")
		id := AddTestFile(t, repo, "new.txt", content)

		// Verify file exists
		AssertNodeExists(t, repo, id)

		// Delete file
		if err := repo.DeleteNode(id); err != nil {
			t.Fatalf("Error deleting node: %v", err)
		}

		// Verify file is gone
		AssertNodeNotExists(t, repo, id)
	})
}
