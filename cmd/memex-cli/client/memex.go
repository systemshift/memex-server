package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MemexClient handles communication with the Memex API
type MemexClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewMemexClient creates a new Memex client
func NewMemexClient(baseURL string) *MemexClient {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &MemexClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Search performs full-text search across all nodes
func (c *MemexClient) Search(query string, limit int) ([]Node, error) {
	if limit <= 0 {
		limit = 10
	}

	endpoint := fmt.Sprintf("%s/api/query/search?q=%s&limit=%d",
		c.baseURL, url.QueryEscape(query), limit)

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	return result.Nodes, nil
}

// GetNode retrieves a specific node by ID
func (c *MemexClient) GetNode(id string) (*Node, error) {
	endpoint := fmt.Sprintf("%s/api/nodes/%s", c.baseURL, url.PathEscape(id))

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("get node request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("node not found: %s", id)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get node failed with status %d: %s", resp.StatusCode, string(body))
	}

	var node Node
	if err := json.NewDecoder(resp.Body).Decode(&node); err != nil {
		return nil, fmt.Errorf("failed to decode node: %w", err)
	}

	return &node, nil
}

// GetLinks retrieves all links for a node
func (c *MemexClient) GetLinks(nodeID string) ([]Link, error) {
	endpoint := fmt.Sprintf("%s/api/nodes/%s/links", c.baseURL, url.PathEscape(nodeID))

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("get links request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get links failed with status %d: %s", resp.StatusCode, string(body))
	}

	var links []Link
	if err := json.NewDecoder(resp.Body).Decode(&links); err != nil {
		return nil, fmt.Errorf("failed to decode links: %w", err)
	}

	return links, nil
}

// Traverse performs graph traversal from a starting node
func (c *MemexClient) Traverse(startID string, depth int, relTypes []string) (*Subgraph, error) {
	if depth <= 0 {
		depth = 2
	}

	endpoint := fmt.Sprintf("%s/api/query/traverse?start=%s&depth=%d",
		c.baseURL, url.QueryEscape(startID), depth)

	if len(relTypes) > 0 {
		endpoint += "&rel_type=" + url.QueryEscape(strings.Join(relTypes, ","))
	}

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("traverse request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("traverse failed with status %d: %s", resp.StatusCode, string(body))
	}

	var subgraph Subgraph
	if err := json.NewDecoder(resp.Body).Decode(&subgraph); err != nil {
		return nil, fmt.Errorf("failed to decode subgraph: %w", err)
	}

	return &subgraph, nil
}

// Filter filters nodes by type and properties
func (c *MemexClient) Filter(nodeType, key, value string, limit int) ([]Node, error) {
	if limit <= 0 {
		limit = 10
	}

	params := url.Values{}
	if nodeType != "" {
		params.Set("type", nodeType)
	}
	if key != "" {
		params.Set("key", key)
	}
	if value != "" {
		params.Set("value", value)
	}
	params.Set("limit", fmt.Sprintf("%d", limit))

	endpoint := fmt.Sprintf("%s/api/query/filter?%s", c.baseURL, params.Encode())

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("filter request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("filter failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode filter results: %w", err)
	}

	return result.Nodes, nil
}

// GetSubgraph gets the neighborhood around a node
func (c *MemexClient) GetSubgraph(startID string, depth int) (*Subgraph, error) {
	if depth <= 0 {
		depth = 1
	}

	endpoint := fmt.Sprintf("%s/api/query/subgraph?start=%s&depth=%d",
		c.baseURL, url.QueryEscape(startID), depth)

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("subgraph request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("subgraph failed with status %d: %s", resp.StatusCode, string(body))
	}

	var subgraph Subgraph
	if err := json.NewDecoder(resp.Body).Decode(&subgraph); err != nil {
		return nil, fmt.Errorf("failed to decode subgraph: %w", err)
	}

	return &subgraph, nil
}

// ListNodes lists all nodes (with optional type filter)
func (c *MemexClient) ListNodes(nodeType string, limit int) ([]Node, error) {
	if limit <= 0 {
		limit = 50
	}

	endpoint := fmt.Sprintf("%s/api/nodes?limit=%d", c.baseURL, limit)
	if nodeType != "" {
		endpoint += "&type=" + url.QueryEscape(nodeType)
	}

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("list nodes request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list nodes failed with status %d: %s", resp.StatusCode, string(body))
	}

	var nodes []Node
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		return nil, fmt.Errorf("failed to decode nodes: %w", err)
	}

	return nodes, nil
}

// Health checks if the Memex server is running
func (c *MemexClient) Health() error {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("memex server unhealthy: status %d", resp.StatusCode)
	}

	return nil
}
