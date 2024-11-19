package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"time"

	"memex/internal/memex/core"
)

// Repository implements the core.Repository interface
type Repository struct {
	objects  *BinaryStore
	versions *BinaryVersionStore
	links    *BinaryLinkStore
	rootDir  string
}

// generateHash creates a hash from content
func generateHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// NewRepository creates a new repository instance
func NewRepository(path string) (*Repository, error) {
	// Initialize storage components
	objects, err := NewBinaryStore(path)
	if err != nil {
		return nil, fmt.Errorf("initializing object store: %w", err)
	}

	versions, err := NewVersionStore(path)
	if err != nil {
		return nil, fmt.Errorf("initializing version store: %w", err)
	}

	links, err := NewLinkStore(path)
	if err != nil {
		return nil, fmt.Errorf("initializing link store: %w", err)
	}

	return &Repository{
		objects:  objects,
		versions: versions,
		links:    links,
		rootDir:  path,
	}, nil
}

// AddNode creates a new node with content
func (r *Repository) AddNode(content []byte, nodeType string, meta map[string]any) (string, error) {
	// Split content into chunks
	chunks, err := ChunkContent(content)
	if err != nil {
		return "", fmt.Errorf("chunking content: %w", err)
	}

	// Store each chunk
	var chunkHashes []string
	for _, chunk := range chunks {
		if err := r.objects.StoreChunk(chunk.Hash, chunk.Content); err != nil {
			return "", fmt.Errorf("storing chunk: %w", err)
		}
		chunkHashes = append(chunkHashes, chunk.Hash)
	}

	// Create initial version
	version := core.Version{
		Hash:      generateHash(content),
		Chunks:    chunkHashes,
		Created:   time.Now(),
		Available: true,
		Meta:      make(map[string]any),
	}

	// Create node
	node := core.Node{
		Type:     nodeType,
		Created:  time.Now(),
		Modified: time.Now(),
		Meta:     meta,
		Versions: []core.Version{version},
		Current:  version.Hash,
	}

	// Store node
	id, err := r.objects.Store(node)
	if err != nil {
		return "", fmt.Errorf("storing node: %w", err)
	}

	node.ID = id

	return id, nil
}

// GetNode retrieves a node by ID
func (r *Repository) GetNode(id string) (core.Node, error) {
	return r.objects.Load(id)
}

// UpdateNode updates a node's content
func (r *Repository) UpdateNode(id string, content []byte, meta map[string]any) error {
	// Get current node
	node, err := r.GetNode(id)
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	// Split new content into chunks
	chunks, err := ChunkContent(content)
	if err != nil {
		return fmt.Errorf("chunking content: %w", err)
	}

	// Store each chunk
	var chunkHashes []string
	for _, chunk := range chunks {
		if err := r.objects.StoreChunk(chunk.Hash, chunk.Content); err != nil {
			return fmt.Errorf("storing chunk: %w", err)
		}
		chunkHashes = append(chunkHashes, chunk.Hash)
	}

	// Create new version
	version := core.Version{
		Hash:      generateHash(content),
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
	if _, err = r.objects.Store(node); err != nil {
		return fmt.Errorf("storing updated node: %w", err)
	}

	return nil
}

// DeleteNode removes a node
func (r *Repository) DeleteNode(id string) error {
	// Get node first to get its chunks
	node, err := r.GetNode(id)
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	// Delete all chunks
	for _, version := range node.Versions {
		for _, hash := range version.Chunks {
			if err := r.objects.DeleteChunk(hash); err != nil {
				return fmt.Errorf("deleting chunk %s: %w", hash, err)
			}
		}
	}

	// Delete all versions
	if err := r.versions.Delete(id); err != nil {
		return fmt.Errorf("deleting versions: %w", err)
	}

	// Delete all links
	if err := r.links.Delete(id, ""); err != nil {
		return fmt.Errorf("deleting links: %w", err)
	}

	// Delete the node itself
	if err := r.objects.Delete(id); err != nil {
		return fmt.Errorf("deleting node: %w", err)
	}

	return nil
}

// GetVersion retrieves a specific version
func (r *Repository) GetVersion(nodeID string, hash string) (core.Version, error) {
	node, err := r.GetNode(nodeID)
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
func (r *Repository) PruneVersion(nodeID string, hash string) error {
	node, err := r.GetNode(nodeID)
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
	if _, err = r.objects.Store(node); err != nil {
		return fmt.Errorf("storing node: %w", err)
	}

	return nil
}

// RestoreVersion marks a version's content as available
func (r *Repository) RestoreVersion(nodeID string, hash string) error {
	node, err := r.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	// Find version
	found := false
	for i := range node.Versions {
		if node.Versions[i].Hash == hash {
			node.Versions[i].Available = true
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("version not found")
	}

	// Store updated node
	if _, err = r.objects.Store(node); err != nil {
		return fmt.Errorf("storing node: %w", err)
	}

	return nil
}

// AddLink creates a link between nodes
func (r *Repository) AddLink(source, target, linkType string, meta map[string]any) error {
	link := core.Link{
		Source: source,
		Target: target,
		Type:   linkType,
		Meta:   meta,
	}
	return r.links.Store(link)
}

// DeleteLink removes a link
func (r *Repository) DeleteLink(source, target string) error {
	return r.links.Delete(source, target)
}

// GetLinks returns all links for a node
func (r *Repository) GetLinks(id string) ([]core.Link, error) {
	return r.links.GetBySource(id), nil
}

// GetRoot returns the current root state
func (r *Repository) GetRoot() (core.Root, error) {
	nodes := r.List()
	root := core.Root{
		Modified: time.Now(),
		Nodes:    nodes,
	}

	// Calculate root hash
	hasher := sha256.New()
	for _, id := range nodes {
		node, err := r.GetNode(id)
		if err != nil {
			continue
		}
		hasher.Write([]byte(node.Current))
	}
	root.Hash = hex.EncodeToString(hasher.Sum(nil))

	return root, nil
}

// UpdateRoot recalculates root hash
func (r *Repository) UpdateRoot() error {
	_, err := r.GetRoot() // Just recalculate
	return err
}

// Search finds nodes matching criteria
func (r *Repository) Search(query map[string]any) ([]core.Node, error) {
	var nodes []core.Node
	for _, id := range r.List() {
		node, err := r.GetNode(id)
		if err != nil {
			continue
		}

		// Check if node matches query
		matches := true
		for k, v := range query {
			if nodeVal, ok := node.Meta[k]; !ok {
				matches = false
				break
			} else if !reflect.DeepEqual(nodeVal, v) {
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
func (r *Repository) FindByType(nodeType string) ([]core.Node, error) {
	var nodes []core.Node
	for _, id := range r.List() {
		node, err := r.GetNode(id)
		if err != nil {
			continue
		}

		if node.Type == nodeType {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

// GetChunk retrieves a chunk by hash
func (r *Repository) GetChunk(hash string) ([]byte, error) {
	return r.objects.LoadChunk(hash)
}

// HasChunk checks if a chunk exists
func (r *Repository) HasChunk(hash string) bool {
	_, err := r.objects.LoadChunk(hash)
	return err == nil
}

// List returns all node IDs
func (r *Repository) List() []string {
	return r.objects.List()
}
