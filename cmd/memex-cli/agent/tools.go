package agent

import "github.com/systemshift/memex/cmd/memex-cli/client"

// GetTools returns the tool definitions for Claude
func GetTools() []client.Tool {
	return []client.Tool{
		{
			Name:        "search",
			Description: "Full-text search across all nodes in the knowledge graph. Use this to find nodes by content, title, or any text. Returns matching nodes with their types and metadata.",
			InputSchema: client.InputSchema{
				Type: "object",
				Properties: map[string]client.Property{
					"query": {
						Type:        "string",
						Description: "Search query - can be keywords, names, topics, or phrases",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results to return (default: 10)",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "get_node",
			Description: "Get full details of a specific node by its ID. Use this when you have a node ID and need complete information including all metadata and content.",
			InputSchema: client.InputSchema{
				Type: "object",
				Properties: map[string]client.Property{
					"id": {
						Type:        "string",
						Description: "The node ID (e.g., 'person:alice', 'document:spec-v2')",
					},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "get_links",
			Description: "Get all relationships (links) connected to a node. Shows what other nodes this node is connected to and the type of relationship.",
			InputSchema: client.InputSchema{
				Type: "object",
				Properties: map[string]client.Property{
					"id": {
						Type:        "string",
						Description: "The node ID to get links for",
					},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "traverse",
			Description: "Traverse the graph starting from a node, following relationships to discover connected nodes. Use this to explore the neighborhood of a node and find related content.",
			InputSchema: client.InputSchema{
				Type: "object",
				Properties: map[string]client.Property{
					"start": {
						Type:        "string",
						Description: "Starting node ID for traversal",
					},
					"depth": {
						Type:        "integer",
						Description: "How many hops to traverse (default: 2, max: 5)",
					},
					"rel_types": {
						Type:        "string",
						Description: "Comma-separated relationship types to follow (optional, e.g., 'WORKS_ON,KNOWS')",
					},
				},
				Required: []string{"start"},
			},
		},
		{
			Name:        "filter",
			Description: "Filter nodes by type and/or property values. Use this to find all nodes of a specific type (e.g., all Person nodes) or with specific properties.",
			InputSchema: client.InputSchema{
				Type: "object",
				Properties: map[string]client.Property{
					"type": {
						Type:        "string",
						Description: "Node type to filter by (e.g., 'Person', 'Document', 'Project', 'Company')",
					},
					"key": {
						Type:        "string",
						Description: "Property key to filter by (optional)",
					},
					"value": {
						Type:        "string",
						Description: "Property value to match (required if key is provided)",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum results (default: 10)",
					},
				},
			},
		},
		{
			Name:        "list_nodes",
			Description: "List all nodes in the graph, optionally filtered by type. Use this to get an overview of what's in the knowledge graph.",
			InputSchema: client.InputSchema{
				Type: "object",
				Properties: map[string]client.Property{
					"type": {
						Type:        "string",
						Description: "Optional: filter by node type (e.g., 'Person', 'Document')",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum results (default: 20)",
					},
				},
			},
		},
	}
}

// SystemPrompt returns the system prompt for the agent
func SystemPrompt() string {
	return `You are a helpful assistant that answers questions about a company's knowledge graph stored in Memex.

Memex contains:
- People (employees, contacts, customers)
- Documents (specs, proposals, notes, emails)
- Projects (initiatives, deals, products)
- Companies (clients, partners, vendors)
- And relationships connecting all of these

When answering questions:
1. Use the available tools to search and explore the graph
2. Look at both nodes AND their relationships to understand context
3. Synthesize information from multiple sources when relevant
4. Be specific - cite node IDs and types when referencing information
5. If you can't find something, say so rather than making up information

Common patterns:
- "Who knows about X?" → Search for X, then look at connected Person nodes
- "What's happening with Y?" → Search for Y, traverse to see related activity
- "Find docs about Z" → Search for Z, filter by Document type
- "Tell me about [person]" → Get their node, traverse to see their work

Always be concise but complete in your answers.`
}
