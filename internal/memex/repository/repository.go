package repository

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"memex/internal/memex/core"
	"memex/internal/memex/storage/rabin"
	"memex/internal/memex/storage/store"
	"memex/internal/memex/transaction"
)

// Repository implements core.Repository using Rabin DAG storage
type Repository struct {
	store   *store.ChunkStore
	txStore *transaction.ActionStore
	path    string
}

// Create creates a new repository
func Create(path string) (*Repository, error) {
	// Create chunk store first since it implements transaction.Storage
	chunker := rabin.NewChunker()
	chunkStore, err := store.NewStore(path, chunker, nil) // nil txStore initially
	if err != nil {
		return nil, fmt.Errorf("creating store: %w", err)
	}

	// Create transaction store using chunk store as storage
	txStore, err := transaction.NewActionStore(chunkStore)
	if err != nil {
		chunkStore.Close()
		return nil, fmt.Errorf("creating transaction store: %w", err)
	}

	// Update chunk store with transaction store
	chunkStore.SetTxStore(txStore)

	return &Repository{
		store:   chunkStore,
		txStore: txStore,
		path:    path,
	}, nil
}

// Open opens an existing repository
func Open(path string) (*Repository, error) {
	// Ensure path has .mx extension
	if filepath.Ext(path) != ".mx" {
		path += ".mx"
	}

	// Open chunk store first
	chunker := rabin.NewChunker()
	chunkStore, err := store.NewStore(path, chunker, nil) // nil txStore initially
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}

	// Create transaction store using chunk store as storage
	txStore, err := transaction.NewActionStore(chunkStore)
	if err != nil {
		chunkStore.Close()
		return nil, fmt.Errorf("opening transaction store: %w", err)
	}

	// Update chunk store with transaction store
	chunkStore.SetTxStore(txStore)

	return &Repository{
		store:   chunkStore,
		txStore: txStore,
		path:    path,
	}, nil
}

// Helper function to convert interface{} to float64
func toFloat64(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		if f, err := x.Float64(); err == nil {
			return f, true
		}
	case string:
		if f, err := strconv.ParseFloat(x, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// AddNode adds a new node
func (r *Repository) AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error) {
	// Store content first
	contentAddresses, err := r.store.Put(content)
	if err != nil {
		return "", fmt.Errorf("storing content: %w", err)
	}

	// Convert addresses to hex strings
	chunks := make([]string, len(contentAddresses))
	for i, addr := range contentAddresses {
		chunks[i] = hex.EncodeToString(addr)
	}

	// Create node metadata
	now := time.Now().UTC().Format(time.RFC3339Nano)
	nodeMeta := map[string]interface{}{
		"type":     nodeType,
		"created":  now,
		"modified": now,
		"chunks":   chunks,
	}
	if meta != nil {
		for k, v := range meta {
			if k != "chunks" { // Don't overwrite chunks
				// Convert numbers to float64
				if f, ok := toFloat64(v); ok {
					nodeMeta[k] = f
				} else {
					nodeMeta[k] = v
				}
			}
		}
	}

	// Store node metadata
	metaBytes, err := json.Marshal(nodeMeta)
	if err != nil {
		return "", fmt.Errorf("marshaling metadata: %w", err)
	}

	// Store metadata
	metaAddresses, err := r.store.Put(metaBytes)
	if err != nil {
		return "", fmt.Errorf("storing metadata: %w", err)
	}

	// Use first metadata chunk address as node ID
	return hex.EncodeToString(metaAddresses[0]), nil
}

// GetNode retrieves a node
func (r *Repository) GetNode(id string) (*core.Node, error) {
	// Convert ID from hex string to bytes
	metaAddr, err := hex.DecodeString(id)
	if err != nil {
		return nil, fmt.Errorf("parsing node ID: %w", err)
	}

	// Get node metadata
	metaBytes, err := r.store.Get([][]byte{metaAddr})
	if err != nil {
		return nil, fmt.Errorf("getting metadata: %w", err)
	}

	// Parse metadata
	var meta map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(metaBytes))
	dec.UseNumber() // Preserve number formats
	if err := dec.Decode(&meta); err != nil {
		return nil, fmt.Errorf("unmarshaling metadata: %w", err)
	}

	// Convert numbers to float64
	for k, v := range meta {
		if f, ok := toFloat64(v); ok {
			meta[k] = f
		}
	}

	// Get content chunks
	var addresses [][]byte
	if chunks, ok := meta["chunks"].([]interface{}); ok {
		addresses = make([][]byte, len(chunks))
		for i, chunk := range chunks {
			if chunkStr, ok := chunk.(string); ok {
				addr, err := hex.DecodeString(chunkStr)
				if err != nil {
					return nil, fmt.Errorf("parsing chunk address: %w", err)
				}
				addresses[i] = addr
			}
		}
	}

	// Get content
	content, err := r.store.Get(addresses)
	if err != nil {
		return nil, fmt.Errorf("getting content: %w", err)
	}

	// Parse timestamps
	created, _ := time.Parse(time.RFC3339Nano, meta["created"].(string))
	modified, _ := time.Parse(time.RFC3339Nano, meta["modified"].(string))

	return &core.Node{
		ID:       id,
		Type:     meta["type"].(string),
		Content:  content,
		Meta:     meta,
		Created:  created,
		Modified: modified,
	}, nil
}

// DeleteNode removes a node and its associated links
func (r *Repository) DeleteNode(id string) error {
	// Get node first
	node, err := r.GetNode(id)
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	// Delete all links associated with this node
	chunks, err := r.store.ListChunks()
	if err != nil {
		return fmt.Errorf("listing chunks: %w", err)
	}

	for _, chunkID := range chunks {
		// Get chunk data
		data, err := r.store.Get([][]byte{chunkID})
		if err != nil {
			continue
		}

		// Try to parse as link metadata
		var meta map[string]interface{}
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}

		// Check if this is a link chunk
		if isLink, ok := meta["isLink"].(bool); !ok || !isLink {
			continue
		}

		// Check if this link involves our node
		source, _ := meta["source"].(string)
		target, _ := meta["target"].(string)
		if source == id || target == id {
			// Delete this link
			if err := r.store.Delete([][]byte{chunkID}); err != nil {
				return fmt.Errorf("deleting associated link: %w", err)
			}
		}
	}

	// Get chunk addresses
	var addresses [][]byte
	if chunks, ok := node.Meta["chunks"].([]interface{}); ok {
		addresses = make([][]byte, len(chunks))
		for i, chunk := range chunks {
			if chunkStr, ok := chunk.(string); ok {
				addr, err := hex.DecodeString(chunkStr)
				if err != nil {
					return fmt.Errorf("parsing chunk address: %w", err)
				}
				addresses[i] = addr
			}
		}
	}

	// Delete content chunks
	if err := r.store.Delete(addresses); err != nil {
		return fmt.Errorf("deleting content: %w", err)
	}

	// Delete metadata chunk
	metaAddr, err := hex.DecodeString(id)
	if err != nil {
		return fmt.Errorf("parsing metadata address: %w", err)
	}
	if err := r.store.Delete([][]byte{metaAddr}); err != nil {
		return fmt.Errorf("deleting metadata: %w", err)
	}

	return nil
}

// AddLink creates a link between nodes
func (r *Repository) AddLink(source, target, linkType string, meta map[string]interface{}) error {
	// Verify nodes exist
	if _, err := r.GetNode(source); err != nil {
		return fmt.Errorf("source node not found: %w", err)
	}
	if _, err := r.GetNode(target); err != nil {
		return fmt.Errorf("target node not found: %w", err)
	}

	// Create link metadata
	now := time.Now().UTC().Format(time.RFC3339Nano)
	linkMeta := map[string]interface{}{
		"source":   source,
		"target":   target,
		"type":     linkType,
		"created":  now,
		"modified": now,
		"isLink":   true, // Mark this chunk as a link
	}
	if meta != nil {
		for k, v := range meta {
			if k != "isLink" { // Don't overwrite isLink flag
				// Convert numbers to float64
				if f, ok := toFloat64(v); ok {
					linkMeta[k] = f
				} else {
					linkMeta[k] = v
				}
			}
		}
	}

	// Store link metadata
	metaBytes, err := json.Marshal(linkMeta)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	// Store metadata
	_, err = r.store.Put(metaBytes)
	if err != nil {
		return fmt.Errorf("storing link: %w", err)
	}

	return nil
}

// GetLinks retrieves links for a node
func (r *Repository) GetLinks(nodeID string) ([]*core.Link, error) {
	// Get all chunks and look for link metadata
	chunks, err := r.store.ListChunks()
	if err != nil {
		return nil, fmt.Errorf("listing chunks: %w", err)
	}

	var links []*core.Link
	for _, chunkID := range chunks {
		// Get chunk data
		data, err := r.store.Get([][]byte{chunkID})
		if err != nil {
			continue // Skip chunks we can't read
		}

		// Try to parse as link metadata
		var meta map[string]interface{}
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber() // Preserve number formats
		if err := dec.Decode(&meta); err != nil {
			continue // Skip non-JSON chunks
		}

		// Convert numbers to float64
		for k, v := range meta {
			if f, ok := toFloat64(v); ok {
				meta[k] = f
			}
		}

		// Check if this is a link chunk
		if isLink, ok := meta["isLink"].(bool); !ok || !isLink {
			continue
		}

		// Check if this link involves our node
		source, _ := meta["source"].(string)
		target, _ := meta["target"].(string)
		if source != nodeID && target != nodeID {
			continue
		}

		// Parse timestamps with nanosecond precision
		created, _ := time.Parse(time.RFC3339Nano, meta["created"].(string))
		modified, _ := time.Parse(time.RFC3339Nano, meta["modified"].(string))

		// Create link
		link := &core.Link{
			Source:   source,
			Target:   target,
			Type:     meta["type"].(string),
			Meta:     meta,
			Created:  created,
			Modified: modified,
		}

		links = append(links, link)
	}

	// Sort links by creation time and then by order field
	sort.SliceStable(links, func(i, j int) bool {
		if links[i].Created.Equal(links[j].Created) {
			// If timestamps are equal, use order field
			orderI, _ := toFloat64(links[i].Meta["order"])
			orderJ, _ := toFloat64(links[j].Meta["order"])
			return orderI < orderJ
		}
		return links[i].Created.Before(links[j].Created)
	})

	return links, nil
}

// DeleteLink removes a link
func (r *Repository) DeleteLink(source, target, linkType string) error {
	// Get all chunks and look for matching link
	chunks, err := r.store.ListChunks()
	if err != nil {
		return fmt.Errorf("listing chunks: %w", err)
	}

	for _, chunkID := range chunks {
		// Get chunk data
		data, err := r.store.Get([][]byte{chunkID})
		if err != nil {
			continue
		}

		// Try to parse as link metadata
		var meta map[string]interface{}
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}

		// Check if this is our link
		if isLink, ok := meta["isLink"].(bool); !ok || !isLink {
			continue
		}

		if meta["source"] == source && meta["target"] == target && meta["type"] == linkType {
			// Delete this chunk
			if err := r.store.Delete([][]byte{chunkID}); err != nil {
				return fmt.Errorf("deleting link: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("link not found")
}

// GetContent retrieves content by ID
func (r *Repository) GetContent(id string) ([]byte, error) {
	node, err := r.GetNode(id)
	if err != nil {
		return nil, fmt.Errorf("getting node: %w", err)
	}
	return node.Content, nil
}

// Close closes the repository
func (r *Repository) Close() error {
	if err := r.store.Close(); err != nil {
		return fmt.Errorf("closing store: %w", err)
	}
	return nil
}
