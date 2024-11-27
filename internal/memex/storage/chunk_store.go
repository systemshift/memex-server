package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ChunkStore manages content chunks with reference counting
type ChunkStore struct {
	path  string
	mutex sync.RWMutex
	refs  map[string]int
}

// NewChunkStore creates a new chunk store
func NewChunkStore(path string) *ChunkStore {
	store := &ChunkStore{
		path: path,
		refs: make(map[string]int),
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(path, 0755); err != nil {
		return store // Return store anyway, error will be caught on first write
	}

	// Load reference counts if they exist
	refsPath := filepath.Join(path, "refs.json")
	if data, err := os.ReadFile(refsPath); err == nil {
		if err := json.Unmarshal(data, &store.refs); err != nil {
			// If refs.json exists but is invalid, scan directory to rebuild refs
			store.scanAndSaveRefs()
		}
	} else if os.IsNotExist(err) {
		// If refs.json doesn't exist, scan directory to build initial refs
		store.scanAndSaveRefs()
	}

	return store
}

// scanAndSaveRefs scans the directory and saves the refs atomically
func (s *ChunkStore) scanAndSaveRefs() {
	// Scan directory without holding lock
	refs := make(map[string]int)

	filepath.Walk(s.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil // Skip directories
		}
		if info.Name() == "refs.json" {
			return nil // Skip refs.json file
		}

		// Get parent directory name (first 2 chars of hash)
		dir := filepath.Base(filepath.Dir(path))
		if len(dir) != 2 {
			return nil // Skip invalid paths
		}

		// Get file name (rest of hash)
		name := filepath.Base(path)
		if len(name) != 62 {
			return nil // Skip invalid files
		}

		// Combine to get full hash
		hash := dir + name

		// Verify it's a valid hex string
		if _, err := hex.DecodeString(hash); err != nil {
			return nil // Skip invalid hashes
		}

		// Add to reference counts
		refs[hash] = 1
		return nil
	})

	// Update refs under lock
	s.mutex.Lock()
	s.refs = refs
	s.mutex.Unlock()

	// Save to disk
	s.saveRefs()
}

// saveRefs saves reference counts to disk
func (s *ChunkStore) saveRefs() error {
	// Copy refs under read lock
	s.mutex.RLock()
	refs := make(map[string]int, len(s.refs))
	for k, v := range s.refs {
		refs[k] = v
	}
	s.mutex.RUnlock()

	// Marshal reference counts
	data, err := json.Marshal(refs)
	if err != nil {
		return fmt.Errorf("marshaling refs: %w", err)
	}

	// Write to file atomically
	refsPath := filepath.Join(s.path, "refs.json")
	tempPath := refsPath + ".tmp"

	// Write to temp file first
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("writing temp refs file: %w", err)
	}

	// Rename temp file to final location
	if err := os.Rename(tempPath, refsPath); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return fmt.Errorf("renaming refs file: %w", err)
	}

	return nil
}

// Store stores a chunk and returns its hash
func (s *ChunkStore) Store(content []byte) (string, error) {
	// Calculate hash
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])

	// Check if chunk already exists
	s.mutex.RLock()
	count := s.refs[hashStr]
	s.mutex.RUnlock()

	if count > 0 {
		// Increment reference count
		s.mutex.Lock()
		s.refs[hashStr]++
		s.mutex.Unlock()

		// Save updated reference counts
		if err := s.saveRefs(); err != nil {
			return "", fmt.Errorf("saving refs: %w", err)
		}

		return hashStr, nil
	}

	// Create directory path
	dirPath := filepath.Join(s.path, hashStr[:2])
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", fmt.Errorf("creating directory: %w", err)
	}

	// Create file path
	filePath := filepath.Join(dirPath, hashStr[2:])

	// Write chunk atomically
	tempPath := filePath + ".tmp"

	// Write to temp file first
	if err := os.WriteFile(tempPath, content, 0644); err != nil {
		return "", fmt.Errorf("writing temp chunk file: %w", err)
	}

	// Rename temp file to final location
	if err := os.Rename(tempPath, filePath); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return "", fmt.Errorf("renaming chunk file: %w", err)
	}

	// Add reference count
	s.mutex.Lock()
	s.refs[hashStr] = 1
	s.mutex.Unlock()

	// Save updated reference counts
	if err := s.saveRefs(); err != nil {
		return "", fmt.Errorf("saving refs: %w", err)
	}

	return hashStr, nil
}

// Get retrieves a chunk by hash
func (s *ChunkStore) Get(hash string) ([]byte, error) {
	// Check reference count
	s.mutex.RLock()
	count := s.refs[hash]
	s.mutex.RUnlock()

	if count == 0 {
		// If no reference count, try scanning directory
		s.scanAndSaveRefs()

		s.mutex.RLock()
		count = s.refs[hash]
		s.mutex.RUnlock()

		if count == 0 {
			return nil, fmt.Errorf("chunk not found or deleted: %s", hash)
		}
	}

	// Create file path
	filePath := filepath.Join(s.path, hash[:2], hash[2:])

	// Read chunk
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// If file doesn't exist but we have a reference count,
			// something is wrong - clear the reference count
			s.mutex.Lock()
			delete(s.refs, hash)
			s.mutex.Unlock()
			s.saveRefs()
			return nil, fmt.Errorf("chunk not found: %w", err)
		}
		return nil, fmt.Errorf("reading chunk: %w", err)
	}

	return content, nil
}

// Delete removes a chunk
func (s *ChunkStore) Delete(hash string) error {
	// Decrement reference count
	s.mutex.Lock()
	s.refs[hash]--
	count := s.refs[hash]
	if count > 0 {
		s.mutex.Unlock()
		// Save updated reference counts
		if err := s.saveRefs(); err != nil {
			return fmt.Errorf("saving refs: %w", err)
		}
		return nil
	}

	// Remove from reference count map
	delete(s.refs, hash)
	s.mutex.Unlock()

	// Save updated reference counts
	if err := s.saveRefs(); err != nil {
		return fmt.Errorf("saving refs: %w", err)
	}

	// Create file path
	filePath := filepath.Join(s.path, hash[:2], hash[2:])

	// Delete chunk
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting chunk: %w", err)
	}

	return nil
}

// ChunkContent splits content into chunks using word boundaries
func ChunkContent(content []byte) ([]Chunk, error) {
	fmt.Printf("Chunking content of size %d bytes\n", len(content))

	// For small content, use word boundaries
	if len(content) <= 1024 {
		var chunks []Chunk
		start := 0
		for i := 0; i < len(content); i++ {
			if content[i] == ' ' || content[i] == '\n' || content[i] == '.' || i == len(content)-1 {
				end := i
				if i == len(content)-1 {
					end = i + 1
				}
				chunk := Chunk{Content: content[start:end]}
				hash := sha256.Sum256(chunk.Content)
				chunk.Hash = hex.EncodeToString(hash[:])
				chunks = append(chunks, chunk)
				start = i + 1
			}
		}
		fmt.Printf("Created %d word-based chunks\n", len(chunks))
		return chunks, nil
	}

	// For large content, use fixed-size chunks
	var chunks []Chunk
	chunkSize := 512
	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}
		chunk := Chunk{Content: content[i:end]}
		hash := sha256.Sum256(chunk.Content)
		chunk.Hash = hex.EncodeToString(hash[:])
		chunks = append(chunks, chunk)
	}

	fmt.Printf("Created %d fixed-size chunks\n", len(chunks))
	return chunks, nil
}
