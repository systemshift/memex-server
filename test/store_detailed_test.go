package test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/systemshift/memex/internal/memex/storage/rabin"
	"github.com/systemshift/memex/internal/memex/storage/store"
)

func TestChunkStoreDetailed(t *testing.T) {
	t.Run("Store Creation", func(t *testing.T) {
		path := "test_store.mx"
		defer os.Remove(path)

		// Create new store
		chunker := rabin.NewChunker()
		s, err := store.NewStore(path, chunker, nil)
		if err != nil {
			t.Fatalf("creating store: %v", err)
		}
		defer s.Close()

		// Verify file exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("store file not created")
		}
	})

	t.Run("Content Storage and Retrieval", func(t *testing.T) {
		path := "test_store.mx"
		defer os.Remove(path)

		// Create store
		chunker := rabin.NewChunker()
		s, err := store.NewStore(path, chunker, nil)
		if err != nil {
			t.Fatalf("creating store: %v", err)
		}
		defer s.Close()

		// Store content
		content := []byte("test content")
		addresses, err := s.Put(content)
		if err != nil {
			t.Fatalf("storing content: %v", err)
		}

		// Retrieve content
		retrieved, err := s.Get(addresses)
		if err != nil {
			t.Fatalf("retrieving content: %v", err)
		}

		// Verify content
		if !bytes.Equal(content, retrieved) {
			t.Error("retrieved content does not match original")
		}
	})

	t.Run("Content Deduplication", func(t *testing.T) {
		path := "test_store.mx"
		defer os.Remove(path)

		// Create store
		chunker := rabin.NewChunker()
		s, err := store.NewStore(path, chunker, nil)
		if err != nil {
			t.Fatalf("creating store: %v", err)
		}
		defer s.Close()

		// Store same content twice
		content := []byte("duplicate content")
		addr1, err := s.Put(content)
		if err != nil {
			t.Fatalf("storing content first time: %v", err)
		}

		addr2, err := s.Put(content)
		if err != nil {
			t.Fatalf("storing content second time: %v", err)
		}

		// Verify addresses are identical
		if !bytes.Equal(addr1[0], addr2[0]) {
			t.Error("duplicate content stored separately")
		}
	})

	t.Run("Content Deletion", func(t *testing.T) {
		path := "test_store.mx"
		defer os.Remove(path)

		// Create store
		chunker := rabin.NewChunker()
		s, err := store.NewStore(path, chunker, nil)
		if err != nil {
			t.Fatalf("creating store: %v", err)
		}
		defer s.Close()

		// Store content
		content := []byte("content to delete")
		addresses, err := s.Put(content)
		if err != nil {
			t.Fatalf("storing content: %v", err)
		}

		// Delete content
		if err := s.Delete(addresses); err != nil {
			t.Fatalf("deleting content: %v", err)
		}

		// Try to retrieve deleted content
		_, err = s.Get(addresses)
		if err == nil {
			t.Error("deleted content still accessible")
		}
	})

	t.Run("Reference Counting", func(t *testing.T) {
		path := "test_store.mx"
		defer os.Remove(path)

		// Create store
		chunker := rabin.NewChunker()
		s, err := store.NewStore(path, chunker, nil)
		if err != nil {
			t.Fatalf("creating store: %v", err)
		}
		defer s.Close()

		// Store content
		content := []byte("shared content")
		addr1, err := s.Put(content)
		if err != nil {
			t.Fatalf("storing content first time: %v", err)
		}

		// Reference same content
		addr2, err := s.Put(content)
		if err != nil {
			t.Fatalf("storing content second time: %v", err)
		}

		// Delete first reference
		if err := s.Delete(addr1); err != nil {
			t.Fatalf("deleting first reference: %v", err)
		}

		// Content should still be accessible
		_, err = s.Get(addr2)
		if err != nil {
			t.Error("content inaccessible after deleting first reference")
		}

		// Delete second reference
		if err := s.Delete(addr2); err != nil {
			t.Fatalf("deleting second reference: %v", err)
		}

		// Content should now be inaccessible
		_, err = s.Get(addr2)
		if err == nil {
			t.Error("content still accessible after deleting all references")
		}
	})

	t.Run("Store Persistence", func(t *testing.T) {
		path := "test_store.mx"
		defer os.Remove(path)

		// Create store and add content
		chunker := rabin.NewChunker()
		s1, err := store.NewStore(path, chunker, nil)
		if err != nil {
			t.Fatalf("creating first store: %v", err)
		}

		content := []byte("persistent content")
		addresses, err := s1.Put(content)
		if err != nil {
			t.Fatalf("storing content: %v", err)
		}

		if err := s1.Close(); err != nil {
			t.Fatalf("closing first store: %v", err)
		}

		// Open new store instance
		s2, err := store.NewStore(path, chunker, nil)
		if err != nil {
			t.Fatalf("creating second store: %v", err)
		}
		defer s2.Close()

		// Try to retrieve content
		retrieved, err := s2.Get(addresses)
		if err != nil {
			t.Fatalf("retrieving content from second store: %v", err)
		}

		if !bytes.Equal(content, retrieved) {
			t.Error("retrieved content does not match original after reopening store")
		}
	})

	t.Run("Content Integrity", func(t *testing.T) {
		path := "test_store.mx"
		defer os.Remove(path)

		// Create store
		chunker := rabin.NewChunker()
		s, err := store.NewStore(path, chunker, nil)
		if err != nil {
			t.Fatalf("creating store: %v", err)
		}
		defer s.Close()

		// Store content
		content := []byte("content to verify")
		addresses, err := s.Put(content)
		if err != nil {
			t.Fatalf("storing content: %v", err)
		}

		// Calculate expected hash
		hash := sha256.Sum256(content)
		expectedAddr := hash[:]

		// Verify stored address matches content hash
		if !bytes.Equal(addresses[0], expectedAddr) {
			t.Errorf("content hash mismatch: got %x, want %x", addresses[0], expectedAddr)
		}

		// Verify content can be retrieved by hash
		retrieved, err := s.Get([][]byte{expectedAddr})
		if err != nil {
			t.Fatalf("retrieving content by hash: %v", err)
		}

		if !bytes.Equal(content, retrieved) {
			t.Error("retrieved content does not match original")
		}
	})

	t.Run("Chunk Listing", func(t *testing.T) {
		path := "test_store.mx"
		defer os.Remove(path)

		// Create store
		chunker := rabin.NewChunker()
		s, err := store.NewStore(path, chunker, nil)
		if err != nil {
			t.Fatalf("creating store: %v", err)
		}
		defer s.Close()

		// Store multiple chunks
		contents := [][]byte{
			[]byte("chunk one"),
			[]byte("chunk two"),
			[]byte("chunk three"),
		}

		var storedAddrs [][]byte
		for _, content := range contents {
			addr, err := s.Put(content)
			if err != nil {
				t.Fatalf("storing chunk: %v", err)
			}
			storedAddrs = append(storedAddrs, addr...)
		}

		// List chunks
		chunks, err := s.ListChunks()
		if err != nil {
			t.Fatalf("listing chunks: %v", err)
		}

		// Verify all stored chunks are listed
		if len(chunks) != len(storedAddrs) {
			t.Errorf("chunk count mismatch: got %d, want %d", len(chunks), len(storedAddrs))
		}

		// Convert addresses to hex strings for comparison
		storedHexes := make(map[string]bool)
		for _, addr := range storedAddrs {
			storedHexes[hex.EncodeToString(addr)] = true
		}

		for _, chunk := range chunks {
			hexAddr := hex.EncodeToString(chunk)
			if !storedHexes[hexAddr] {
				t.Errorf("unexpected chunk in listing: %s", hexAddr)
			}
		}
	})
}
