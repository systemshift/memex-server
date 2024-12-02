package node

import (
	"encoding/json"
	"fmt"
)

// MetadataManager handles node metadata operations
type MetadataManager struct {
	store *Store
}

// NewMetadataManager creates a new metadata manager
func NewMetadataManager(store *Store) *MetadataManager {
	return &MetadataManager{
		store: store,
	}
}

// ValidateMetadata validates node metadata
func (m *MetadataManager) ValidateMetadata(meta map[string]any) error {
	// Check if metadata is nil
	if meta == nil {
		return nil
	}

	// Marshal metadata to check size
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	// Check size
	if len(metaBytes) > MaxMetaSize {
		return fmt.Errorf("metadata too large: %d bytes (max %d)", len(metaBytes), MaxMetaSize)
	}

	return nil
}

// UpdateMetadata updates a node's metadata
func (m *MetadataManager) UpdateMetadata(id string, meta map[string]any) error {
	// Validate metadata
	if err := m.ValidateMetadata(meta); err != nil {
		return fmt.Errorf("validating metadata: %w", err)
	}

	// Get existing node
	node, err := m.store.Get(id)
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	// Update metadata
	node.Meta = meta

	// Convert back to NodeData
	var nodeData NodeData
	if err := nodeData.FromCore(node); err != nil {
		return fmt.Errorf("converting node: %w", err)
	}

	// Write updated node
	offset, err := m.store.writeNode(nodeData)
	if err != nil {
		return fmt.Errorf("writing node: %w", err)
	}

	// Update index
	entry := IndexEntry{
		ID:     nodeData.ID,
		Offset: offset,
		Length: uint32(nodeData.Size()),
		Flags:  FlagModified,
	}

	// Remove old entry and add new one
	if oldEntry, found := m.store.index.FindByString(id); found {
		m.store.index.Remove(oldEntry.ID)
		m.store.index.Add(entry)
	} else {
		return fmt.Errorf("node index entry not found: %s", id)
	}

	return nil
}

// GetMetadataField gets a specific metadata field
func (m *MetadataManager) GetMetadataField(id string, field string) (any, error) {
	// Get node
	node, err := m.store.Get(id)
	if err != nil {
		return nil, fmt.Errorf("getting node: %w", err)
	}

	// Check if metadata exists
	if node.Meta == nil {
		return nil, fmt.Errorf("node has no metadata")
	}

	// Get field
	value, ok := node.Meta[field]
	if !ok {
		return nil, fmt.Errorf("metadata field not found: %s", field)
	}

	return value, nil
}

// SetMetadataField sets a specific metadata field
func (m *MetadataManager) SetMetadataField(id string, field string, value any) error {
	// Get node
	node, err := m.store.Get(id)
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	// Initialize metadata if nil
	if node.Meta == nil {
		node.Meta = make(map[string]any)
	}

	// Set field
	node.Meta[field] = value

	// Validate updated metadata
	if err := m.ValidateMetadata(node.Meta); err != nil {
		return fmt.Errorf("validating metadata: %w", err)
	}

	// Update node with new metadata
	return m.UpdateMetadata(id, node.Meta)
}

// RemoveMetadataField removes a specific metadata field
func (m *MetadataManager) RemoveMetadataField(id string, field string) error {
	// Get node
	node, err := m.store.Get(id)
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	// Check if metadata exists
	if node.Meta == nil {
		return fmt.Errorf("node has no metadata")
	}

	// Check if field exists
	if _, ok := node.Meta[field]; !ok {
		return fmt.Errorf("metadata field not found: %s", field)
	}

	// Remove field
	delete(node.Meta, field)

	// Update node with new metadata
	return m.UpdateMetadata(id, node.Meta)
}
