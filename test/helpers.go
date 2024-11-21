package test

import (
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex/core"
	"memex/internal/memex/storage"
)

// CreateTempDir creates a temporary directory for testing
func CreateTempDir(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "memex-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	return tmpDir
}

// AssertFileExists verifies that a file exists
func AssertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Errorf("File does not exist: %s", path)
	}
}

// AssertFileNotExists verifies that a file does not exist
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("File exists when it should not: %s", path)
	}
}

// CreateTestRepo creates a temporary repository for testing
func CreateTestRepo(t *testing.T) (string, *storage.MXStore) {
	t.Helper()

	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "memex-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	repoPath := filepath.Join(tmpDir, "test.mx")
	store, err := storage.CreateMX(repoPath)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})

	return repoPath, store
}

// OpenTestRepo opens an existing test repository
func OpenTestRepo(t *testing.T, path string) *storage.MXStore {
	t.Helper()

	store, err := storage.OpenMX(path)
	if err != nil {
		t.Fatalf("Failed to open repository: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})

	return store
}

// AddTestFile adds a test file to the repository
func AddTestFile(t *testing.T, store *storage.MXStore, name string, content []byte) string {
	t.Helper()

	id, err := store.AddNode(content, "file", map[string]any{
		"filename": name,
	})
	if err != nil {
		t.Fatalf("Failed to add test file: %v", err)
	}

	return id
}

// CreateTestLink creates a test link between nodes
func CreateTestLink(t *testing.T, store *storage.MXStore, source, target, linkType string, meta map[string]any) {
	t.Helper()

	err := store.AddLink(source, target, linkType, meta)
	if err != nil {
		t.Fatalf("Failed to create test link: %v", err)
	}
}

// AssertNodeExists verifies that a node exists and returns it
func AssertNodeExists(t *testing.T, store *storage.MXStore, id string) core.Node {
	t.Helper()

	node, err := store.GetNode(id)
	if err != nil {
		t.Fatalf("Node %s not found: %v", id, err)
	}

	return node
}

// AssertNodeNotExists verifies that a node does not exist
func AssertNodeNotExists(t *testing.T, store *storage.MXStore, id string) {
	t.Helper()

	_, err := store.GetNode(id)
	if err == nil {
		t.Errorf("Node %s still exists when it should be gone", id)
	}
}

// AssertLinkExists verifies that a link exists between nodes
func AssertLinkExists(t *testing.T, store *storage.MXStore, source, target, linkType string) {
	t.Helper()

	links, err := store.GetLinks(source)
	if err != nil {
		t.Fatalf("Failed to get links: %v", err)
	}

	for _, link := range links {
		if link.Target == target && link.Type == linkType {
			return // Found matching link
		}
	}

	t.Errorf("Link not found: %s -[%s]-> %s", source, linkType, target)
}

// AssertLinkNotExists verifies that a link does not exist between nodes
func AssertLinkNotExists(t *testing.T, store *storage.MXStore, source, target, linkType string) {
	t.Helper()

	links, err := store.GetLinks(source)
	if err != nil {
		t.Fatalf("Failed to get links: %v", err)
	}

	for _, link := range links {
		if link.Target == target && link.Type == linkType {
			t.Errorf("Link exists when it should be gone: %s -[%s]-> %s", source, linkType, target)
			return
		}
	}
}

// WriteTestFile writes a test file with the given content
func WriteTestFile(t *testing.T, path string, content []byte) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		t.Fatalf("Failed to create directories: %v", err)
	}

	err = os.WriteFile(path, content, 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
}

// ReadTestFile reads a test file and returns its content
func ReadTestFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	return content
}
