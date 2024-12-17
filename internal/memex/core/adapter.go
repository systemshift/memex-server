package core

import (
	"memex/pkg/sdk/types"
)

// RepositoryAdapter adapts core.Repository to types.Repository
type RepositoryAdapter struct {
	repo Repository
}

// NewRepositoryAdapter creates a new repository adapter
func NewRepositoryAdapter(repo Repository) types.Repository {
	return &RepositoryAdapter{repo: repo}
}

// Convert core.Node to types.Node
func (a *RepositoryAdapter) toSDKNode(node *Node) *types.Node {
	if node == nil {
		return nil
	}
	return &types.Node{
		ID:      node.ID,
		Type:    node.Type,
		Content: node.Content,
		Meta:    node.Meta,
	}
}

// Convert core.Link to types.Link
func (a *RepositoryAdapter) toSDKLink(link *Link) *types.Link {
	if link == nil {
		return nil
	}
	return &types.Link{
		Source: link.Source,
		Target: link.Target,
		Type:   link.Type,
		Meta:   link.Meta,
	}
}

// Convert []*core.Link to []*types.Link
func (a *RepositoryAdapter) toSDKLinks(links []*Link) []*types.Link {
	if links == nil {
		return nil
	}
	result := make([]*types.Link, len(links))
	for i, link := range links {
		result[i] = a.toSDKLink(link)
	}
	return result
}

// AddNode implements types.Repository
func (a *RepositoryAdapter) AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error) {
	return a.repo.AddNode(content, nodeType, meta)
}

// GetNode implements types.Repository
func (a *RepositoryAdapter) GetNode(id string) (*types.Node, error) {
	node, err := a.repo.GetNode(id)
	if err != nil {
		return nil, err
	}
	return a.toSDKNode(node), nil
}

// DeleteNode implements types.Repository
func (a *RepositoryAdapter) DeleteNode(id string) error {
	return a.repo.DeleteNode(id)
}

// AddLink implements types.Repository
func (a *RepositoryAdapter) AddLink(source, target, linkType string, meta map[string]interface{}) error {
	return a.repo.AddLink(source, target, linkType, meta)
}

// GetLinks implements types.Repository
func (a *RepositoryAdapter) GetLinks(nodeID string) ([]*types.Link, error) {
	links, err := a.repo.GetLinks(nodeID)
	if err != nil {
		return nil, err
	}
	return a.toSDKLinks(links), nil
}

// DeleteLink implements types.Repository
func (a *RepositoryAdapter) DeleteLink(source, target, linkType string) error {
	return a.repo.DeleteLink(source, target, linkType)
}

// QueryNodes implements types.Repository
func (a *RepositoryAdapter) QueryNodes(query types.Query) ([]*types.Node, error) {
	// Convert query to module-specific query
	nodes, err := a.repo.QueryNodesByModule(query.ModuleID)
	if err != nil {
		return nil, err
	}

	// Convert nodes
	result := make([]*types.Node, len(nodes))
	for i, node := range nodes {
		result[i] = a.toSDKNode(node)
	}
	return result, nil
}

// QueryLinks implements types.Repository
func (a *RepositoryAdapter) QueryLinks(query types.Query) ([]*types.Link, error) {
	// Convert query to module-specific query
	links, err := a.repo.QueryLinksByModule(query.ModuleID)
	if err != nil {
		return nil, err
	}

	// Convert links
	return a.toSDKLinks(links), nil
}
