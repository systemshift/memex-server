package sdk

import (
	"fmt"

	"memex/pkg/types"
)

// RepositoryHelper provides common repository operations for modules
type RepositoryHelper struct {
	repo types.Repository
}

// NewRepositoryHelper creates a new repository helper
func NewRepositoryHelper(repo types.Repository) *RepositoryHelper {
	return &RepositoryHelper{repo: repo}
}

// AddNodeWithMeta adds a node with module metadata
func (h *RepositoryHelper) AddNodeWithMeta(content []byte, nodeType string, moduleID string, meta map[string]interface{}) (string, error) {
	if meta == nil {
		meta = make(map[string]interface{})
	}
	meta["module"] = moduleID
	return h.repo.AddNode(content, nodeType, meta)
}

// GetModuleNodes returns all nodes created by a module
func (h *RepositoryHelper) GetModuleNodes(moduleID string) ([]*types.Node, error) {
	return h.repo.QueryNodesByModule(moduleID)
}

// GetModuleLinks returns all links created by a module
func (h *RepositoryHelper) GetModuleLinks(moduleID string) ([]*types.Link, error) {
	return h.repo.QueryLinksByModule(moduleID)
}

// AddModuleLink adds a link with module metadata
func (h *RepositoryHelper) AddModuleLink(source, target, linkType string, moduleID string, meta map[string]interface{}) error {
	if meta == nil {
		meta = make(map[string]interface{})
	}
	meta["module"] = moduleID
	return h.repo.AddLink(source, target, linkType, meta)
}

// DeleteModuleNodes deletes all nodes created by a module
func (h *RepositoryHelper) DeleteModuleNodes(moduleID string) error {
	nodes, err := h.repo.QueryNodesByModule(moduleID)
	if err != nil {
		return fmt.Errorf("querying module nodes: %w", err)
	}

	for _, node := range nodes {
		if err := h.repo.DeleteNode(node.ID); err != nil {
			return fmt.Errorf("deleting node %s: %w", node.ID, err)
		}
	}

	return nil
}

// DeleteModuleLinks deletes all links created by a module
func (h *RepositoryHelper) DeleteModuleLinks(moduleID string) error {
	links, err := h.repo.QueryLinksByModule(moduleID)
	if err != nil {
		return fmt.Errorf("querying module links: %w", err)
	}

	for _, link := range links {
		if err := h.repo.DeleteLink(link.Source, link.Target, link.Type); err != nil {
			return fmt.Errorf("deleting link %s -> %s: %w", link.Source, link.Target, err)
		}
	}

	return nil
}

// CleanupModule removes all nodes and links created by a module
func (h *RepositoryHelper) CleanupModule(moduleID string) error {
	if err := h.DeleteModuleLinks(moduleID); err != nil {
		return fmt.Errorf("deleting module links: %w", err)
	}

	if err := h.DeleteModuleNodes(moduleID); err != nil {
		return fmt.Errorf("deleting module nodes: %w", err)
	}

	return nil
}
