package test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/systemshift/memex/internal/memex/storage/rabin"
	"github.com/systemshift/memex/internal/memex/storage/store"
	"github.com/systemshift/memex/internal/memex/transaction"
)

func TestChunkStore(t *testing.T) {
	// Create test directory
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "test.db")
	txPath := filepath.Join(tmpDir, "tx.db")

	// Create mock storage for transactions
	mockStorage, err := NewMockStorage(txPath)
	if err != nil {
		t.Fatalf("creating mock storage: %v", err)
	}
	defer mockStorage.Close()

	// Create transaction store
	txStore, err := transaction.NewActionStore(mockStorage)
	if err != nil {
		t.Fatalf("creating transaction store: %v", err)
	}

	// Create store
	chunker := rabin.NewChunker()
	store, err := store.NewStore(storePath, chunker, txStore)
	if err != nil {
		t.Fatalf("creating store: %v", err)
	}
	defer store.Close()

	t.Run("Basic Storage", func(t *testing.T) {
		content := []byte("Hello, World!")

		// Store content
		addresses, err := store.Put(content)
		if err != nil {
			t.Fatalf("storing content: %v", err)
		}

		// Verify transaction was recorded
		history, err := txStore.GetHistory()
		if err != nil {
			t.Fatalf("getting history: %v", err)
		}
		if len(history) == 0 {
			t.Error("no transaction recorded")
		}
		lastAction := history[len(history)-1]
		if lastAction.Type != transaction.ActionPutContent {
			t.Errorf("wrong action type: got %v, want %v", lastAction.Type, transaction.ActionPutContent)
		}

		// Retrieve content
		retrieved, err := store.Get(addresses)
		if err != nil {
			t.Fatalf("retrieving content: %v", err)
		}

		if !bytes.Equal(content, retrieved) {
			t.Errorf("content mismatch: got %q, want %q", retrieved, content)
		}
	})

	t.Run("Content Deduplication", func(t *testing.T) {
		// Create two similar documents
		doc1 := []byte("The quick brown fox jumps over the lazy dog")
		doc2 := []byte("The quick brown fox runs past the sleeping cat")

		// Store both documents
		addr1, err := store.Put(doc1)
		if err != nil {
			t.Fatalf("storing doc1: %v", err)
		}

		addr2, err := store.Put(doc2)
		if err != nil {
			t.Fatalf("storing doc2: %v", err)
		}

		// Verify common prefix is deduplicated
		commonPrefix := "The quick brown fox"
		if len(addr1) == 0 || len(addr2) == 0 {
			t.Fatal("no addresses returned")
		}

		// Get first chunk of each document
		chunk1, err := store.Get(addr1[:1])
		if err != nil {
			t.Fatalf("getting chunk1: %v", err)
		}

		chunk2, err := store.Get(addr2[:1])
		if err != nil {
			t.Fatalf("getting chunk2: %v", err)
		}

		// Both chunks should contain the common prefix
		if !bytes.Contains(chunk1, []byte(commonPrefix)) {
			t.Error("chunk1 missing common prefix")
		}
		if !bytes.Contains(chunk2, []byte(commonPrefix)) {
			t.Error("chunk2 missing common prefix")
		}

		// Verify transactions were recorded
		history, err := txStore.GetHistory()
		if err != nil {
			t.Fatalf("getting history: %v", err)
		}
		putCount := 0
		for _, action := range history {
			if action.Type == transaction.ActionPutContent {
				putCount++
			}
		}
		if putCount != 3 { // Including the basic storage test
			t.Errorf("wrong number of put actions: got %d, want 3", putCount)
		}
	})

	t.Run("Large Content", func(t *testing.T) {
		// Create content larger than max chunk size
		size := 2 * 1024 * 1024 // 2MB
		content := make([]byte, size)
		for i := range content {
			content[i] = byte(i % 256)
		}

		// Store content
		addresses, err := store.Put(content)
		if err != nil {
			t.Fatalf("storing large content: %v", err)
		}

		// Content should be split into multiple chunks
		if len(addresses) < 2 {
			t.Errorf("large content not split: got %d chunks, want >1", len(addresses))
		}

		// Retrieve content
		retrieved, err := store.Get(addresses)
		if err != nil {
			t.Fatalf("retrieving large content: %v", err)
		}

		if !bytes.Equal(content, retrieved) {
			t.Error("large content mismatch")
		}
	})

	t.Run("Delete Content", func(t *testing.T) {
		content := []byte("Temporary content")

		// Store content
		addresses, err := store.Put(content)
		if err != nil {
			t.Fatalf("storing content: %v", err)
		}

		// Delete content
		if err := store.Delete(addresses); err != nil {
			t.Fatalf("deleting content: %v", err)
		}

		// Verify delete transaction was recorded
		history, err := txStore.GetHistory()
		if err != nil {
			t.Fatalf("getting history: %v", err)
		}
		lastAction := history[len(history)-1]
		if lastAction.Type != transaction.ActionDeleteContent {
			t.Errorf("wrong action type: got %v, want %v", lastAction.Type, transaction.ActionDeleteContent)
		}

		// Try to retrieve deleted content
		_, err = store.Get(addresses)
		if err == nil {
			t.Error("expected error retrieving deleted content")
		}
	})
}
