package test

import (
	"bytes"
	"os"
	"testing"

	"memex/internal/memex"
)

func TestCommands(t *testing.T) {
	// Create temporary directory for test repository
	tmpDir, err := os.MkdirTemp("", "memex-test-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize repository
	err = memex.InitCommand(tmpDir)
	if err != nil {
		t.Fatalf("Error initializing repository: %v", err)
	}

	// Change to test directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting current directory: %v", err)
	}
	defer os.Chdir(origDir)

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Error changing to test directory: %v", err)
	}

	// Test adding a file
	testFile := "test.txt"
	content := []byte("Test content")
	err = os.WriteFile(testFile, content, 0644)
	if err != nil {
		t.Fatalf("Error creating test file: %v", err)
	}
	defer os.Remove(testFile)

	err = memex.AddCommand(testFile)
	if err != nil {
		t.Fatalf("Error adding file: %v", err)
	}

	// Get repository to check results directly
	repo, err := memex.GetRepository()
	if err != nil {
		t.Fatalf("Error getting repository: %v", err)
	}

	// Search for added file
	results := repo.Search(map[string]any{
		"filename": "test.txt",
	})

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	// Test showing content
	obj := results[0]
	if !bytes.Equal(obj.Content, content) {
		t.Error("Retrieved content does not match original")
	}

	// Test linking
	content2 := []byte("Another test content")
	testFile2 := "test2.txt"
	err = os.WriteFile(testFile2, content2, 0644)
	if err != nil {
		t.Fatalf("Error creating second test file: %v", err)
	}
	defer os.Remove(testFile2)

	err = memex.AddCommand(testFile2)
	if err != nil {
		t.Fatalf("Error adding second file: %v", err)
	}

	results2 := repo.Search(map[string]any{
		"filename": "test2.txt",
	})
	if len(results2) != 1 {
		t.Fatalf("Expected 1 result for second file, got %d", len(results2))
	}

	// Create link between files
	err = memex.LinkCommand(obj.ID, results2[0].ID, "references", "Test link")
	if err != nil {
		t.Fatalf("Error creating link: %v", err)
	}

	// Only create chunk-level link if both objects have chunks
	if len(obj.Chunks) > 0 && len(results2[0].Chunks) > 0 {
		t.Logf("Creating chunk-level link between objects using chunks %s and %s",
			obj.Chunks[0], results2[0].Chunks[0])

		// Create link between specific chunks
		err = repo.LinkChunks(obj.ID, obj.Chunks[0], results2[0].ID, results2[0].Chunks[0], "references", map[string]any{
			"note": "Link between specific portions",
		})
		if err != nil {
			t.Fatalf("Error creating chunk link: %v", err)
		}

		// Verify links
		links, err := repo.GetLinks(obj.ID)
		if err != nil {
			t.Fatalf("Error getting links: %v", err)
		}

		// Should have two links: one file-level and one chunk-level
		if len(links) != 2 {
			t.Errorf("Expected 2 links, got %d", len(links))
			for i, link := range links {
				t.Logf("Link %d: Source=%s Target=%s Type=%s SourceChunk=%s TargetChunk=%s",
					i, link.Source, link.Target, link.Type, link.SourceChunk, link.TargetChunk)
			}
		}

		// Verify chunk-level link exists
		found := false
		for _, link := range links {
			if link.SourceChunk != "" && link.TargetChunk != "" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Chunk-level link not found")
		}
	} else {
		t.Logf("Skipping chunk-level link test - first object has %d chunks, second has %d chunks",
			len(obj.Chunks), len(results2[0].Chunks))

		// Just verify the file-level link exists
		links, err := repo.GetLinks(obj.ID)
		if err != nil {
			t.Fatalf("Error getting links: %v", err)
		}

		if len(links) != 1 {
			t.Errorf("Expected 1 link, got %d", len(links))
			for i, link := range links {
				t.Logf("Link %d: Source=%s Target=%s Type=%s", i, link.Source, link.Target, link.Type)
			}
		}
	}
}

func TestStatusCommand(t *testing.T) {
	// Create temporary directory for test repository
	tmpDir, err := os.MkdirTemp("", "memex-test-*")
	if err != nil {
		t.Fatalf("Error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize repository
	err = memex.InitCommand(tmpDir)
	if err != nil {
		t.Fatalf("Error initializing repository: %v", err)
	}

	// Change to test directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting current directory: %v", err)
	}
	defer os.Chdir(origDir)

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Error changing to test directory: %v", err)
	}

	// Add some test content
	testFiles := []struct {
		name    string
		content []byte
	}{
		{"test1.txt", []byte("Test content 1")},
		{"test2.txt", []byte("Test content 2")},
	}

	for _, tf := range testFiles {
		err = os.WriteFile(tf.name, tf.content, 0644)
		if err != nil {
			t.Fatalf("Error creating test file: %v", err)
		}
		defer os.Remove(tf.name)

		err = memex.AddCommand(tf.name)
		if err != nil {
			t.Fatalf("Error adding file: %v", err)
		}
	}

	// Test status command
	err = memex.StatusCommand()
	if err != nil {
		t.Fatalf("Error running status command: %v", err)
	}
}
