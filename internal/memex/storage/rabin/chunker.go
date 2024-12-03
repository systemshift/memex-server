package rabin

import (
	"bytes"
	"math/bits"
)

const (
	// Window size for rolling hash
	WindowSize = 32

	// Minimum and maximum chunk sizes
	MinSize = 32      // 32 bytes minimum
	MaxSize = 1 << 16 // 64KB maximum

	// Average chunk size (target)
	TargetSize = 256 // 256 bytes target

	// Polynomial for rolling hash
	// Using a random irreducible polynomial
	Polynomial = 0x3DA3358B4DC173

	// Pattern size for repetition detection
	PatternSize = 16
)

// RabinChunker implements content-defined chunking using Rabin fingerprinting
type RabinChunker struct {
	// Current rolling hash value
	hash uint64

	// Sliding window buffer
	window []byte

	// Current position in window
	pos int

	// Lookup tables for polynomial operations
	appends [256]uint64
	skips   [256]uint64

	// Current chunk size
	size int

	// Pattern detection
	lastPattern []byte
	patternPos  int
}

// NewChunker creates a new Rabin chunker
func NewChunker() *RabinChunker {
	c := &RabinChunker{
		window:      make([]byte, WindowSize),
		lastPattern: make([]byte, PatternSize),
	}
	c.buildTables()
	return c
}

// Split divides content into chunks using Rabin fingerprinting
func (c *RabinChunker) Split(content []byte) [][]byte {
	if len(content) == 0 {
		return nil
	}

	// Handle small content
	if len(content) <= MinSize {
		return [][]byte{content}
	}

	// Check if this is JSON content
	isJSON := bytes.HasPrefix(bytes.TrimSpace(content), []byte("{"))
	if isJSON {
		return [][]byte{content} // Don't split JSON
	}

	var chunks [][]byte
	start := 0
	c.reset()

	// Process content
	for i := 0; i < len(content); i++ {
		c.slide(content[i])
		c.size++

		// Update pattern detection
		if c.size >= PatternSize {
			pattern := content[max(0, i-PatternSize+1) : i+1]
			if bytes.Equal(pattern, c.lastPattern) && c.size >= MinSize {
				// Found repeated pattern, create chunk
				chunks = append(chunks, content[start:i+1])
				start = i + 1
				c.reset()
				continue
			}
			copy(c.lastPattern, pattern)
		}

		// Check for chunk boundary
		if c.size >= MinSize {
			// Use lowest 8 bits for boundary detection
			// This gives average chunk size of ~256 bytes (2^8)
			if (c.hash&0xFF) == 0 || c.size >= MaxSize {
				// Look for sentence or phrase boundaries
				boundary := i + 1
				if boundary-start >= MinSize {
					// Look back for sentence boundary
					for j := 0; j < 16 && i-j >= start; j++ {
						if isSentenceBoundary(content, i-j) {
							boundary = i - j + 1
							break
						}
					}
					// If no sentence boundary found, look forward
					if boundary == i+1 {
						for j := 0; j < 16 && i+j < len(content); j++ {
							if isSentenceBoundary(content, i+j) {
								boundary = i + j + 1
								break
							}
						}
					}
					// If still no boundary found, use phrase boundary
					if boundary == i+1 {
						for j := 0; j < 8 && i-j >= start; j++ {
							if isPhraseBoundary(content[i-j]) {
								boundary = i - j + 1
								break
							}
						}
					}
				}
				chunks = append(chunks, content[start:boundary])
				start = boundary
				i = boundary - 1 // -1 because loop will increment
				c.reset()
			}
		}
	}

	// Add remaining content as final chunk
	if start < len(content) {
		chunks = append(chunks, content[start:])
	}

	return chunks
}

// Internal methods

func (c *RabinChunker) reset() {
	c.hash = 0
	c.pos = 0
	c.size = 0
	c.patternPos = 0
	for i := range c.window {
		c.window[i] = 0
	}
	for i := range c.lastPattern {
		c.lastPattern[i] = 0
	}
}

func (c *RabinChunker) slide(b byte) {
	// Remove oldest byte
	if c.pos >= WindowSize {
		out := c.window[c.pos%WindowSize]
		c.hash = ((c.hash - c.skips[out]) * Polynomial) & 0xFFFFFFFFFFFFFFFF
	}

	// Add new byte
	c.window[c.pos%WindowSize] = b
	c.hash = (c.hash + c.appends[b]) & 0xFFFFFFFFFFFFFFFF
	c.pos++
}

func (c *RabinChunker) buildTables() {
	// Build lookup tables for fast polynomial operations
	for i := 0; i < 256; i++ {
		hash := uint64(i)
		c.appends[i] = hash
		for j := 0; j < WindowSize-1; j++ {
			hash = (hash * Polynomial) & 0xFFFFFFFFFFFFFFFF
		}
		c.skips[i] = hash
	}
}

// Helper function to count trailing zeros
func trailingZeros(x uint64) int {
	return bits.TrailingZeros64(x)
}

// Helper function to check if a position is a sentence boundary
func isSentenceBoundary(content []byte, pos int) bool {
	if pos < 0 || pos >= len(content) {
		return false
	}
	// Check for period, exclamation mark, or question mark
	if content[pos] == '.' || content[pos] == '!' || content[pos] == '?' {
		// Make sure it's followed by whitespace or end of content
		if pos+1 >= len(content) || content[pos+1] == ' ' || content[pos+1] == '\n' || content[pos+1] == '\r' || content[pos+1] == '\t' {
			return true
		}
	}
	return false
}

// Helper function to check if a byte is a phrase boundary
func isPhraseBoundary(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == ',' || b == ';' || b == ':' || b == '-'
}

// Helper function for Go < 1.21
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
