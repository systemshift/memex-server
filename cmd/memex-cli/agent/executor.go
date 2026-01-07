package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/systemshift/memex/cmd/memex-cli/client"
)

// Executor executes tools against the Memex API
type Executor struct {
	memex *client.MemexClient
}

// NewExecutor creates a new tool executor
func NewExecutor(memex *client.MemexClient) *Executor {
	return &Executor{memex: memex}
}

// Execute runs a tool and returns the result as a string
func (e *Executor) Execute(toolName string, input map[string]interface{}) (string, error) {
	switch toolName {
	case "search":
		return e.executeSearch(input)
	case "get_node":
		return e.executeGetNode(input)
	case "get_links":
		return e.executeGetLinks(input)
	case "traverse":
		return e.executeTraverse(input)
	case "filter":
		return e.executeFilter(input)
	case "list_nodes":
		return e.executeListNodes(input)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (e *Executor) executeSearch(input map[string]interface{}) (string, error) {
	query, _ := input["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	limit := 10
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}

	nodes, err := e.memex.Search(query, limit)
	if err != nil {
		return "", err
	}

	if len(nodes) == 0 {
		return "No results found for query: " + query, nil
	}

	return formatNodes(nodes), nil
}

func (e *Executor) executeGetNode(input map[string]interface{}) (string, error) {
	id, _ := input["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	node, err := e.memex.GetNode(id)
	if err != nil {
		return "", err
	}

	return formatNode(node), nil
}

func (e *Executor) executeGetLinks(input map[string]interface{}) (string, error) {
	id, _ := input["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	links, err := e.memex.GetLinks(id)
	if err != nil {
		return "", err
	}

	if len(links) == 0 {
		return fmt.Sprintf("No links found for node: %s", id), nil
	}

	return formatLinks(links, id), nil
}

func (e *Executor) executeTraverse(input map[string]interface{}) (string, error) {
	start, _ := input["start"].(string)
	if start == "" {
		return "", fmt.Errorf("start is required")
	}

	depth := 2
	if d, ok := input["depth"].(float64); ok {
		depth = int(d)
		if depth > 5 {
			depth = 5
		}
	}

	var relTypes []string
	if rt, ok := input["rel_types"].(string); ok && rt != "" {
		relTypes = strings.Split(rt, ",")
	}

	subgraph, err := e.memex.Traverse(start, depth, relTypes)
	if err != nil {
		return "", err
	}

	return formatSubgraph(subgraph), nil
}

func (e *Executor) executeFilter(input map[string]interface{}) (string, error) {
	nodeType, _ := input["type"].(string)
	key, _ := input["key"].(string)
	value, _ := input["value"].(string)

	limit := 10
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}

	nodes, err := e.memex.Filter(nodeType, key, value, limit)
	if err != nil {
		return "", err
	}

	if len(nodes) == 0 {
		return "No nodes found matching filter criteria", nil
	}

	return formatNodes(nodes), nil
}

func (e *Executor) executeListNodes(input map[string]interface{}) (string, error) {
	nodeType, _ := input["type"].(string)

	limit := 20
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}

	nodes, err := e.memex.ListNodes(nodeType, limit)
	if err != nil {
		return "", err
	}

	if len(nodes) == 0 {
		if nodeType != "" {
			return fmt.Sprintf("No nodes of type '%s' found", nodeType), nil
		}
		return "No nodes found in the graph", nil
	}

	return formatNodes(nodes), nil
}

// formatNodes formats a list of nodes for display
func formatNodes(nodes []client.Node) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d nodes:\n\n", len(nodes)))

	for i, node := range nodes {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, node.Type, node.ID))

		// Show key metadata
		if title := getStringMeta(node.Meta, "title", "name"); title != "" {
			sb.WriteString(fmt.Sprintf("   Title: %s\n", title))
		}
		if node.Content != "" && len(node.Content) < 200 {
			sb.WriteString(fmt.Sprintf("   Content: %s\n", node.Content))
		}

		// Show other interesting metadata
		for key, val := range node.Meta {
			if key != "title" && key != "name" && key != "content" {
				if str, ok := val.(string); ok && len(str) < 100 {
					sb.WriteString(fmt.Sprintf("   %s: %s\n", key, str))
				}
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatNode formats a single node with full details
func formatNode(node *client.Node) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Node: %s\n", node.ID))
	sb.WriteString(fmt.Sprintf("Type: %s\n", node.Type))

	if !node.Created.IsZero() {
		sb.WriteString(fmt.Sprintf("Created: %s\n", node.Created.Format("2006-01-02 15:04")))
	}
	if !node.Modified.IsZero() {
		sb.WriteString(fmt.Sprintf("Modified: %s\n", node.Modified.Format("2006-01-02 15:04")))
	}

	if node.Content != "" {
		sb.WriteString(fmt.Sprintf("\nContent:\n%s\n", node.Content))
	}

	if len(node.Meta) > 0 {
		sb.WriteString("\nMetadata:\n")
		metaJSON, _ := json.MarshalIndent(node.Meta, "  ", "  ")
		sb.WriteString(fmt.Sprintf("  %s\n", string(metaJSON)))
	}

	return sb.String()
}

// formatLinks formats links for display
func formatLinks(links []client.Link, nodeID string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Links for %s:\n\n", nodeID))

	outgoing := []client.Link{}
	incoming := []client.Link{}

	for _, link := range links {
		if link.Source == nodeID {
			outgoing = append(outgoing, link)
		} else {
			incoming = append(incoming, link)
		}
	}

	if len(outgoing) > 0 {
		sb.WriteString("Outgoing:\n")
		for _, link := range outgoing {
			sb.WriteString(fmt.Sprintf("  -[%s]-> %s\n", link.Type, link.Target))
		}
		sb.WriteString("\n")
	}

	if len(incoming) > 0 {
		sb.WriteString("Incoming:\n")
		for _, link := range incoming {
			sb.WriteString(fmt.Sprintf("  %s -[%s]->\n", link.Source, link.Type))
		}
	}

	return sb.String()
}

// formatSubgraph formats a subgraph for display
func formatSubgraph(sg *client.Subgraph) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Subgraph: %d nodes, %d edges\n\n", len(sg.Nodes), len(sg.Edges)))

	sb.WriteString("Nodes:\n")
	for _, node := range sg.Nodes {
		title := getStringMeta(node.Meta, "title", "name")
		if title != "" {
			sb.WriteString(fmt.Sprintf("  [%s] %s - %s\n", node.Type, node.ID, title))
		} else {
			sb.WriteString(fmt.Sprintf("  [%s] %s\n", node.Type, node.ID))
		}
	}

	if len(sg.Edges) > 0 {
		sb.WriteString("\nRelationships:\n")
		for _, edge := range sg.Edges {
			sb.WriteString(fmt.Sprintf("  %s -[%s]-> %s\n", edge.Source, edge.Type, edge.Target))
		}
	}

	return sb.String()
}

// getStringMeta gets a string value from metadata, trying multiple keys
func getStringMeta(meta map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := meta[key]; ok {
			if str, ok := val.(string); ok {
				return str
			}
		}
	}
	return ""
}
