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

// initRoot creates initial root state if it doesn't exist
func (s *DAGStore) initRoot() error {
	rootPath := filepath.Join(s.path, "root.json")
	log.Printf("Initializing root state at %s", rootPath)

	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		// Create initial root with empty hash
		hasher := sha256.New()
		root := core.Root{
			Hash:     hex.EncodeToString(hasher.Sum(nil)),
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

// GetRoot returns the current root state
func (s *DAGStore) GetRoot() (core.Root, error) {
	rootPath := filepath.Join(s.path, "root.json")
	data, err := os.ReadFile(rootPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty root if no state exists
			root := core.Root{
				Modified: time.Now(),
				Nodes:    []string{},
			}
			// Calculate initial hash
			hasher := sha256.New()
			root.Hash = hex.EncodeToString(hasher.Sum(nil))
			return root, nil
		}
		return core.Root{}, fmt.Errorf("reading root state: %w", err)
	}

	var root core.Root
	if err := json.Unmarshal(data, &root); err != nil {
		return core.Root{}, fmt.Errorf("parsing root state: %w", err)
	}

	// Calculate root hash
	hasher := sha256.New()
	for _, id := range root.Nodes {
		node, err := s.GetNode(id)
		if err != nil {
			continue
		}
		hasher.Write([]byte(node.Current)) // Hash includes current version of each node
	}
	root.Hash = hex.EncodeToString(hasher.Sum(nil))

	return root, nil
}

// UpdateRoot recalculates and stores the root hash
func (s *DAGStore) UpdateRoot() error {
	// List all nodes in the nodes directory
	var nodes []string
	nodesDir := filepath.Join(s.path, "nodes")
	entries, err := os.ReadDir(nodesDir)
	if err != nil {
		if os.IsNotExist(err) {
			nodes = []string{}
		} else {
			return fmt.Errorf("reading nodes directory: %w", err)
		}
	}

	// Get node IDs from filenames
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			// Remove .json extension to get ID
			id := entry.Name()[:len(entry.Name())-5]
			nodes = append(nodes, id)
		}
	}

	// Create root object
	root := core.Root{
		Modified: time.Now(),
		Nodes:    nodes,
	}

	// Calculate root hash
	hasher := sha256.New()
	for _, id := range nodes {
		node, err := s.GetNode(id)
		if err != nil {
			continue
		}
		hasher.Write([]byte(node.Current)) // Hash includes current version of each node
	}
	root.Hash = hex.EncodeToString(hasher.Sum(nil))

	// Store updated root
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling root: %w", err)
	}

	rootPath := filepath.Join(s.path, "root.json")
	if err := os.WriteFile(rootPath, data, 0644); err != nil {
		return fmt.Errorf("writing root state: %w", err)
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
