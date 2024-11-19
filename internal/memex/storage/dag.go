package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"memex/internal/memex/core"
)

// DAGStore implements the core.Repository interface using a DAG structure
type DAGStore struct {
	rootDir    string
	chunkStore core.ChunkStore
}

// NewDAGStore creates a new DAG-based storage
func NewDAGStore(rootDir string) (*DAGStore, error) {
	log.Printf("Creating DAG store in %s", rootDir)

	// Create required directories
	dirs := []string{
		filepath.Join(rootDir, "nodes"),  // Node metadata
		filepath.Join(rootDir, "chunks"), // Content chunks
		filepath.Join(rootDir, "root"),   // Root state
		filepath.Join(rootDir, "links"),  // Link metadata
	}

	for _, dir := range dirs {
		log.Printf("Creating directory: %s", dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// Initialize chunk store
	chunkStore := NewChunkStore(filepath.Join(rootDir, "chunks"))

	store := &DAGStore{
		rootDir:    rootDir,
		chunkStore: chunkStore,
	}

	// Initialize root if it doesn't exist
	if err := store.initRoot(); err != nil {
		return nil, fmt.Errorf("initializing root: %w", err)
	}

	return store, nil
}

// initRoot creates initial root state if it doesn't exist
func (s *DAGStore) initRoot() error {
	rootPath := filepath.Join(s.rootDir, "root", "state.json")
	log.Printf("Initializing root state at %s", rootPath)

	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		root := core.Root{
			Modified: time.Now(),
			Nodes:    []string{},
		}
		data, err := json.MarshalIndent(root, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling root: %w", err)
		}
		if err := os.WriteFile(rootPath, data, 0644); err != nil {
			return fmt.Errorf("writing root state: %w", err)
		}
		log.Printf("Created initial root state")
	} else {
		log.Printf("Root state already exists")
	}
	return nil
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
	root, err := s.GetRoot()
	if err != nil {
		return "", fmt.Errorf("getting root: %w", err)
	}
	root.Nodes = append(root.Nodes, id)
	root.Modified = time.Now()

	// Store updated root
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling root: %w", err)
	}

	rootPath := filepath.Join(s.rootDir, "root", "state.json")
	if err := os.WriteFile(rootPath, data, 0644); err != nil {
		return "", fmt.Errorf("writing root state: %w", err)
	}
	log.Printf("Updated root state with new node")

	return id, nil
}

// GetNode retrieves a node by ID
func (s *DAGStore) GetNode(id string) (core.Node, error) {
	nodePath := filepath.Join(s.rootDir, "nodes", id+".json")
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
	nodePath := filepath.Join(s.rootDir, "nodes", id+".json")
	if err := os.Remove(nodePath); err != nil {
		return fmt.Errorf("removing node file: %w", err)
	}

	// Update root
	root, err := s.GetRoot()
	if err != nil {
		return fmt.Errorf("getting root: %w", err)
	}

	// Remove node from root
	var nodes []string
	for _, n := range root.Nodes {
		if n != id {
			nodes = append(nodes, n)
		}
	}
	root.Nodes = nodes
	root.Modified = time.Now()

	// Store updated root
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling root: %w", err)
	}

	rootPath := filepath.Join(s.rootDir, "root", "state.json")
	if err := os.WriteFile(rootPath, data, 0644); err != nil {
		return fmt.Errorf("writing root state: %w", err)
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

// AddLink creates a link between nodes
func (s *DAGStore) AddLink(source, target, linkType string, meta map[string]any) error {
	// Create link
	link := core.Link{
		Source: source,
		Target: target,
		Type:   linkType,
		Meta:   meta,
	}

	// Store link
	data, err := json.MarshalIndent(link, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling link: %w", err)
	}

	// Use full IDs in link filename
	linkPath := filepath.Join(s.rootDir, "links", fmt.Sprintf("%s-%s-%s.json", source, target, linkType))
	if err := os.WriteFile(linkPath, data, 0644); err != nil {
		return fmt.Errorf("writing link file: %w", err)
	}

	log.Printf("Created link file: %s", linkPath)
	return nil
}

// GetLinks returns all links for a node
func (s *DAGStore) GetLinks(nodeID string) ([]core.Link, error) {
	var links []core.Link
	linksDir := filepath.Join(s.rootDir, "links")

	entries, err := os.ReadDir(linksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return links, nil
		}
		return nil, fmt.Errorf("reading links directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(linksDir, entry.Name()))
			if err != nil {
				continue
			}

			var link core.Link
			if err := json.Unmarshal(data, &link); err != nil {
				continue
			}

			if link.Source == nodeID || link.Target == nodeID {
				links = append(links, link)
			}
		}
	}

	return links, nil
}

// DeleteLink removes a link
func (s *DAGStore) DeleteLink(source, target string) error {
	linksDir := filepath.Join(s.rootDir, "links")
	entries, err := os.ReadDir(linksDir)
	if err != nil {
		return fmt.Errorf("reading links directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(linksDir, entry.Name()))
			if err != nil {
				continue
			}

			var link core.Link
			if err := json.Unmarshal(data, &link); err != nil {
				continue
			}

			if link.Source == source && link.Target == target {
				if err := os.Remove(filepath.Join(linksDir, entry.Name())); err != nil {
					return fmt.Errorf("removing link file: %w", err)
				}
			}
		}
	}

	return nil
}

// Search finds nodes matching criteria
func (s *DAGStore) Search(query map[string]any) ([]core.Node, error) {
	var nodes []core.Node
	root, err := s.GetRoot()
	if err != nil {
		return nodes, err
	}

	for _, id := range root.Nodes {
		node, err := s.GetNode(id)
		if err != nil {
			continue
		}

		// Check if node matches query
		matches := true
		for k, v := range query {
			if nodeVal, ok := node.Meta[k]; !ok {
				matches = false
				break
			} else if nodeVal != v {
				matches = false
				break
			}
		}

		if matches {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

// FindByType returns all nodes of a specific type
func (s *DAGStore) FindByType(nodeType string) ([]core.Node, error) {
	var nodes []core.Node
	root, err := s.GetRoot()
	if err != nil {
		return nodes, err
	}

	for _, id := range root.Nodes {
		node, err := s.GetNode(id)
		if err != nil {
			continue
		}

		if node.Type == nodeType {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

// GetRoot returns the current root state
func (s *DAGStore) GetRoot() (core.Root, error) {
	rootPath := filepath.Join(s.rootDir, "root", "state.json")
	data, err := os.ReadFile(rootPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty root if no state exists
			return core.Root{
				Modified: time.Now(),
				Nodes:    []string{},
			}, nil
		}
		return core.Root{}, fmt.Errorf("reading root state: %w", err)
	}

	var root core.Root
	if err := json.Unmarshal(data, &root); err != nil {
		return core.Root{}, fmt.Errorf("parsing root state: %w", err)
	}

	return root, nil
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
	nodePath := filepath.Join(s.rootDir, "nodes", node.ID+".json")
	if err := os.WriteFile(nodePath, data, 0644); err != nil {
		return fmt.Errorf("writing node file: %w", err)
	}

	return nil
}

func (s *DAGStore) hashContent(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
