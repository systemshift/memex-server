package rabin

import (
	"math/bits"
)

const (
	// Window size for rolling hash
	WindowSize = 64

	// Minimum and maximum chunk sizes
	MinSize = 4 * 1024    // 4KB
	MaxSize = 1024 * 1024 // 1MB

	// Average chunk size (target)
	TargetSize = 64 * 1024 // 64KB

	// Polynomial for rolling hash
	// Using a random irreducible polynomial
	Polynomial = 0x3DA3358B4DC173
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
}

// NewChunker creates a new Rabin chunker
func NewChunker() *RabinChunker {
	c := &RabinChunker{
		window: make([]byte, WindowSize),
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

	var chunks [][]byte
	start := 0
	length := 0
	c.reset()

	// Process content
	for i := 0; i < len(content); i++ {
		c.slide(content[i])
		length++

		// Check for chunk boundary
		if length >= MinSize {
			// Use lowest 13 bits for boundary detection
			// This gives average chunk size of ~64KB
			if c.hash&0x1FFF == 0 || length >= MaxSize {
				chunks = append(chunks, content[start:i+1])
				start = i + 1
				length = 0
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
	for i := range c.window {
		c.window[i] = 0
	}
}

func (c *RabinChunker) slide(b byte) {
	// Remove oldest byte
	if c.pos >= WindowSize {
		out := c.window[c.pos%WindowSize]
		c.hash = (c.hash - c.skips[out]) * Polynomial
	}

	// Add new byte
	c.window[c.pos%WindowSize] = b
	c.hash = c.hash + c.appends[b]
	c.pos++
}

func (c *RabinChunker) buildTables() {
	// Build lookup tables for fast polynomial operations
	for i := 0; i < 256; i++ {
		hash := uint64(i)
		c.appends[i] = hash
		for j := 0; j < WindowSize-1; j++ {
			hash = hash * Polynomial
		}
		c.skips[i] = hash
	}
}

// Helper function to count trailing zeros
func trailingZeros(x uint64) int {
	return bits.TrailingZeros64(x)
}
