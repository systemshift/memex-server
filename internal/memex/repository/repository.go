package repository

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"memex/internal/memex/core"
	"memex/internal/memex/storage/rabin"
	"memex/internal/memex/storage/store"
	"memex/internal/memex/transaction"
)

// Repository represents a content repository
type Repository struct {
	path      string
	store     *store.ChunkStore
	nodeStore *store.ChunkStore
	txStore   *transaction.ActionStore
	lockMgr   sync.Mutex
}

// Ensure Repository implements transaction.Storage
var _ transaction.Storage = (*Repository)(nil)

// Create creates a new repository at the given path
func Create(path string) (*Repository, error) {
	// Create repository directory
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("creating repository directory: %w", err)
	}

	// Create repository instance
	repo := &Repository{
		path: path,
	}

	// Create transaction store first
	txStore, err := transaction.NewActionStore(repo)
	if err != nil {
		return nil, fmt.Errorf("creating transaction store: %w", err)
	}
	repo.txStore = txStore

	// Create chunker
	chunker := rabin.NewChunker()

	// Create store
	contentStore, err := store.NewStore(filepath.Join(path, "store"), chunker, txStore)
	if err != nil {
		return nil, fmt.Errorf("creating store: %w", err)
	}
	repo.store = contentStore

	// Create node store
	nodeStore, err := store.NewStore(filepath.Join(path, "nodes"), chunker, txStore)
	if err != nil {
		return nil, fmt.Errorf("creating node store: %w", err)
	}
	repo.nodeStore = nodeStore

	return repo, nil
}

// Open opens an existing repository
func Open(path string) (*Repository, error) {
	// Create repository instance
	repo := &Repository{
		path: path,
	}

	// Create transaction store first
	txStore, err := transaction.NewActionStore(repo)
	if err != nil {
		return nil, fmt.Errorf("creating transaction store: %w", err)
	}
	repo.txStore = txStore

	// Create chunker
	chunker := rabin.NewChunker()

	// Open store
	contentStore, err := store.NewStore(filepath.Join(path, "store"), chunker, txStore)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}
	repo.store = contentStore

	// Open node store
	nodeStore, err := store.NewStore(filepath.Join(path, "nodes"), chunker, txStore)
	if err != nil {
		return nil, fmt.Errorf("opening node store: %w", err)
	}
	repo.nodeStore = nodeStore

	return repo, nil
}

// Path returns the repository path (implements transaction.Storage)
func (r *Repository) Path() string {
	return r.path
}

// GetFile returns the underlying file for transaction storage (implements transaction.Storage)
func (r *Repository) GetFile() interface{} {
	return r.nodeStore.GetFile()
}

// GetLockManager returns the lock manager for transaction storage (implements transaction.Storage)
func (r *Repository) GetLockManager() interface{} {
	return &r.lockMgr
}

// AddNode adds a node to the repository
func (r *Repository) AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error) {
	// Store content
	chunks, err := r.store.Put(content)
	if err != nil {
		return "", fmt.Errorf("storing content: %w", err)
	}

	// Create node
	now := time.Now().UTC()
	node := &core.Node{
		Type:     nodeType,
		Content:  content,
		Meta:     meta,
		Created:  now,
		Modified: now,
	}

	// Add chunks to metadata
	if node.Meta == nil {
		node.Meta = make(map[string]interface{})
	} else {
		// Deep copy metadata to avoid modifying the original
		metaCopy := make(map[string]interface{})
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return "", fmt.Errorf("marshaling metadata: %w", err)
		}
		if err := json.Unmarshal(metaJSON, &metaCopy); err != nil {
			return "", fmt.Errorf("unmarshaling metadata: %w", err)
		}
		node.Meta = metaCopy
	}

	// Add chunks to metadata
	chunkHashes := make([]string, len(chunks))
	for i, chunk := range chunks {
		chunkHashes[i] = fmt.Sprintf("%x", chunk)
	}
	node.Meta["chunks"] = chunkHashes

	// Store node
	data, err := json.Marshal(node)
	if err != nil {
		return "", fmt.Errorf("marshaling node: %w", err)
	}

	nodeChunks, err := r.nodeStore.Put(data)
	if err != nil {
		return "", fmt.Errorf("storing node: %w", err)
	}

	// Use first chunk hash as node ID
	if len(nodeChunks) == 0 {
		return "", fmt.Errorf("no chunks generated for node")
	}
	node.ID = fmt.Sprintf("%x", nodeChunks[0])

	// Record action
	if err := r.txStore.RecordAction(transaction.ActionAddNode, map[string]any{
		"id":   node.ID,
		"type": nodeType,
		"meta": meta,
	}); err != nil {
		return "", fmt.Errorf("recording action: %w", err)
	}

	return node.ID, nil
}

// AddNodeWithID adds a node with a specific ID to the repository
func (r *Repository) AddNodeWithID(id string, content []byte, nodeType string, meta map[string]interface{}) error {
	// Store content
	chunks, err := r.store.Put(content)
	if err != nil {
		return fmt.Errorf("storing content: %w", err)
	}

	// Create node
	now := time.Now().UTC()
	node := &core.Node{
		ID:       id,
		Type:     nodeType,
		Content:  content,
		Meta:     meta,
		Created:  now,
		Modified: now,
	}

	// Add chunks to metadata
	if node.Meta == nil {
		node.Meta = make(map[string]interface{})
	} else {
		// Deep copy metadata to avoid modifying the original
		metaCopy := make(map[string]interface{})
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("marshaling metadata: %w", err)
		}
		if err := json.Unmarshal(metaJSON, &metaCopy); err != nil {
			return fmt.Errorf("unmarshaling metadata: %w", err)
		}
		node.Meta = metaCopy
	}

	// Add chunks to metadata
	chunkHashes := make([]string, len(chunks))
	for i, chunk := range chunks {
		chunkHashes[i] = fmt.Sprintf("%x", chunk)
	}
	node.Meta["chunks"] = chunkHashes

	// Store node
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("marshaling node: %w", err)
	}

	// Store node with specific ID
	if err := r.nodeStore.PutWithID(id, data); err != nil {
		return fmt.Errorf("storing node: %w", err)
	}

	// Record action
	if err := r.txStore.RecordAction(transaction.ActionAddNode, map[string]any{
		"id":   id,
		"type": nodeType,
		"meta": meta,
	}); err != nil {
		return fmt.Errorf("recording action: %w", err)
	}

	return nil
}

// GetNode retrieves a node from the repository
func (r *Repository) GetNode(id string) (*core.Node, error) {
	// Get node data
	data, err := r.nodeStore.Get([][]byte{[]byte(id)})
	if err != nil {
		// Try hex decoding if it's a hex string
		if len(id) == 64 { // Length of a hex-encoded SHA-256 hash
			if hashBytes, err := hex.DecodeString(id); err == nil {
				data, err = r.nodeStore.Get([][]byte{hashBytes})
				if err != nil {
					return nil, fmt.Errorf("getting node: %w", err)
				}
			}
		} else {
			return nil, fmt.Errorf("getting node: %w", err)
		}
	}

	// Parse node
	var node core.Node
	if err := json.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("parsing node: %w", err)
	}

	node.ID = id
	return &node, nil
}

// ListNodes returns a list of all node IDs in the repository
func (r *Repository) ListNodes() ([]string, error) {
	chunks, err := r.nodeStore.ListChunks()
	if err != nil {
		return nil, fmt.Errorf("listing chunks: %w", err)
	}

	ids := make([]string, len(chunks))
	for i, chunk := range chunks {
		ids[i] = fmt.Sprintf("%x", chunk)
	}
	return ids, nil
}

// DeleteNode removes a node from the repository
func (r *Repository) DeleteNode(id string) error {
	// Get node first to get chunk references
	node, err := r.GetNode(id)
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	// Delete node
	err = r.nodeStore.Delete([][]byte{[]byte(id)})
	if err != nil {
		// Try hex decoding if it's a hex string
		if len(id) == 64 { // Length of a hex-encoded SHA-256 hash
			if hashBytes, err := hex.DecodeString(id); err == nil {
				if err := r.nodeStore.Delete([][]byte{hashBytes}); err != nil {
					return fmt.Errorf("deleting node: %w", err)
				}
			}
		} else {
			return fmt.Errorf("deleting node: %w", err)
		}
	}

	// Delete content chunks
	if chunks, ok := node.Meta["chunks"].([]string); ok {
		chunkData := make([][]byte, len(chunks))
		for i, chunk := range chunks {
			hashBytes, err := hex.DecodeString(chunk)
			if err != nil {
				continue
			}
			chunkData[i] = hashBytes
		}
		if err := r.store.Delete(chunkData); err != nil {
			return fmt.Errorf("deleting chunks: %w", err)
		}
	}

	// Delete associated links
	links, err := r.GetLinks(id)
	if err != nil {
		return fmt.Errorf("getting links: %w", err)
	}
	for _, link := range links {
		if err := r.DeleteLink(link.Source, link.Target, link.Type); err != nil {
			return fmt.Errorf("deleting link: %w", err)
		}
	}

	// Record action
	if err := r.txStore.RecordAction(transaction.ActionDeleteNode, map[string]any{
		"id": id,
	}); err != nil {
		return fmt.Errorf("recording action: %w", err)
	}

	return nil
}

// AddLink creates a link between nodes
func (r *Repository) AddLink(source, target, linkType string, meta map[string]interface{}) error {
	// Verify nodes exist
	if _, err := r.GetNode(source); err != nil {
		return fmt.Errorf("getting source node: %w", err)
	}
	if _, err := r.GetNode(target); err != nil {
		return fmt.Errorf("getting target node: %w", err)
	}

	// Create link
	now := time.Now().UTC()
	link := &core.Link{
		Source:   source,
		Target:   target,
		Type:     linkType,
		Meta:     meta,
		Created:  now,
		Modified: now,
	}

	// Deep copy metadata to avoid modifying the original
	if meta != nil {
		metaCopy := make(map[string]interface{})
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("marshaling metadata: %w", err)
		}
		if err := json.Unmarshal(metaJSON, &metaCopy); err != nil {
			return fmt.Errorf("unmarshaling metadata: %w", err)
		}
		link.Meta = metaCopy
	}

	// Store link
	data, err := json.Marshal(link)
	if err != nil {
		return fmt.Errorf("marshaling link: %w", err)
	}

	chunks, err := r.nodeStore.Put(data)
	if err != nil {
		return fmt.Errorf("storing link: %w", err)
	}

	// Use first chunk hash as link ID
	if len(chunks) == 0 {
		return fmt.Errorf("no chunks generated for link")
	}

	// Record action with metadata
	if err := r.txStore.RecordAction(transaction.ActionAddLink, map[string]any{
		"source": source,
		"target": target,
		"type":   linkType,
		"meta":   meta,
	}); err != nil {
		return fmt.Errorf("recording action: %w", err)
	}

	return nil
}

// GetLinks returns all links for a node (both incoming and outgoing)
func (r *Repository) GetLinks(nodeID string) ([]*core.Link, error) {
	// List all chunks
	chunks, err := r.nodeStore.ListChunks()
	if err != nil {
		return nil, fmt.Errorf("listing chunks: %w", err)
	}

	// Filter and parse links
	var links []*core.Link
	for _, chunk := range chunks {
		// Get chunk data
		data, err := r.nodeStore.Get([][]byte{chunk})
		if err != nil {
			continue
		}

		// Try to parse as link
		var link core.Link
		if err := json.Unmarshal(data, &link); err != nil {
			continue
		}

		// Check if link is related to node (either as source or target)
		if link.Source == nodeID || link.Target == nodeID {
			links = append(links, &link)
		}
	}

	return links, nil
}

// DeleteLink removes a link
func (r *Repository) DeleteLink(source, target, linkType string) error {
	// List all chunks
	chunks, err := r.nodeStore.ListChunks()
	if err != nil {
		return fmt.Errorf("listing chunks: %w", err)
	}

	// Find and delete matching link
	for _, chunk := range chunks {
		// Get chunk data
		data, err := r.nodeStore.Get([][]byte{chunk})
		if err != nil {
			continue
		}

		// Try to parse as link
		var link core.Link
		if err := json.Unmarshal(data, &link); err != nil {
			continue
		}

		// Check if this is the link to delete
		if link.Source == source && link.Target == target && link.Type == linkType {
			if err := r.nodeStore.Delete([][]byte{chunk}); err != nil {
				return fmt.Errorf("deleting link: %w", err)
			}

			// Record action
			if err := r.txStore.RecordAction(transaction.ActionDeleteLink, map[string]any{
				"source": source,
				"target": target,
				"type":   linkType,
			}); err != nil {
				return fmt.Errorf("recording action: %w", err)
			}

			return nil
		}
	}

	return nil
}

// GetContent retrieves content from the repository
func (r *Repository) GetContent(id string) ([]byte, error) {
	hashBytes, err := hex.DecodeString(id)
	if err != nil {
		return nil, fmt.Errorf("parsing content ID: %w", err)
	}
	return r.store.Get([][]byte{hashBytes})
}

// Close closes the repository
func (r *Repository) Close() error {
	if err := r.store.Close(); err != nil {
		return fmt.Errorf("closing store: %w", err)
	}
	if err := r.nodeStore.Close(); err != nil {
		return fmt.Errorf("closing node store: %w", err)
	}
	if err := r.txStore.Close(); err != nil {
		return fmt.Errorf("closing transaction store: %w", err)
	}
	return nil
}
