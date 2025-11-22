package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/systemshift/memex/internal/memex/core"
)

// Repository wraps Neo4j operations
type Repository struct {
	driver neo4j.DriverWithContext
}

// Config holds Neo4j connection configuration
type Config struct {
	URI      string
	Username string
	Password string
	Database string
}

// New creates a new Neo4j repository
func New(ctx context.Context, cfg Config) (*Repository, error) {
	driver, err := neo4j.NewDriverWithContext(
		cfg.URI,
		neo4j.BasicAuth(cfg.Username, cfg.Password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("creating neo4j driver: %w", err)
	}

	// Verify connectivity
	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("connecting to neo4j: %w", err)
	}

	return &Repository{driver: driver}, nil
}

// Close closes the Neo4j connection
func (r *Repository) Close(ctx context.Context) error {
	return r.driver.Close(ctx)
}

// EnsureIndexes creates necessary indexes for performance
func (r *Repository) EnsureIndexes(ctx context.Context) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	indexes := []string{
		// Index on node ID for fast lookups
		"CREATE INDEX node_id_index IF NOT EXISTS FOR (n:Node) ON (n.id)",
		// Index on node type for filtering
		"CREATE INDEX node_type_index IF NOT EXISTS FOR (n:Node) ON (n.type)",
		// Text index on properties for search (Neo4j 5+)
		"CREATE TEXT INDEX node_properties_text_index IF NOT EXISTS FOR (n:Node) ON (n.properties)",
	}

	for _, indexQuery := range indexes {
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx, indexQuery, nil)
			return nil, err
		})
		if err != nil {
			return fmt.Errorf("creating index: %w", err)
		}
	}

	return nil
}

// parseNodeFromNeo4j is a helper to parse Neo4j node data into core.Node
func parseNodeFromNeo4j(nodeData neo4j.Node) (*core.Node, error) {
	// Unmarshal properties JSON string back to map
	var meta map[string]any
	if propsStr, ok := nodeData.Props["properties"].(string); ok {
		if err := json.Unmarshal([]byte(propsStr), &meta); err != nil {
			return nil, fmt.Errorf("unmarshaling properties: %w", err)
		}
	}

	// Get content
	var content []byte
	if contentStr, ok := nodeData.Props["content"].(string); ok {
		content = []byte(contentStr)
	}

	// Parse timestamps
	var created, modified, deletedAt time.Time
	if createdVal, ok := nodeData.Props["created"]; ok {
		if neo4jTime, ok := createdVal.(time.Time); ok {
			created = neo4jTime
		}
	}
	if modifiedVal, ok := nodeData.Props["modified"]; ok {
		if neo4jTime, ok := modifiedVal.(time.Time); ok {
			modified = neo4jTime
		}
	}
	if deletedAtVal, ok := nodeData.Props["deleted_at"]; ok {
		if neo4jTime, ok := deletedAtVal.(time.Time); ok {
			deletedAt = neo4jTime
		}
	}

	// Parse deleted flag
	deleted := false
	if deletedVal, ok := nodeData.Props["deleted"]; ok {
		if d, ok := deletedVal.(bool); ok {
			deleted = d
		}
	}

	return &core.Node{
		ID:        nodeData.Props["id"].(string),
		Type:      nodeData.Props["type"].(string),
		Content:   content,
		Meta:      meta,
		Created:   created,
		Modified:  modified,
		Deleted:   deleted,
		DeletedAt: deletedAt,
	}, nil
}

// CreateNode creates a new node in the graph
func (r *Repository) CreateNode(ctx context.Context, node *core.Node) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Convert meta to JSON string (Neo4j doesn't support nested maps)
		metaJSON, err := json.Marshal(node.Meta)
		if err != nil {
			return nil, fmt.Errorf("marshaling meta: %w", err)
		}

		query := `
			CREATE (n:Node {
				id: $id,
				type: $type,
				content: $content,
				properties: $properties,
				created: datetime($created),
				modified: datetime($modified),
				deleted: false
			})
			RETURN n
		`

		params := map[string]any{
			"id":         node.ID,
			"type":       node.Type,
			"content":    string(node.Content),
			"properties": string(metaJSON),
			"created":    node.Created.Format("2006-01-02T15:04:05Z"),
			"modified":   node.Modified.Format("2006-01-02T15:04:05Z"),
		}

		_, err = tx.Run(ctx, query, params)
		return nil, err
	})

	return err
}

// GetNode retrieves a node by ID
func (r *Repository) GetNode(ctx context.Context, id string) (*core.Node, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (n:Node {id: $id})
			WHERE (n.deleted IS NULL OR n.deleted = false)
			RETURN n
		`

		result, err := tx.Run(ctx, query, map[string]any{"id": id})
		if err != nil {
			return nil, err
		}

		if !result.Next(ctx) {
			return nil, fmt.Errorf("node not found: %s", id)
		}

		record := result.Record()
		nodeValue, _ := record.Get("n")
		nodeData := nodeValue.(neo4j.Node)

		return parseNodeFromNeo4j(nodeData)
	})

	if err != nil {
		return nil, err
	}

	return result.(*core.Node), nil
}

// CreateLink creates a relationship between two nodes
func (r *Repository) CreateLink(ctx context.Context, link *core.Link) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Convert meta to JSON string
		metaJSON, err := json.Marshal(link.Meta)
		if err != nil {
			return nil, fmt.Errorf("marshaling meta: %w", err)
		}

		query := `
			MATCH (source:Node {id: $source_id})
			MATCH (target:Node {id: $target_id})
			CREATE (source)-[r:LINK {
				type: $type,
				properties: $properties,
				created: datetime($created),
				modified: datetime($modified)
			}]->(target)
			RETURN r
		`

		params := map[string]any{
			"source_id":  link.Source,
			"target_id":  link.Target,
			"type":       link.Type,
			"properties": string(metaJSON),
			"created":    link.Created.Format("2006-01-02T15:04:05Z"),
			"modified":   link.Modified.Format("2006-01-02T15:04:05Z"),
		}

		_, err = tx.Run(ctx, query, params)
		return nil, err
	})

	return err
}

// GetLinks retrieves all links for a node
func (r *Repository) GetLinks(ctx context.Context, nodeID string) ([]*core.Link, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (source:Node {id: $node_id})-[r:LINK]->(target:Node)
			RETURN r, target.id as target_id
		`

		result, err := tx.Run(ctx, query, map[string]any{"node_id": nodeID})
		if err != nil {
			return nil, err
		}

		var links []*core.Link
		for result.Next(ctx) {
			record := result.Record()
			relValue, _ := record.Get("r")
			targetID, _ := record.Get("target_id")

			relData := relValue.(neo4j.Relationship)

			// Unmarshal properties JSON string back to map
			var meta map[string]any
			if propsStr, ok := relData.Props["properties"].(string); ok {
				if err := json.Unmarshal([]byte(propsStr), &meta); err != nil {
					return nil, fmt.Errorf("unmarshaling properties: %w", err)
				}
			}

			link := &core.Link{
				Source: nodeID,
				Target: targetID.(string),
				Type:   relData.Props["type"].(string),
				Meta:   meta,
			}
			links = append(links, link)
		}

		return links, nil
	})

	if err != nil {
		return nil, err
	}

	return result.([]*core.Link), nil
}

// ListNodes returns all node IDs
func (r *Repository) ListNodes(ctx context.Context) ([]string, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (n:Node)
			WHERE (n.deleted IS NULL OR n.deleted = false)
			RETURN n.id as id
		`

		result, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}

		var ids []string
		for result.Next(ctx) {
			record := result.Record()
			id, _ := record.Get("id")
			ids = append(ids, id.(string))
		}

		return ids, nil
	})

	if err != nil {
		return nil, err
	}

	return result.([]string), nil
}

// FilterNodes returns nodes matching filter criteria
func (r *Repository) FilterNodes(ctx context.Context, nodeTypes []string, propertyKey string, propertyValue string, limit int, offset int) ([]*core.Node, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `MATCH (n:Node) WHERE (n.deleted IS NULL OR n.deleted = false)`
		params := make(map[string]any)

		// Add type filter
		if len(nodeTypes) > 0 {
			query += ` AND n.type IN $types`
			params["types"] = nodeTypes
		}

		// Add property filter (searches in JSON properties)
		if propertyKey != "" && propertyValue != "" {
			query += ` AND n.properties CONTAINS $searchValue`
			// Search for the key-value pair in JSON
			params["searchValue"] = fmt.Sprintf(`"%s":"%s"`, propertyKey, propertyValue)
		}

		query += ` RETURN n`

		// Add pagination
		if limit > 0 {
			query += ` SKIP $offset LIMIT $limit`
			params["offset"] = offset
			params["limit"] = limit
		}

		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var nodes []*core.Node
		for result.Next(ctx) {
			record := result.Record()
			nodeValue, _ := record.Get("n")
			nodeData := nodeValue.(neo4j.Node)

			node, err := parseNodeFromNeo4j(nodeData)
			if err != nil {
				continue // Skip nodes that fail to parse
			}
			nodes = append(nodes, node)
		}

		return nodes, nil
	})

	if err != nil {
		return nil, err
	}

	return result.([]*core.Node), nil
}

// SearchNodes performs full-text search across node properties
func (r *Repository) SearchNodes(ctx context.Context, searchTerm string, limit int, offset int) ([]*core.Node, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (n:Node)
			WHERE (n.deleted IS NULL OR n.deleted = false)
			  AND (n.id CONTAINS $term
			   OR n.type CONTAINS $term
			   OR n.properties CONTAINS $term
			   OR n.content CONTAINS $term)
			RETURN n
		`

		// Add pagination
		params := map[string]any{"term": searchTerm}
		if limit > 0 {
			query += ` SKIP $offset LIMIT $limit`
			params["offset"] = offset
			params["limit"] = limit
		}

		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var nodes []*core.Node
		for result.Next(ctx) {
			record := result.Record()
			nodeValue, _ := record.Get("n")
			nodeData := nodeValue.(neo4j.Node)

			node, err := parseNodeFromNeo4j(nodeData)
			if err != nil {
				continue // Skip nodes that fail to parse
			}
			nodes = append(nodes, node)
		}

		return nodes, nil
	})

	if err != nil {
		return nil, err
	}

	return result.([]*core.Node), nil
}

// DeleteNode marks a node as deleted (tombstone) instead of removing it
func (r *Repository) DeleteNode(ctx context.Context, nodeID string, force bool) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Check if node exists and get its type
		checkQuery := `
			MATCH (n:Node {id: $id})
			WHERE n.deleted IS NULL OR n.deleted = false
			RETURN n.type as type
		`
		checkResult, err := tx.Run(ctx, checkQuery, map[string]any{"id": nodeID})
		if err != nil {
			return nil, err
		}

		if !checkResult.Next(ctx) {
			return nil, fmt.Errorf("node not found or already deleted: %s", nodeID)
		}

		record := checkResult.Record()
		nodeType, _ := record.Get("type")
		nodeTypeStr := nodeType.(string)

		// Protect Source layer (content-addressed nodes) unless force=true
		if !force && len(nodeID) > 7 && nodeID[:7] == "sha256:" {
			return nil, fmt.Errorf("cannot delete Source layer node (content-addressed): %s. Source nodes are immutable to maintain DAG integrity. Use force=true to override (not recommended)", nodeID)
		}

		// Tombstone the node (soft delete)
		updateQuery := `
			MATCH (n:Node {id: $id})
			SET n.deleted = true,
				n.deleted_at = datetime($deleted_at)
			RETURN n
		`

		result, err := tx.Run(ctx, updateQuery, map[string]any{
			"id":         nodeID,
			"deleted_at": time.Now().Format("2006-01-02T15:04:05Z"),
		})
		if err != nil {
			return nil, err
		}

		if !result.Next(ctx) {
			return nil, fmt.Errorf("failed to tombstone node: %s", nodeID)
		}

		return map[string]any{
			"tombstoned": true,
			"type":       nodeTypeStr,
		}, nil
	})

	return err
}

// DeleteLink deletes a specific relationship between two nodes
func (r *Repository) DeleteLink(ctx context.Context, sourceID string, targetID string, linkType string) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (source:Node {id: $source_id})-[r:LINK {type: $link_type}]->(target:Node {id: $target_id})
			DELETE r
		`

		result, err := tx.Run(ctx, query, map[string]any{
			"source_id": sourceID,
			"target_id": targetID,
			"link_type": linkType,
		})
		if err != nil {
			return nil, err
		}

		summary, err := result.Consume(ctx)
		if err != nil {
			return nil, err
		}

		if summary.Counters().RelationshipsDeleted() == 0 {
			return nil, fmt.Errorf("link not found: %s -[%s]-> %s", sourceID, linkType, targetID)
		}

		return nil, nil
	})

	return err
}

// TraverseGraph performs graph traversal from a starting node
func (r *Repository) TraverseGraph(ctx context.Context, startNodeID string, depth int, relationshipTypes []string, limit int, offset int) (map[string]*core.Node, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH path = (start:Node {id: $start_id})-[r:LINK*1..` + fmt.Sprintf("%d", depth) + `]->(n:Node)
			WHERE (start.deleted IS NULL OR start.deleted = false)
			  AND (n.deleted IS NULL OR n.deleted = false)
		`

		params := map[string]any{"start_id": startNodeID}

		// Add relationship type filter
		if len(relationshipTypes) > 0 {
			query += ` AND ALL(rel in r WHERE rel.type IN $rel_types)`
			params["rel_types"] = relationshipTypes
		}

		query += ` RETURN DISTINCT n`

		// Add pagination
		if limit > 0 {
			query += ` SKIP $offset LIMIT $limit`
			params["offset"] = offset
			params["limit"] = limit
		}

		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		nodes := make(map[string]*core.Node)
		for result.Next(ctx) {
			record := result.Record()
			nodeValue, _ := record.Get("n")
			nodeData := nodeValue.(neo4j.Node)

			node, err := parseNodeFromNeo4j(nodeData)
			if err != nil {
				continue // Skip nodes that fail to parse
			}
			nodes[node.ID] = node
		}

		return nodes, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(map[string]*core.Node), nil
}

// SubgraphEdge represents an edge in the subgraph
type SubgraphEdge struct {
	Source string                 `json:"source"`
	Target string                 `json:"target"`
	Type   string                 `json:"type"`
	Meta   map[string]interface{} `json:"meta,omitempty"`
}

// Subgraph represents nodes and edges within a graph region
type Subgraph struct {
	Nodes []*core.Node    `json:"nodes"`
	Edges []*SubgraphEdge `json:"edges"`
	Stats SubgraphStats   `json:"stats"`
}

// SubgraphStats provides metadata about the subgraph
type SubgraphStats struct {
	NodeCount int `json:"node_count"`
	EdgeCount int `json:"edge_count"`
	Depth     int `json:"depth"`
}

// GetSubgraph extracts a subgraph centered on a start node
// Returns all nodes within depth hops and ALL edges between those nodes
func (r *Repository) GetSubgraph(ctx context.Context, startNodeID string, depth int, relationshipTypes []string) (*Subgraph, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// First, get all nodes within depth hops
		nodeQuery := `
			MATCH path = (start:Node {id: $start_id})-[r:LINK*0..` + fmt.Sprintf("%d", depth) + `]-(n:Node)
			WHERE (start.deleted IS NULL OR start.deleted = false)
			  AND (n.deleted IS NULL OR n.deleted = false)
		`
		params := map[string]any{"start_id": startNodeID}

		// Add relationship type filter if specified
		if len(relationshipTypes) > 0 {
			nodeQuery += ` AND ALL(rel in r WHERE rel.type IN $rel_types)`
			params["rel_types"] = relationshipTypes
		}

		nodeQuery += ` RETURN DISTINCT n`

		nodeResult, err := tx.Run(ctx, nodeQuery, params)
		if err != nil {
			return nil, err
		}

		// Collect all node IDs and nodes
		nodeMap := make(map[string]*core.Node)
		var nodeIDs []string
		for nodeResult.Next(ctx) {
			record := nodeResult.Record()
			nodeValue, _ := record.Get("n")
			nodeData := nodeValue.(neo4j.Node)

			node, err := parseNodeFromNeo4j(nodeData)
			if err != nil {
				continue
			}
			nodeMap[node.ID] = node
			nodeIDs = append(nodeIDs, node.ID)
		}

		// Now get ALL edges between these nodes
		edgeQuery := `
			MATCH (source:Node)-[r:LINK]->(target:Node)
			WHERE source.id IN $node_ids AND target.id IN $node_ids
		`
		edgeParams := map[string]any{"node_ids": nodeIDs}

		// Add relationship type filter for edges if specified
		if len(relationshipTypes) > 0 {
			edgeQuery += ` AND r.type IN $rel_types`
			edgeParams["rel_types"] = relationshipTypes
		}

		edgeQuery += ` RETURN source.id as source_id, target.id as target_id, r`

		edgeResult, err := tx.Run(ctx, edgeQuery, edgeParams)
		if err != nil {
			return nil, err
		}

		var edges []*SubgraphEdge
		for edgeResult.Next(ctx) {
			record := edgeResult.Record()
			sourceID, _ := record.Get("source_id")
			targetID, _ := record.Get("target_id")
			relValue, _ := record.Get("r")

			relData := relValue.(neo4j.Relationship)

			// Unmarshal properties JSON string back to map
			var meta map[string]any
			if propsStr, ok := relData.Props["properties"].(string); ok {
				if err := json.Unmarshal([]byte(propsStr), &meta); err != nil {
					// Skip if can't parse
					meta = make(map[string]any)
				}
			}

			edge := &SubgraphEdge{
				Source: sourceID.(string),
				Target: targetID.(string),
				Type:   relData.Props["type"].(string),
				Meta:   meta,
			}
			edges = append(edges, edge)
		}

		// Convert node map to slice
		nodes := make([]*core.Node, 0, len(nodeMap))
		for _, node := range nodeMap {
			nodes = append(nodes, node)
		}

		subgraph := &Subgraph{
			Nodes: nodes,
			Edges: edges,
			Stats: SubgraphStats{
				NodeCount: len(nodes),
				EdgeCount: len(edges),
				Depth:     depth,
			},
		}

		return subgraph, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*Subgraph), nil
}
