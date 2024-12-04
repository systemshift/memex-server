package test

import (
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex"
)

func TestEditor(t *testing.T) {
	t.Run("Editor Creation", func(t *testing.T) {
		editor := memex.NewEditor("test_repo")
		if editor == nil {
			t.Fatal("editor creation failed")
		}
		defer editor.Close()

		// Check temp file creation
		tempFile := editor.GetTempFile()
		if tempFile == "" {
			t.Error("no temp file created")
		}
		if _, err := os.Stat(tempFile); os.IsNotExist(err) {
			t.Error("temp file does not exist")
		}

		// Verify temp file location
		if !filepath.IsAbs(tempFile) {
			t.Error("temp file path is not absolute")
		}
		if filepath.Dir(tempFile) != os.TempDir() {
			t.Error("temp file not in system temp directory")
		}
	})

	t.Run("Editor Cleanup", func(t *testing.T) {
		editor := memex.NewEditor("test_repo")
		if editor == nil {
			t.Fatal("editor creation failed")
		}

		tempFile := editor.GetTempFile()
		editor.Close()

		// Verify temp file is cleaned up
		if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
			t.Error("temp file not cleaned up")
		}
	})

	t.Run("Content Operations", func(t *testing.T) {
		editor := memex.NewEditor("test_repo")
		if editor == nil {
			t.Fatal("editor creation failed")
		}
		defer editor.Close()

		// Test writing content
		content := []byte("test content")
		if err := editor.WriteContent(content); err != nil {
			t.Fatalf("writing content: %v", err)
		}

		// Test reading content
		read, err := editor.ReadContent()
		if err != nil {
			t.Fatalf("reading content: %v", err)
		}

		if string(read) != string(content) {
			t.Errorf("content mismatch:\ngot: %q\nwant: %q", read, content)
		}
	})

	t.Run("Control Characters", func(t *testing.T) {
		// Test Ctrl function
		if memex.Ctrl('q') != 17 { // Ctrl-Q
			t.Error("wrong control character value for Ctrl-Q")
		}
		if memex.Ctrl('s') != 19 { // Ctrl-S
			t.Error("wrong control character value for Ctrl-S")
		}

		// Test IsCntrl function
		controlChars := []byte{
			memex.Ctrl('q'),
			memex.Ctrl('s'),
			0,   // NUL
			127, // DEL
			7,   // BEL
			13,  // CR
			10,  // LF
			9,   // TAB
			27,  // ESC
		}
		for _, ch := range controlChars {
			if !memex.IsCntrl(ch) {
				t.Errorf("character %d should be recognized as control character", ch)
			}
		}

		regularChars := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
		for _, ch := range regularChars {
			if memex.IsCntrl(ch) {
				t.Errorf("character %c should not be recognized as control character", ch)
			}
		}
	})

	t.Run("File Permissions", func(t *testing.T) {
		editor := memex.NewEditor("test_repo")
		if editor == nil {
			t.Fatal("editor creation failed")
		}
		defer editor.Close()

		// Write some content
		content := []byte("test content")
		if err := editor.WriteContent(content); err != nil {
			t.Fatalf("writing content: %v", err)
		}

		// Check file permissions
		info, err := os.Stat(editor.GetTempFile())
		if err != nil {
			t.Fatalf("getting file info: %v", err)
		}

		// On Unix systems, temp files should be readable/writable by owner
		// This is more secure than 0644 for temporary files
		expectedPerm := os.FileMode(0600)
		if info.Mode().Perm() != expectedPerm {
			t.Errorf("wrong file permissions: got %o want %o", info.Mode().Perm(), expectedPerm)
		}
	})

	t.Run("Empty Content", func(t *testing.T) {
		editor := memex.NewEditor("test_repo")
		if editor == nil {
			t.Fatal("editor creation failed")
		}
		defer editor.Close()

		// Read without writing should succeed with empty content
		content, err := editor.ReadContent()
		if err != nil {
			t.Fatalf("reading empty content: %v", err)
		}
		if len(content) != 0 {
			t.Errorf("expected empty content, got %q", content)
		}
	})

	t.Run("Invalid Operations", func(t *testing.T) {
		editor := memex.NewEditor("test_repo")
		if editor == nil {
			t.Fatal("editor creation failed")
		}

		// Close editor
		editor.Close()

		// Operations after close should fail
		if err := editor.WriteContent([]byte("test")); err == nil {
			t.Error("expected error writing to closed editor")
		}
		if _, err := editor.ReadContent(); err == nil {
			t.Error("expected error reading from closed editor")
		}
	})
}
