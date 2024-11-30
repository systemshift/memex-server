package test

import (
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex"
)

// TestEditor tests the editor functionality
func TestEditor(t *testing.T) {
	t.Run("New Editor", func(t *testing.T) {
		// Create temp dir for test
		tmpDir := t.TempDir()
		repoPath := filepath.Join(tmpDir, "test_repo.mx")

		editor := memex.NewEditor(repoPath)
		if editor == nil {
			t.Fatal("NewEditor returned nil")
		}

		// Verify temp file is created
		tmpFile := editor.GetTempFile()
		if tmpFile == "" {
			t.Fatal("No temp file created")
		}

		// Verify temp file exists
		if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
			t.Fatal("Temp file does not exist")
		}

		// Clean up
		editor.Close()

		// Verify temp file is cleaned up
		if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
			t.Fatal("Temp file not cleaned up")
		}
	})

	t.Run("Write and Read Content", func(t *testing.T) {
		// Create temp dir for test
		tmpDir := t.TempDir()
		repoPath := filepath.Join(tmpDir, "test_repo.mx")

		editor := memex.NewEditor(repoPath)
		if editor == nil {
			t.Fatal("NewEditor returned nil")
		}
		defer editor.Close()

		// Write some content
		content := []byte("Test content\nLine 2")
		if err := editor.WriteContent(content); err != nil {
			t.Fatalf("Writing content: %v", err)
		}

		// Read it back
		readContent, err := editor.ReadContent()
		if err != nil {
			t.Fatalf("Reading content: %v", err)
		}

		// Verify content matches
		if string(readContent) != string(content) {
			t.Errorf("Content mismatch:\ngot:  %q\nwant: %q", readContent, content)
		}
	})
}

// TestEditorHelpers tests the helper functions
func TestEditorHelpers(t *testing.T) {
	tests := []struct {
		name     string
		input    byte
		wantCtrl byte
		isCntrl  bool
	}{
		{
			name:     "Regular character",
			input:    'a',
			wantCtrl: 0x01, // Ctrl-A
			isCntrl:  false,
		},
		{
			name:     "Control character",
			input:    0x01, // Ctrl-A
			wantCtrl: 0x01,
			isCntrl:  true,
		},
		{
			name:     "Space",
			input:    ' ',
			wantCtrl: 0x00,
			isCntrl:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test ctrl function
			if got := memex.Ctrl(tt.input); got != tt.wantCtrl {
				t.Errorf("ctrl(%v) = %v, want %v", tt.input, got, tt.wantCtrl)
			}

			// Test iscntrl function
			if got := memex.IsCntrl(tt.input); got != tt.isCntrl {
				t.Errorf("iscntrl(%v) = %v, want %v", tt.input, got, tt.isCntrl)
			}
		})
	}
}
