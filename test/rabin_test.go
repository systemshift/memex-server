package test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/systemshift/memex/internal/memex/storage/rabin"
)

func TestRabinChunker(t *testing.T) {
	t.Run("Empty Content", func(t *testing.T) {
		chunker := rabin.NewChunker()
		chunks := chunker.Split([]byte{})
		if chunks != nil {
			t.Error("expected nil chunks for empty content")
		}
	})

	t.Run("Small Content", func(t *testing.T) {
		chunker := rabin.NewChunker()
		content := []byte("small")
		chunks := chunker.Split(content)
		if len(chunks) != 1 {
			t.Errorf("expected 1 chunk, got %d", len(chunks))
		}
		if !bytes.Equal(chunks[0], content) {
			t.Error("chunk content mismatch")
		}
	})

	t.Run("JSON Content", func(t *testing.T) {
		chunker := rabin.NewChunker()
		content := []byte(`{"key": "value", "array": [1,2,3], "nested": {"a": "b"}}`)
		chunks := chunker.Split(content)
		if len(chunks) != 1 {
			t.Errorf("expected 1 chunk for JSON, got %d", len(chunks))
		}
		if !bytes.Equal(chunks[0], content) {
			t.Error("JSON content was modified")
		}
	})

	t.Run("Repeated Content", func(t *testing.T) {
		chunker := rabin.NewChunker()
		// Create content larger than MinSize with repeated pattern
		pattern := "This is a repeated pattern. "
		content := []byte(strings.Repeat(pattern, 20)) // Ensure it's large enough
		chunks := chunker.Split(content)

		// Should create multiple chunks
		if len(chunks) < 2 {
			t.Errorf("expected multiple chunks for repeated content, got %d", len(chunks))
		}

		// Verify chunk sizes
		for i, chunk := range chunks[:len(chunks)-1] { // Skip last chunk
			if len(chunk) < rabin.MinSize {
				t.Errorf("chunk %d size %d is below MinSize %d", i, len(chunk), rabin.MinSize)
			}
			if len(chunk) > rabin.MaxSize {
				t.Errorf("chunk %d size %d exceeds MaxSize %d", i, len(chunk), rabin.MaxSize)
			}
		}
	})

	t.Run("Sentence Boundaries", func(t *testing.T) {
		chunker := rabin.NewChunker()
		// Create content larger than MinSize
		sentences := []string{
			"This is the first sentence. ",
			"Here comes the second sentence! ",
			"What about a third sentence? ",
			"And finally the fourth sentence. ",
		}
		content := []byte(strings.Repeat(strings.Join(sentences, ""), 5))
		chunks := chunker.Split(content)

		// Should create multiple chunks
		if len(chunks) < 2 {
			t.Errorf("expected multiple chunks, got %d", len(chunks))
		}

		// Verify chunk sizes
		for i, chunk := range chunks[:len(chunks)-1] { // Skip last chunk
			if len(chunk) < rabin.MinSize {
				t.Errorf("chunk %d size %d is below MinSize %d", i, len(chunk), rabin.MinSize)
			}
			if len(chunk) > rabin.MaxSize {
				t.Errorf("chunk %d size %d exceeds MaxSize %d", i, len(chunk), rabin.MaxSize)
			}
		}
	})

	t.Run("Phrase Boundaries", func(t *testing.T) {
		chunker := rabin.NewChunker()
		// Create content larger than MinSize
		phrases := []string{
			"this is the first phrase, ",
			"followed by the second phrase; ",
			"then comes the third phrase: ",
			"and finally the fourth phrase",
		}
		content := []byte(strings.Repeat(strings.Join(phrases, ""), 5))
		chunks := chunker.Split(content)

		// Should create multiple chunks
		if len(chunks) < 2 {
			t.Errorf("expected multiple chunks, got %d", len(chunks))
		}

		// Verify chunk sizes
		for i, chunk := range chunks[:len(chunks)-1] { // Skip last chunk
			if len(chunk) < rabin.MinSize {
				t.Errorf("chunk %d size %d is below MinSize %d", i, len(chunk), rabin.MinSize)
			}
			if len(chunk) > rabin.MaxSize {
				t.Errorf("chunk %d size %d exceeds MaxSize %d", i, len(chunk), rabin.MaxSize)
			}
		}
	})

	t.Run("Large Content", func(t *testing.T) {
		chunker := rabin.NewChunker()
		// Create content larger than MaxSize
		content := make([]byte, rabin.MaxSize*2)
		for i := range content {
			content[i] = byte(i % 256)
		}
		chunks := chunker.Split(content)

		// Should create multiple chunks
		if len(chunks) < 2 {
			t.Errorf("expected multiple chunks, got %d", len(chunks))
		}

		// Verify chunk sizes
		for i, chunk := range chunks[:len(chunks)-1] { // Skip last chunk
			if len(chunk) < rabin.MinSize {
				t.Errorf("chunk %d size %d is below MinSize %d", i, len(chunk), rabin.MinSize)
			}
			if len(chunk) > rabin.MaxSize {
				t.Errorf("chunk %d size %d exceeds MaxSize %d", i, len(chunk), rabin.MaxSize)
			}
		}
	})

	t.Run("Content Reconstruction", func(t *testing.T) {
		chunker := rabin.NewChunker()
		// Create content larger than MinSize
		original := []byte(strings.Repeat("This is a test of content reconstruction to ensure no data is lost during chunking. ", 10))
		chunks := chunker.Split(original)

		// Reconstruct content from chunks
		var reconstructed []byte
		for _, chunk := range chunks {
			reconstructed = append(reconstructed, chunk...)
		}

		if !bytes.Equal(original, reconstructed) {
			t.Error("reconstructed content does not match original")
		}
	})

	t.Run("Pattern Detection", func(t *testing.T) {
		chunker := rabin.NewChunker()
		// Create content with repeated patterns
		pattern := strings.Repeat("ABCDEFGHIJKLMNOP", 4) // 64 bytes
		content := []byte(strings.Repeat(pattern, 10))   // 640 bytes total
		chunks := chunker.Split(content)

		// Should create multiple chunks
		if len(chunks) < 2 {
			t.Errorf("expected multiple chunks, got %d", len(chunks))
		}

		// Verify chunk sizes
		for i, chunk := range chunks[:len(chunks)-1] { // Skip last chunk
			if len(chunk) < rabin.MinSize {
				t.Errorf("chunk %d size %d is below MinSize %d", i, len(chunk), rabin.MinSize)
			}
			if len(chunk) > rabin.MaxSize {
				t.Errorf("chunk %d size %d exceeds MaxSize %d", i, len(chunk), rabin.MaxSize)
			}
		}

		// Verify some chunks are similar in size (pattern detection)
		sizes := make(map[int]int)
		for _, chunk := range chunks {
			sizes[len(chunk)]++
		}

		// Should have some chunks of the same size
		foundDuplicate := false
		for _, count := range sizes {
			if count > 1 {
				foundDuplicate = true
				break
			}
		}
		if !foundDuplicate {
			t.Error("no chunks of similar size found (pattern detection may not be working)")
		}
	})
}
