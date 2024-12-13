package repository

import (
	"fmt"

	"memex/pkg/sdk/types"
)

// Client provides access to memex repository operations
type Client struct {
	repo types.Repository
}

// NewClient creates a new repository client
func NewClient(repo types.Repository) *Client {
	return &Client{
		repo: repo,
	}
}

// AddNode adds a node to the repository
func (c *Client) AddNode(content []byte, nodeType string, meta types.Meta) (string, error) {
	return c.repo.AddNode(content, nodeType, meta)
}

// GetNode retrieves a node by ID
func (c *Client) GetNode(id string) (*types.Node, error) {
	return c.repo.GetNode(id)
}

// DeleteNode removes a node
func (c *Client) DeleteNode(id string) error {
	return c.repo.DeleteNode(id)
}

// AddLink creates a link between nodes
func (c *Client) AddLink(source, target, linkType string, meta types.Meta) error {
	return c.repo.AddLink(source, target, linkType, meta)
}

// GetLinks returns all links for a node
func (c *Client) GetLinks(nodeID string) ([]*types.Link, error) {
	return c.repo.GetLinks(nodeID)
}

// DeleteLink removes a link
func (c *Client) DeleteLink(source, target, linkType string) error {
	return c.repo.DeleteLink(source, target, linkType)
}

// QueryNodes searches for nodes
func (c *Client) QueryNodes(query types.Query) ([]*types.Node, error) {
	return c.repo.QueryNodes(query)
}

// QueryLinks searches for links
func (c *Client) QueryLinks(query types.Query) ([]*types.Link, error) {
	return c.repo.QueryLinks(query)
}

// QueryByModule returns all nodes and links created by a module
func (c *Client) QueryByModule(moduleID string) ([]*types.Node, []*types.Link, error) {
	query := types.Query{
		ModuleID: moduleID,
	}

	nodes, err := c.QueryNodes(query)
	if err != nil {
		return nil, nil, fmt.Errorf("querying nodes: %w", err)
	}

	links, err := c.QueryLinks(query)
	if err != nil {
		return nil, nil, fmt.Errorf("querying links: %w", err)
	}

	return nodes, links, nil
}

// Helper methods for common operations

// AddNodeWithMeta adds a node with module metadata
func (c *Client) AddNodeWithMeta(content []byte, nodeType string, moduleID string, meta types.Meta) (string, error) {
	if meta == nil {
		meta = types.Meta{}
	}
	meta["module"] = moduleID
	return c.AddNode(content, nodeType, meta)
}

// AddLinkWithMeta adds a link with module metadata
func (c *Client) AddLinkWithMeta(source, target, linkType string, moduleID string, meta types.Meta) error {
	if meta == nil {
		meta = types.Meta{}
	}
	meta["module"] = moduleID
	return c.AddLink(source, target, linkType, meta)
}

// GetNodeContent retrieves a node's content
func (c *Client) GetNodeContent(id string) ([]byte, error) {
	node, err := c.GetNode(id)
	if err != nil {
		return nil, err
	}
	return node.Content, nil
}

// GetNodeMeta retrieves a node's metadata
func (c *Client) GetNodeMeta(id string) (types.Meta, error) {
	node, err := c.GetNode(id)
	if err != nil {
		return nil, err
	}
	return node.Meta, nil
}
