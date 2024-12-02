package test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"memex/internal/memex/storage"
)

var (
	closeMutex sync.Mutex
)

// CreateTestRepo creates a temporary repository for testing
func CreateTestRepo(t *testing.T) (*storage.MXStore, string) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "memex-test-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}

	// Create repository path
	repoPath := filepath.Join(tmpDir, "test.mx")

	// Create repository
	repo, err := storage.CreateMX(repoPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Error creating repository: %v", err)
	}

	return repo, tmpDir
}

// CleanupTestRepo cleans up a test repository
func CleanupTestRepo(t *testing.T, repo *storage.MXStore, tmpDir string) {
	closeMutex.Lock()
	defer closeMutex.Unlock()

	// Close repository if not already closed
	if repo != nil {
		if err := repo.Close(); err != nil {
			// Only report error if it's not "file already closed"
			if !os.IsNotExist(err) && err.Error() != "file already closed" {
				t.Errorf("Error closing repository: %v", err)
			}
		}
	}

	// Remove temporary directory
	if err := os.RemoveAll(tmpDir); err != nil {
		t.Errorf("Error removing temp dir: %v", err)
	}
}

// OpenTestRepo opens an existing test repository
func OpenTestRepo(t *testing.T, path string) *storage.MXStore {
	repo, err := storage.OpenMX(path)
	if err != nil {
		t.Fatalf("Error opening repository: %v", err)
	}
	return repo
}

// CreateTempDir creates a temporary directory for testing
func CreateTempDir(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "memex-test-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	return tmpDir
}

// AssertFileExists checks if a file exists
func AssertFileExists(t *testing.T, path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("File %s does not exist", path)
	}
}

// AssertNodeExists checks if a node exists
func AssertNodeExists(t *testing.T, repo *storage.MXStore, id string) {
	node, err := repo.GetNode(id)
	if err != nil {
		t.Errorf("Node %s does not exist: %v", id, err)
	}
	if node.ID != id {
		t.Errorf("Node ID mismatch: got %s want %s", node.ID, id)
	}
}

// AssertNodeNotExists checks if a node does not exist
func AssertNodeNotExists(t *testing.T, repo *storage.MXStore, id string) {
	if _, err := repo.GetNode(id); err == nil {
		t.Errorf("Node %s exists when it should not", id)
	}
}

// AssertLinkExists checks if a link exists
func AssertLinkExists(t *testing.T, repo *storage.MXStore, sourceID, targetID, linkType string) {
	links, err := repo.GetLinks(sourceID)
	if err != nil {
		t.Errorf("Error getting links: %v", err)
		return
	}

	for _, link := range links {
		if link.Source == sourceID && link.Target == targetID && link.Type == linkType {
			return
		}
	}

	t.Errorf("Link not found: %s -[%s]-> %s", sourceID, linkType, targetID)
}

// AssertLinkNotExists checks if a link does not exist
func AssertLinkNotExists(t *testing.T, repo *storage.MXStore, sourceID, targetID, linkType string) {
	links, err := repo.GetLinks(sourceID)
	if err != nil {
		t.Errorf("Error getting links: %v", err)
		return
	}

	for _, link := range links {
		if link.Source == sourceID && link.Target == targetID && link.Type == linkType {
			t.Errorf("Link exists when it should not: %s -[%s]-> %s", sourceID, linkType, targetID)
			return
		}
	}
}

// AddTestFile adds a test file to the repository
func AddTestFile(t *testing.T, repo *storage.MXStore, filename string, content []byte) string {
	meta := map[string]any{
		"filename": filename,
		"type":     "file",
	}
	id, err := repo.AddNode(content, "file", meta)
	if err != nil {
		t.Fatalf("Error adding file: %v", err)
	}
	return id
}

// CreateTestLink creates a test link between nodes
func CreateTestLink(t *testing.T, repo *storage.MXStore, sourceID, targetID, linkType string) {
	if err := repo.AddLink(sourceID, targetID, linkType, nil); err != nil {
		t.Fatalf("Error creating link: %v", err)
	}
}

// CreateTestData creates test data in a repository
func CreateTestData(t *testing.T, repo *storage.MXStore) {
	// Create root node
	content := []byte("Root node")
	meta := map[string]any{
		"title": "Root",
		"tags":  []string{"test", "root"},
	}
	rootID, err := repo.AddNode(content, "note", meta)
	if err != nil {
		t.Fatalf("Error creating root node: %v", err)
	}

	// Create child node
	childContent := []byte("Child node")
	childMeta := map[string]any{
		"title": "Child",
		"tags":  []string{"test", "child"},
	}
	childID, err := repo.AddNode(childContent, "note", childMeta)
	if err != nil {
		t.Fatalf("Error creating child node: %v", err)
	}

	// Create grandchild node
	grandchildContent := []byte("Grandchild node")
	grandchildMeta := map[string]any{
		"title": "Grandchild",
		"tags":  []string{"test", "grandchild"},
	}
	grandchildID, err := repo.AddNode(grandchildContent, "note", grandchildMeta)
	if err != nil {
		t.Fatalf("Error creating grandchild node: %v", err)
	}

	// Create links
	if err := repo.AddLink(rootID, childID, "contains", nil); err != nil {
		t.Fatalf("Error creating root->child link: %v", err)
	}
	if err := repo.AddLink(childID, grandchildID, "contains", nil); err != nil {
		t.Fatalf("Error creating child->grandchild link: %v", err)
	}

	fmt.Printf("Repository: %s\n", repo.Path())
	fmt.Printf("Nodes: %d\n", len(repo.Nodes()))
}
