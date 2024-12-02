package migration

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"memex/internal/memex/core"
	"memex/internal/memex/storage"
)

// Verifier handles export/import verification
type Verifier struct {
	store *storage.MXStore
}

// NewVerifier creates a new verifier
func NewVerifier(store *storage.MXStore) *Verifier {
	return &Verifier{
		store: store,
	}
}

// VerifyExport verifies an exported repository
func (v *Verifier) VerifyExport(export *Export) error {
	// Verify version
	if export.Version != Version {
		return fmt.Errorf("invalid export version: got %d, want %d", export.Version, Version)
	}

	// Verify timestamps
	if export.Created.After(time.Now()) {
		return fmt.Errorf("export created timestamp is in the future")
	}
	if export.Modified.After(time.Now()) {
		return fmt.Errorf("export modified timestamp is in the future")
	}
	if export.Modified.Before(export.Created) {
		return fmt.Errorf("export modified timestamp is before created timestamp")
	}

	// Verify nodes
	if err := v.verifyNodes(export.Nodes); err != nil {
		return fmt.Errorf("verifying nodes: %w", err)
	}

	// Verify links
	if err := v.verifyLinks(export.Links); err != nil {
		return fmt.Errorf("verifying links: %w", err)
	}

	// Verify chunks
	if err := v.verifyChunks(export.Chunks); err != nil {
		return fmt.Errorf("verifying chunks: %w", err)
	}

	return nil
}

// VerifyImport verifies an imported repository
func (v *Verifier) VerifyImport(export *Export) error {
	// Get all nodes from store
	storeNodes := v.store.Nodes()

	// Verify all exported nodes were imported
	for _, exportNode := range export.Nodes {
		found := false
		for _, storeNode := range storeNodes {
			if v.compareNodes(exportNode, storeNode) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("exported node %s not found in store", exportNode.ID)
		}
	}

	// Verify all links were imported
	for _, exportLink := range export.Links {
		links, err := v.store.GetLinks(exportLink.Source)
		if err != nil {
			return fmt.Errorf("getting links for node %s: %w", exportLink.Source, err)
		}

		found := false
		for _, storeLink := range links {
			if v.compareLinks(exportLink, storeLink) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("exported link %s -> %s not found in store", exportLink.Source, exportLink.Target)
		}
	}

	// Verify all chunks were imported
	for chunkHash := range export.Chunks {
		if _, err := v.store.GetChunk(chunkHash); err != nil {
			return fmt.Errorf("chunk %s not found in store", chunkHash)
		}
	}

	return nil
}

// VerifyConsistency verifies consistency between export and import
func (v *Verifier) VerifyConsistency(export *Export) error {
	// Verify node count matches
	storeNodes := v.store.Nodes()
	if len(storeNodes) != len(export.Nodes) {
		return fmt.Errorf("node count mismatch: store has %d, export has %d", len(storeNodes), len(export.Nodes))
	}

	// Verify content integrity
	for _, node := range storeNodes {
		contentHash, ok := node.Meta["content"].(string)
		if !ok {
			return fmt.Errorf("node %s missing content hash", node.ID)
		}

		content, err := v.store.ReconstructContent(contentHash)
		if err != nil {
			return fmt.Errorf("reconstructing content for node %s: %w", node.ID, err)
		}

		// Verify content hash
		hash := sha256.Sum256(content)
		if fmt.Sprintf("%x", hash) != contentHash {
			return fmt.Errorf("content hash mismatch for node %s", node.ID)
		}
	}

	return nil
}

// Internal verification methods

func (v *Verifier) verifyNodes(nodes []*core.Node) error {
	seen := make(map[string]bool)

	for _, node := range nodes {
		// Check for duplicate IDs
		if seen[node.ID] {
			return fmt.Errorf("duplicate node ID: %s", node.ID)
		}
		seen[node.ID] = true

		// Verify required fields
		if node.Type == "" {
			return fmt.Errorf("node %s missing type", node.ID)
		}

		// Verify timestamps
		if node.Created.After(time.Now()) {
			return fmt.Errorf("node %s created timestamp is in the future", node.ID)
		}
		if node.Modified.After(time.Now()) {
			return fmt.Errorf("node %s modified timestamp is in the future", node.ID)
		}
		if node.Modified.Before(node.Created) {
			return fmt.Errorf("node %s modified timestamp is before created timestamp", node.ID)
		}

		// Verify metadata
		if err := v.verifyNodeMetadata(node); err != nil {
			return fmt.Errorf("verifying node %s metadata: %w", node.ID, err)
		}
	}

	return nil
}

func (v *Verifier) verifyLinks(links []*core.Link) error {
	seen := make(map[string]bool)

	for _, link := range links {
		// Create unique key for link
		key := fmt.Sprintf("%s-%s-%s", link.Source, link.Target, link.Type)

		// Check for duplicate links
		if seen[key] {
			return fmt.Errorf("duplicate link: %s -> %s [%s]", link.Source, link.Target, link.Type)
		}
		seen[key] = true

		// Verify required fields
		if link.Source == "" {
			return fmt.Errorf("link missing source")
		}
		if link.Target == "" {
			return fmt.Errorf("link missing target")
		}
		if link.Type == "" {
			return fmt.Errorf("link missing type")
		}

		// Verify metadata size
		if link.Meta != nil {
			metaBytes, err := json.Marshal(link.Meta)
			if err != nil {
				return fmt.Errorf("marshaling link metadata: %w", err)
			}
			if len(metaBytes) > MaxMetaSize {
				return fmt.Errorf("link metadata too large: %d bytes (max %d)", len(metaBytes), MaxMetaSize)
			}
		}
	}

	return nil
}

func (v *Verifier) verifyChunks(chunks map[string]bool) error {
	for hash := range chunks {
		// Verify hash format
		if len(hash) != 64 {
			return fmt.Errorf("invalid chunk hash length: %s", hash)
		}

		// Get chunk content
		content, err := v.store.GetChunk(hash)
		if err != nil {
			return fmt.Errorf("getting chunk %s: %w", hash, err)
		}

		// Verify content hash
		contentHash := sha256.Sum256(content)
		if fmt.Sprintf("%x", contentHash) != hash {
			return fmt.Errorf("chunk hash mismatch: %s", hash)
		}
	}

	return nil
}

func (v *Verifier) verifyNodeMetadata(node *core.Node) error {
	if node.Meta == nil {
		return nil
	}

	// Verify metadata size
	metaBytes, err := json.Marshal(node.Meta)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}
	if len(metaBytes) > MaxMetaSize {
		return fmt.Errorf("metadata too large: %d bytes (max %d)", len(metaBytes), MaxMetaSize)
	}

	// Verify content hash if present
	if contentHash, ok := node.Meta["content"].(string); ok {
		if len(contentHash) != 64 {
			return fmt.Errorf("invalid content hash length: %s", contentHash)
		}
	}

	// Verify chunks if present
	if chunks, ok := node.Meta["chunks"].([]string); ok {
		for _, chunk := range chunks {
			if len(chunk) != 64 {
				return fmt.Errorf("invalid chunk hash length: %s", chunk)
			}
		}
	}

	return nil
}

func (v *Verifier) compareNodes(n1, n2 *core.Node) bool {
	// Compare basic fields
	if n1.Type != n2.Type {
		return false
	}

	// Compare metadata
	if len(n1.Meta) != len(n2.Meta) {
		return false
	}
	for k, v1 := range n1.Meta {
		v2, ok := n2.Meta[k]
		if !ok {
			return false
		}
		if fmt.Sprintf("%v", v1) != fmt.Sprintf("%v", v2) {
			return false
		}
	}

	return true
}

func (v *Verifier) compareLinks(l1, l2 *core.Link) bool {
	// Compare basic fields
	if l1.Source != l2.Source || l1.Target != l2.Target || l1.Type != l2.Type {
		return false
	}

	// Compare metadata
	if len(l1.Meta) != len(l2.Meta) {
		return false
	}
	for k, v1 := range l1.Meta {
		v2, ok := l2.Meta[k]
		if !ok {
			return false
		}
		if fmt.Sprintf("%v", v1) != fmt.Sprintf("%v", v2) {
			return false
		}
	}

	return true
}
