package test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/systemshift/memex/internal/memex/storage/common"
)

func TestFile(t *testing.T) {
	// Create temporary directory for tests
	tmpDir := t.TempDir()

	t.Run("File Operations", func(t *testing.T) {
		path := filepath.Join(tmpDir, "test.bin")

		// Test file creation
		file, err := common.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("creating file: %v", err)
		}
		defer file.Close()

		// Verify file permissions
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("getting file info: %v", err)
		}
		if mode := info.Mode().Perm(); mode != 0644 {
			t.Errorf("wrong file permissions: got %v, want %v", mode, 0644)
		}
	})

	t.Run("Integer Operations", func(t *testing.T) {
		path := filepath.Join(tmpDir, "integers.bin")
		file, err := common.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("creating file: %v", err)
		}
		defer file.Close()

		// Test uint32
		want32 := uint32(0xDEADBEEF)
		if err := file.WriteUint32(want32); err != nil {
			t.Errorf("writing uint32: %v", err)
		}

		// Test uint64
		want64 := uint64(0xDEADBEEFCAFEBABE)
		if err := file.WriteUint64(want64); err != nil {
			t.Errorf("writing uint64: %v", err)
		}

		// Seek back to start
		if _, err := file.Seek(0, 0); err != nil {
			t.Fatalf("seeking to start: %v", err)
		}

		// Read and verify uint32
		got32, err := file.ReadUint32()
		if err != nil {
			t.Errorf("reading uint32: %v", err)
		}
		if got32 != want32 {
			t.Errorf("uint32 mismatch: got %x, want %x", got32, want32)
		}

		// Read and verify uint64
		got64, err := file.ReadUint64()
		if err != nil {
			t.Errorf("reading uint64: %v", err)
		}
		if got64 != want64 {
			t.Errorf("uint64 mismatch: got %x, want %x", got64, want64)
		}
	})

	t.Run("Exact Read Write", func(t *testing.T) {
		path := filepath.Join(tmpDir, "exact.bin")
		file, err := common.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("creating file: %v", err)
		}
		defer file.Close()

		// Write test data
		want := []byte("test data with exact length")
		if err := file.WriteExactly(want); err != nil {
			t.Errorf("writing exact data: %v", err)
		}

		// Seek back to start
		if _, err := file.Seek(0, 0); err != nil {
			t.Fatalf("seeking to start: %v", err)
		}

		// Read exact amount
		got, err := file.ReadExactly(len(want))
		if err != nil {
			t.Errorf("reading exact data: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("data mismatch:\ngot: %q\nwant: %q", got, want)
		}
	})

	t.Run("Short Read", func(t *testing.T) {
		path := filepath.Join(tmpDir, "short.bin")
		file, err := common.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("creating file: %v", err)
		}
		defer file.Close()

		// Write small amount of data
		data := []byte("small")
		if err := file.WriteExactly(data); err != nil {
			t.Fatalf("writing data: %v", err)
		}

		// Seek back to start
		if _, err := file.Seek(0, 0); err != nil {
			t.Fatalf("seeking to start: %v", err)
		}

		// Try to read more than available
		_, err = file.ReadExactly(100)
		if err == nil {
			t.Error("expected error reading more than available")
		}
	})

	t.Run("File Sync", func(t *testing.T) {
		path := filepath.Join(tmpDir, "sync.bin")
		file, err := common.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("creating file: %v", err)
		}
		defer file.Close()

		// Write some data
		if err := file.WriteExactly([]byte("test data")); err != nil {
			t.Fatalf("writing data: %v", err)
		}

		// Test sync
		if err := file.Sync(); err != nil {
			t.Errorf("syncing file: %v", err)
		}
	})

	t.Run("Error Cases", func(t *testing.T) {
		// Try to open non-existent file read-only
		_, err := common.OpenFile(filepath.Join(tmpDir, "nonexistent"), os.O_RDONLY, 0)
		if err == nil {
			t.Error("expected error opening non-existent file")
		}

		// Try to open file in invalid directory
		_, err = common.OpenFile(filepath.Join(tmpDir, "invalid", "file"), os.O_CREATE|os.O_RDWR, 0644)
		if err == nil {
			t.Error("expected error creating file in non-existent directory")
		}

		// Create file for error tests
		path := filepath.Join(tmpDir, "errors.bin")
		file, err := common.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("creating file: %v", err)
		}

		// Close file and try operations on closed file
		if err := file.Close(); err != nil {
			t.Fatalf("closing file: %v", err)
		}

		if err := file.WriteUint32(42); err == nil {
			t.Error("expected error writing to closed file")
		}

		if _, err := file.ReadUint32(); err == nil {
			t.Error("expected error reading from closed file")
		}

		if err := file.WriteUint64(42); err == nil {
			t.Error("expected error writing to closed file")
		}

		if _, err := file.ReadUint64(); err == nil {
			t.Error("expected error reading from closed file")
		}

		if err := file.WriteExactly([]byte("test")); err == nil {
			t.Error("expected error writing to closed file")
		}

		if _, err := file.ReadExactly(4); err == nil {
			t.Error("expected error reading from closed file")
		}

		if err := file.Sync(); err == nil {
			t.Error("expected error syncing closed file")
		}
	})
}
