package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memex/internal/memex/core"
)

// Metadata contains repository information
type Metadata struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Created     time.Time      `json:"created"`
	Modified    time.Time      `json:"modified"`
	Version     string         `json:"version"`
	Settings    map[string]any `json:"settings"`
}

// DAGStore implements the core.Repository interface using a DAG structure
type DAGStore struct {
	path       string
	metadata   Metadata
	chunkStore core.ChunkStore
}

// GetChunkStore returns the underlying chunk store
func (s *DAGStore) GetChunkStore() core.ChunkStore {
	return s.chunkStore
}

// CreateRepository creates a new repository at the given path
func CreateRepository(path string, name string) (*DAGStore, error) {
	// Ensure path ends with .mx
	if !strings.HasSuffix(path, ".mx") {
		path += ".mx"
	}

	// Create metadata
	metadata := Metadata{
		Name:     name,
		Created:  time.Now(),
		Modified: time.Now(),
		Version:  "1.0",
		Settings: make(map[string]any),
	}

	// Create repository
	store := &DAGStore{
		path:     path,
		metadata: metadata,
	}

	// Create required directories
	dirs := []string{
		filepath.Join(path, "nodes"),  // Node metadata
		filepath.Join(path, "chunks"), // Content chunks
		filepath.Join(path, "links"),  // Link metadata
	}

	for _, dir := range dirs {
		log.Printf("Creating directory: %s", dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// Initialize chunk store
	store.chunkStore = NewChunkStore(filepath.Join(path, "chunks"))

	// Save metadata
	if err := store.saveMetadata(); err != nil {
		return nil, fmt.Errorf("saving metadata: %w", err)
	}

	// Initialize root state
	if err := store.initRoot(); err != nil {
		return nil, fmt.Errorf("initializing root: %w", err)
	}

	return store, nil
}

// OpenRepository opens an existing repository
func OpenRepository(path string) (*DAGStore, error) {
	// Ensure path ends with .mx
	if !strings.HasSuffix(path, ".mx") {
		path += ".mx"
	}

	// Check if repository exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository not found: %s", path)
	}

	// Read metadata
	data, err := os.ReadFile(filepath.Join(path, "metadata.json"))
	if err != nil {
		return nil, fmt.Errorf("reading metadata: %w", err)
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("parsing metadata: %w", err)
	}

	// Create store
	store := &DAGStore{
		path:     path,
		metadata: metadata,
	}

	// Initialize chunk store
	store.chunkStore = NewChunkStore(filepath.Join(path, "chunks"))

	return store, nil
}

// ListRepositories returns a list of repositories in the given directory
func ListRepositories(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	var repos []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".mx") {
			repos = append(repos, entry.Name())
		}
	}

	return repos, nil
}

// saveMetadata saves the repository metadata
func (s *DAGStore) saveMetadata() error {
	data, err := json.MarshalIndent(s.metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	if err := os.WriteFile(filepath.Join(s.path, "metadata.json"), data, 0644); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	return nil
}

// updateModified updates the repository's modified time
func (s *DAGStore) updateModified() error {
	s.metadata.Modified = time.Now()
	return s.saveMetadata()
}

// AddNode creates a new node with content
func (s *DAGStore) AddNode(content []byte, nodeType string, meta map[string]any) (string, error) {
	log.Printf("Adding node of type %s", nodeType)

	// Split content into chunks
	chunks, err := ChunkContent(content)
	if err != nil {
		return "", fmt.Errorf("chunking content: %w", err)
	}

	// Store chunks and collect hashes
	var chunkHashes []string
	for _, chunk := range chunks {
		hash, err := s.chunkStore.Store(chunk.Content)
		if err != nil {
			return "", fmt.Errorf("storing chunk: %w", err)
		}
		chunkHashes = append(chunkHashes, hash)
		log.Printf("Stored chunk: %s", hash)
	}

	// Create initial version
	version := core.Version{
		Hash:      s.hashContent(content),
		Chunks:    chunkHashes,
		Created:   time.Now(),
		Available: true,
		Meta:      make(map[string]any),
	}

	// Generate stable ID for node
	id := s.generateID(content, nodeType, meta)
	log.Printf("Generated node ID: %s", id)

	// Create node
	node := core.Node{
		ID:       id,
		Type:     nodeType,
		Meta:     meta,
		Created:  time.Now(),
		Modified: time.Now(),
		Versions: []core.Version{version},
		Current:  version.Hash,
	}

	// Store node
	if err := s.storeNode(node); err != nil {
		return "", fmt.Errorf("storing node: %w", err)
	}
	log.Printf("Stored node metadata")

	// Update root
	if err := s.UpdateRoot(); err != nil {
		return "", fmt.Errorf("updating root: %w", err)
	}

	// Update repository modified time
	if err := s.updateModified(); err != nil {
		return "", fmt.Errorf("updating modified time: %w", err)
	}

	return id, nil
}

// GetNode retrieves a node by ID
func (s *DAGStore) GetNode(id string) (core.Node, error) {
	nodePath := filepath.Join(s.path, "nodes", id+".json")
	data, err := os.ReadFile(nodePath)
	if err != nil {
		return core.Node{}, fmt.Errorf("reading node file: %w", err)
	}

	var node core.Node
	if err := json.Unmarshal(data, &node); err != nil {
		return core.Node{}, fmt.Errorf("parsing node data: %w", err)
	}

	return node, nil
}

// UpdateNode updates a node's content and metadata
func (s *DAGStore) UpdateNode(id string, content []byte, meta map[string]any) error {
	// Get existing node
	node, err := s.GetNode(id)
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	// Split content into chunks
	chunks, err := ChunkContent(content)
	if err != nil {
		return fmt.Errorf("chunking content: %w", err)
	}

	// Store chunks and collect hashes
	var chunkHashes []string
	for _, chunk := range chunks {
		hash, err := s.chunkStore.Store(chunk.Content)
		if err != nil {
			return fmt.Errorf("storing chunk: %w", err)
		}
		chunkHashes = append(chunkHashes, hash)
	}

	// Create new version
	version := core.Version{
		Hash:      s.hashContent(content),
		Chunks:    chunkHashes,
		Created:   time.Now(),
		Available: true,
		Meta:      make(map[string]any),
	}

	// Update node
	node.Versions = append(node.Versions, version)
	node.Current = version.Hash
	node.Modified = time.Now()
	if meta != nil {
		node.Meta = meta
	}

	// Store updated node
	if err := s.storeNode(node); err != nil {
		return fmt.Errorf("storing node: %w", err)
	}

	// Update root hash
	if err := s.UpdateRoot(); err != nil {
		return fmt.Errorf("updating root: %w", err)
	}

	// Update repository modified time
	if err := s.updateModified(); err != nil {
		return fmt.Errorf("updating modified time: %w", err)
	}

	return nil
}

// DeleteNode removes a node
func (s *DAGStore) DeleteNode(id string) error {
	// Get node first
	node, err := s.GetNode(id)
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	// Remove all versions
	for _, version := range node.Versions {
		// Only try to remove chunks if they're still available
		if version.Available {
			for _, chunk := range version.Chunks {
				// Ignore errors as chunks might be shared
				s.chunkStore.Delete(chunk)
			}
		}
	}

	// Remove node file
	nodePath := filepath.Join(s.path, "nodes", id+".json")
	if err := os.Remove(nodePath); err != nil {
		return fmt.Errorf("removing node file: %w", err)
	}

	// Update root
	if err := s.UpdateRoot(); err != nil {
		return fmt.Errorf("updating root: %w", err)
	}

	// Update repository modified time
	if err := s.updateModified(); err != nil {
		return fmt.Errorf("updating modified time: %w", err)
	}

	return nil
}

// GetVersion retrieves a specific version
func (s *DAGStore) GetVersion(nodeID string, hash string) (core.Version, error) {
	node, err := s.GetNode(nodeID)
	if err != nil {
		return core.Version{}, fmt.Errorf("getting node: %w", err)
	}

	for _, version := range node.Versions {
		if version.Hash == hash {
			return version, nil
		}
	}

	return core.Version{}, fmt.Errorf("version not found")
}

// PruneVersion marks a version's content as unavailable
func (s *DAGStore) PruneVersion(nodeID string, hash string) error {
	node, err := s.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	// Can't prune current version
	if node.Current == hash {
		return fmt.Errorf("cannot prune current version")
	}

	// Find and mark version as unavailable
	found := false
	for i := range node.Versions {
		if node.Versions[i].Hash == hash {
			node.Versions[i].Available = false
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("version not found")
	}

	// Store updated node
	if err := s.storeNode(node); err != nil {
		return fmt.Errorf("storing node: %w", err)
	}

	// Update repository modified time
	if err := s.updateModified(); err != nil {
		return fmt.Errorf("updating modified time: %w", err)
	}

	return nil
}

// GetChunk retrieves a chunk by hash
func (s *DAGStore) GetChunk(hash string) ([]byte, error) {
	return s.chunkStore.Load(hash)
}

// HasChunk checks if a chunk exists
func (s *DAGStore) HasChunk(hash string) bool {
	return s.chunkStore.Has(hash)
}

// Internal helper functions

func (s *DAGStore) generateID(content []byte, nodeType string, meta map[string]any) string {
	// Create unique ID based on initial content and metadata
	hasher := sha256.New()
	hasher.Write(content)
	hasher.Write([]byte(nodeType))
	metaJSON, _ := json.Marshal(meta)
	hasher.Write(metaJSON)
	hash := hasher.Sum(nil)
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes for shorter IDs
}

func (s *DAGStore) storeNode(node core.Node) error {
	// Marshal node data
	data, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling node: %w", err)
	}

	// Write node file
	nodePath := filepath.Join(s.path, "nodes", node.ID+".json")
	if err := os.WriteFile(nodePath, data, 0644); err != nil {
		return fmt.Errorf("writing node file: %w", err)
	}

	return nil
}

func (s *DAGStore) hashContent(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
