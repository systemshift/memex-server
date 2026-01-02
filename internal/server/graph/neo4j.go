package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/systemshift/memex/internal/memex/core"
	"github.com/systemshift/memex/internal/server/subscriptions"
)

// Repository wraps Neo4j operations
type Repository struct {
	driver       neo4j.DriverWithContext
	eventEmitter func(subscriptions.Event)
}

// SetEventEmitter sets the callback for emitting events to the subscription manager
func (r *Repository) SetEventEmitter(emitter func(subscriptions.Event)) {
	r.eventEmitter = emitter
}

// emit sends an event to the subscription manager if one is registered
func (r *Repository) emit(event subscriptions.Event) {
	if r.eventEmitter != nil {
		r.eventEmitter(event)
	}
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
		// Index on degree for fast top-connected queries
		"CREATE INDEX node_degree_index IF NOT EXISTS FOR (n:Node) ON (n.degree)",
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
				deleted: false,
				degree: 0
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

	// Emit event on successful creation
	if err == nil {
		r.emit(subscriptions.Event{
			ID:        uuid.New().String(),
			Type:      subscriptions.EventNodeCreated,
			Timestamp: time.Now(),
			NodeID:    node.ID,
			NodeType:  node.Type,
			Meta:      node.Meta,
		})
	}

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

// UpdateNodeMeta updates a node's metadata (merges with existing)
func (r *Repository) UpdateNodeMeta(ctx context.Context, id string, meta map[string]any) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// First get existing properties
		getQuery := `
			MATCH (n:Node {id: $id})
			WHERE (n.deleted IS NULL OR n.deleted = false)
			RETURN n.properties as properties
		`
		result, err := tx.Run(ctx, getQuery, map[string]any{"id": id})
		if err != nil {
			return nil, err
		}

		if !result.Next(ctx) {
			return nil, fmt.Errorf("node not found: %s", id)
		}

		// Parse existing properties
		existingMeta := make(map[string]any)
		record := result.Record()
		if propsVal, ok := record.Get("properties"); ok {
			if propsStr, ok := propsVal.(string); ok && propsStr != "" {
				if err := json.Unmarshal([]byte(propsStr), &existingMeta); err != nil {
					return nil, fmt.Errorf("unmarshaling existing properties: %w", err)
				}
			}
		}

		// Merge new meta into existing
		for k, v := range meta {
			existingMeta[k] = v
		}

		// Convert back to JSON
		metaJSON, err := json.Marshal(existingMeta)
		if err != nil {
			return nil, fmt.Errorf("marshaling meta: %w", err)
		}

		// Update the node
		updateQuery := `
			MATCH (n:Node {id: $id})
			SET n.properties = $properties, n.modified = datetime()
			RETURN n
		`
		_, err = tx.Run(ctx, updateQuery, map[string]any{
			"id":         id,
			"properties": string(metaJSON),
		})
		return existingMeta, err
	})

	// Emit event on successful update
	if err == nil {
		// Get node type from existing meta or default
		nodeType := ""
		if meta, ok := result.(map[string]any); ok {
			if t, ok := meta["type"].(string); ok {
				nodeType = t
			}
		}
		r.emit(subscriptions.Event{
			ID:        uuid.New().String(),
			Type:      subscriptions.EventNodeUpdated,
			Timestamp: time.Now(),
			NodeID:    id,
			NodeType:  nodeType,
			Meta:      meta,
		})
	}

	return err
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
			SET source.degree = COALESCE(source.degree, 0) + 1,
			    target.degree = COALESCE(target.degree, 0) + 1
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

	// Emit event on successful creation
	if err == nil {
		r.emit(subscriptions.Event{
			ID:         uuid.New().String(),
			Type:       subscriptions.EventLinkCreated,
			Timestamp:  time.Now(),
			LinkSource: link.Source,
			LinkTarget: link.Target,
			LinkType:   link.Type,
			Meta:       link.Meta,
		})
	}

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

	// Emit event on successful deletion
	if err == nil {
		r.emit(subscriptions.Event{
			ID:        uuid.New().String(),
			Type:      subscriptions.EventNodeDeleted,
			Timestamp: time.Now(),
			NodeID:    nodeID,
		})
	}

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
			SET source.degree = CASE WHEN source.degree > 0 THEN source.degree - 1 ELSE 0 END,
			    target.degree = CASE WHEN target.degree > 0 THEN target.degree - 1 ELSE 0 END
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

	// Emit event on successful deletion
	if err == nil {
		r.emit(subscriptions.Event{
			ID:         uuid.New().String(),
			Type:       subscriptions.EventLinkDeleted,
			Timestamp:  time.Now(),
			LinkSource: sourceID,
			LinkTarget: targetID,
			LinkType:   linkType,
		})
	}

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

// UpdateAttentionEdge creates or updates an attention-weighted edge between nodes
// This allows the DAG to learn which nodes are frequently co-attended across queries
func (r *Repository) UpdateAttentionEdge(ctx context.Context, source, target, queryID string, weight float64) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Check if ATTENDED edge already exists
		checkQuery := `
			MATCH (s:Node {id: $source})-[r:LINK {type: 'ATTENDED'}]->(t:Node {id: $target})
			RETURN r
		`
		result, err := tx.Run(ctx, checkQuery, map[string]any{
			"source": source,
			"target": target,
		})
		if err != nil {
			return nil, err
		}

		edgeExists := result.Next(ctx)

		if edgeExists {
			// Get current properties to compute running average
			record := result.Record()
			relValue, _ := record.Get("r")
			relData := relValue.(neo4j.Relationship)

			var currentMeta map[string]interface{}
			if propsStr, ok := relData.Props["properties"].(string); ok {
				json.Unmarshal([]byte(propsStr), &currentMeta)
			}

			// Compute running average
			currentWeight := 0.0
			currentCount := 0.0
			if w, ok := currentMeta["weight"].(float64); ok {
				currentWeight = w
			}
			if c, ok := currentMeta["query_count"].(float64); ok {
				currentCount = c
			}

			newWeight := (currentWeight*currentCount + weight) / (currentCount + 1)
			newCount := currentCount + 1

			// Update with new values
			updatedMeta := map[string]interface{}{
				"weight":        newWeight,
				"query_count":   newCount,
				"last_updated":  time.Now().Format(time.RFC3339),
				"last_query_id": queryID,
			}
			updatedJSON, _ := json.Marshal(updatedMeta)

			updateQuery := `
				MATCH (s:Node {id: $source})-[r:LINK {type: 'ATTENDED'}]->(t:Node {id: $target})
				SET r.properties = $properties,
				    r.modified = datetime($modified)
				RETURN r
			`
			_, err = tx.Run(ctx, updateQuery, map[string]any{
				"source":     source,
				"target":     target,
				"properties": string(updatedJSON),
				"modified":   time.Now().Format(time.RFC3339),
			})
			return nil, err
		} else {
			// Create new ATTENDED edge
			meta := map[string]interface{}{
				"weight":       weight,
				"query_count":  1,
				"last_updated": time.Now().Format(time.RFC3339),
				"last_query_id": queryID,
			}
			metaJSON, err := json.Marshal(meta)
			if err != nil {
				return nil, err
			}

			createQuery := `
				MATCH (s:Node {id: $source}), (t:Node {id: $target})
				CREATE (s)-[r:LINK {
					type: 'ATTENDED',
					properties: $properties,
					created: datetime($created),
					modified: datetime($modified)
				}]->(t)
				RETURN r
			`
			now := time.Now().Format(time.RFC3339)
			_, err = tx.Run(ctx, createQuery, map[string]any{
				"source":     source,
				"target":     target,
				"properties": string(metaJSON),
				"created":    now,
				"modified":   now,
			})
			return nil, err
		}
	})

	return err
}

// GetAttentionSubgraph extracts nodes connected by high-weight attention edges
// This enables sparse, learned attention patterns to guide retrieval
func (r *Repository) GetAttentionSubgraph(ctx context.Context, startNodeID string, minWeight float64, maxNodes int) (*Subgraph, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Get nodes connected by ATTENDED edges (simpler query)
		query := `
			MATCH (start:Node {id: $start_id})-[r:LINK]-(n:Node)
			WHERE (start.deleted IS NULL OR start.deleted = false)
			  AND (n.deleted IS NULL OR n.deleted = false)
			  AND r.type = 'ATTENDED'
			RETURN DISTINCT n, r
			LIMIT $max_nodes
		`

		nodeResult, err := tx.Run(ctx, query, map[string]any{
			"start_id":  startNodeID,
			"max_nodes": maxNodes,
		})
		if err != nil {
			return nil, err
		}

		// Collect nodes, filtering by weight in Go
		nodeMap := make(map[string]*core.Node)
		var nodeIDs []string
		for nodeResult.Next(ctx) {
			record := nodeResult.Record()
			nodeValue, _ := record.Get("n")
			relValue, _ := record.Get("r")

			// Parse relationship to check weight
			relData := relValue.(neo4j.Relationship)
			var meta map[string]interface{}
			if propsStr, ok := relData.Props["properties"].(string); ok {
				json.Unmarshal([]byte(propsStr), &meta)
			}

			// Filter by weight
			if weight, ok := meta["weight"].(float64); ok && weight >= minWeight {
				nodeData := nodeValue.(neo4j.Node)
				node, err := parseNodeFromNeo4j(nodeData)
				if err != nil {
					continue
				}
				nodeMap[node.ID] = node
				nodeIDs = append(nodeIDs, node.ID)
			}
		}

		// Get all ATTENDED edges between these nodes
		edgeQuery := `
			MATCH (source:Node)-[r:LINK]->(target:Node)
			WHERE source.id IN $node_ids
			  AND target.id IN $node_ids
			  AND r.type = 'ATTENDED'
			RETURN source.id as source_id, target.id as target_id, r
		`

		edgeResult, err := tx.Run(ctx, edgeQuery, map[string]any{"node_ids": nodeIDs})
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

			var meta map[string]any
			if propsStr, ok := relData.Props["properties"].(string); ok {
				if err := json.Unmarshal([]byte(propsStr), &meta); err != nil {
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
				Depth:     2, // Fixed for attention queries
			},
		}

		return subgraph, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*Subgraph), nil
}

// GraphMap represents the high-level structure of the graph for agent exploration
type GraphMap struct {
	Stats           GraphStats            `json:"stats"`
	NodeTypes       map[string]int        `json:"node_types"`
	EdgeTypes       map[string]int        `json:"edge_types"`
	TopConnected    []NodeSummary         `json:"top_connected"`
	SamplesByType   map[string][]string   `json:"samples_by_type"`
}

// GraphStats holds basic graph statistics
type GraphStats struct {
	TotalNodes int `json:"total_nodes"`
	TotalEdges int `json:"total_edges"`
}

// NodeSummary is a lightweight node representation for the map
type NodeSummary struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Degree int    `json:"degree"`
}

// GetGraphMap returns a high-level map of the graph for agent exploration
func (r *Repository) GetGraphMap(ctx context.Context, sampleSize int) (*GraphMap, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		graphMap := &GraphMap{
			NodeTypes:     make(map[string]int),
			EdgeTypes:     make(map[string]int),
			SamplesByType: make(map[string][]string),
		}

		// Get total node count and count by type
		nodeCountQuery := `
			MATCH (n:Node)
			WHERE n.deleted IS NULL OR n.deleted = false
			RETURN n.type as type, count(*) as count
		`
		nodeResult, err := tx.Run(ctx, nodeCountQuery, nil)
		if err != nil {
			return nil, err
		}

		totalNodes := 0
		for nodeResult.Next(ctx) {
			record := nodeResult.Record()
			nodeType, _ := record.Get("type")
			count, _ := record.Get("count")
			countInt := int(count.(int64))
			graphMap.NodeTypes[nodeType.(string)] = countInt
			totalNodes += countInt
		}
		graphMap.Stats.TotalNodes = totalNodes

		// Get total edge count and count by type
		edgeCountQuery := `
			MATCH ()-[r:LINK]->()
			RETURN r.type as type, count(*) as count
		`
		edgeResult, err := tx.Run(ctx, edgeCountQuery, nil)
		if err != nil {
			return nil, err
		}

		totalEdges := 0
		for edgeResult.Next(ctx) {
			record := edgeResult.Record()
			edgeType, _ := record.Get("type")
			count, _ := record.Get("count")
			countInt := int(count.(int64))
			graphMap.EdgeTypes[edgeType.(string)] = countInt
			totalEdges += countInt
		}
		graphMap.Stats.TotalEdges = totalEdges

		// Get top connected nodes (using precomputed degree)
		topConnectedQuery := `
			MATCH (n:Node)
			WHERE (n.deleted IS NULL OR n.deleted = false)
			  AND n.degree IS NOT NULL
			RETURN n.id as id, n.type as type, n.degree as degree
			ORDER BY n.degree DESC
			LIMIT $limit
		`
		topResult, err := tx.Run(ctx, topConnectedQuery, map[string]any{"limit": sampleSize})
		if err != nil {
			return nil, err
		}

		for topResult.Next(ctx) {
			record := topResult.Record()
			id, _ := record.Get("id")
			nodeType, _ := record.Get("type")
			degree, _ := record.Get("degree")

			degreeInt := 0
			if degree != nil {
				if d, ok := degree.(int64); ok {
					degreeInt = int(d)
				}
			}

			graphMap.TopConnected = append(graphMap.TopConnected, NodeSummary{
				ID:     id.(string),
				Type:   nodeType.(string),
				Degree: degreeInt,
			})
		}

		// Get sample nodes per type
		for nodeType := range graphMap.NodeTypes {
			sampleQuery := `
				MATCH (n:Node {type: $type})
				WHERE n.deleted IS NULL OR n.deleted = false
				RETURN n.id as id
				LIMIT $limit
			`
			sampleResult, err := tx.Run(ctx, sampleQuery, map[string]any{
				"type":  nodeType,
				"limit": sampleSize,
			})
			if err != nil {
				continue
			}

			var samples []string
			for sampleResult.Next(ctx) {
				record := sampleResult.Record()
				id, _ := record.Get("id")
				samples = append(samples, id.(string))
			}
			graphMap.SamplesByType[nodeType] = samples
		}

		return graphMap, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*GraphMap), nil
}

// PruneWeakAttentionEdges removes attention edges with low weight or query count
// This maintains DAG quality by removing noise
func (r *Repository) PruneWeakAttentionEdges(ctx context.Context, minWeight float64, minQueryCount int) (int, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Get all ATTENDED edges
		query := `
			MATCH ()-[r:LINK]->()
			WHERE r.type = 'ATTENDED'
			RETURN id(r) as rel_id, r.properties as props
		`

		result, err := tx.Run(ctx, query, nil)
		if err != nil {
			return 0, err
		}

		// Collect edges to delete (filter in Go)
		var edgesToDelete []int64
		for result.Next(ctx) {
			record := result.Record()
			relID, _ := record.Get("rel_id")
			propsValue, _ := record.Get("props")

			if propsStr, ok := propsValue.(string); ok {
				var meta map[string]interface{}
				json.Unmarshal([]byte(propsStr), &meta)

				shouldDelete := false
				if weight, ok := meta["weight"].(float64); ok && weight < minWeight {
					shouldDelete = true
				}
				if count, ok := meta["query_count"].(float64); ok && int(count) < minQueryCount {
					shouldDelete = true
				}

				if shouldDelete {
					edgesToDelete = append(edgesToDelete, relID.(int64))
				}
			}
		}

		// Delete collected edges
		if len(edgesToDelete) > 0 {
			deleteQuery := `
				MATCH ()-[r]->()
				WHERE id(r) IN $ids
				DELETE r
			`
			_, err = tx.Run(ctx, deleteQuery, map[string]any{"ids": edgesToDelete})
			if err != nil {
				return 0, err
			}
		}

		return len(edgesToDelete), nil
	})

	if err != nil {
		return 0, err
	}

	return result.(int), nil
}

// GetEntitiesInterpretedThrough returns all entities linked to a lens via INTERPRETED_THROUGH edges
func (r *Repository) GetEntitiesInterpretedThrough(ctx context.Context, lensID string) ([]*core.Node, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Query entities that have INTERPRETED_THROUGH links to this lens
		query := `
			MATCH (e:Node)-[r:LINK {type: 'INTERPRETED_THROUGH'}]->(l:Node {id: $lens_id})
			WHERE (e.deleted IS NULL OR e.deleted = false)
			RETURN e, r.properties as link_props
		`

		result, err := tx.Run(ctx, query, map[string]any{"lens_id": lensID})
		if err != nil {
			return nil, err
		}

		var nodes []*core.Node
		for result.Next(ctx) {
			record := result.Record()
			nodeValue, _ := record.Get("e")
			nodeData := nodeValue.(neo4j.Node)

			node, err := parseNodeFromNeo4j(nodeData)
			if err != nil {
				continue
			}

			// Add link metadata to node meta
			if linkPropsValue, ok := record.Get("link_props"); ok {
				if linkPropsStr, ok := linkPropsValue.(string); ok {
					var linkMeta map[string]interface{}
					if err := json.Unmarshal([]byte(linkPropsStr), &linkMeta); err == nil {
						if node.Meta == nil {
							node.Meta = make(map[string]interface{})
						}
						node.Meta["_interpretation"] = linkMeta
					}
				}
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

// CreateInterpretedThroughLink creates an INTERPRETED_THROUGH link between an entity and a lens
func (r *Repository) CreateInterpretedThroughLink(ctx context.Context, entityID, lensID string, meta map[string]interface{}) error {
	link := &core.Link{
		Source:   entityID,
		Target:   lensID,
		Type:     "INTERPRETED_THROUGH",
		Meta:     meta,
		Created:  time.Now(),
		Modified: time.Now(),
	}
	return r.CreateLink(ctx, link)
}

// QueryByLens returns entities interpreted through a lens, optionally filtered by pattern
func (r *Repository) QueryByLens(ctx context.Context, lensID string, pattern string, limit int, offset int) ([]*core.Node, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (e:Node)-[r:LINK {type: 'INTERPRETED_THROUGH'}]->(l:Node {id: $lens_id})
			WHERE (e.deleted IS NULL OR e.deleted = false)
		`
		params := map[string]any{"lens_id": lensID}

		// Filter by pattern if specified (searches in link properties)
		if pattern != "" {
			query += ` AND r.properties CONTAINS $pattern`
			params["pattern"] = pattern
		}

		query += ` RETURN e, r.properties as link_props`

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
			nodeValue, _ := record.Get("e")
			nodeData := nodeValue.(neo4j.Node)

			node, err := parseNodeFromNeo4j(nodeData)
			if err != nil {
				continue
			}

			// Add interpretation metadata
			if linkPropsValue, ok := record.Get("link_props"); ok {
				if linkPropsStr, ok := linkPropsValue.(string); ok {
					var linkMeta map[string]interface{}
					if err := json.Unmarshal([]byte(linkPropsStr), &linkMeta); err == nil {
						if node.Meta == nil {
							node.Meta = make(map[string]interface{})
						}
						node.Meta["_interpretation"] = linkMeta
					}
				}
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

// LensExport represents a complete lens export with entities and links
type LensExport struct {
	Lens     *core.Node      `json:"lens"`
	Entities []*core.Node    `json:"entities"`
	Links    []*SubgraphEdge `json:"links"`
	Stats    struct {
		EntityCount int `json:"entity_count"`
		LinkCount   int `json:"link_count"`
	} `json:"stats"`
}

// ExportLens returns a complete export of a lens and its interpreted entities
func (r *Repository) ExportLens(ctx context.Context, lensID string, includeExtractedFrom bool) (*LensExport, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		export := &LensExport{}

		// Get the lens node
		lensQuery := `
			MATCH (l:Node {id: $lens_id})
			WHERE (l.deleted IS NULL OR l.deleted = false)
			RETURN l
		`
		lensResult, err := tx.Run(ctx, lensQuery, map[string]any{"lens_id": lensID})
		if err != nil {
			return nil, err
		}

		if !lensResult.Next(ctx) {
			return nil, fmt.Errorf("lens not found: %s", lensID)
		}

		lensRecord := lensResult.Record()
		lensValue, _ := lensRecord.Get("l")
		lensData := lensValue.(neo4j.Node)
		lensNode, err := parseNodeFromNeo4j(lensData)
		if err != nil {
			return nil, err
		}
		export.Lens = lensNode

		// Get all entities interpreted through this lens
		entityQuery := `
			MATCH (e:Node)-[r:LINK {type: 'INTERPRETED_THROUGH'}]->(l:Node {id: $lens_id})
			WHERE (e.deleted IS NULL OR e.deleted = false)
			RETURN e, r
		`
		entityResult, err := tx.Run(ctx, entityQuery, map[string]any{"lens_id": lensID})
		if err != nil {
			return nil, err
		}

		entityIDs := []string{}
		for entityResult.Next(ctx) {
			record := entityResult.Record()
			nodeValue, _ := record.Get("e")
			relValue, _ := record.Get("r")

			nodeData := nodeValue.(neo4j.Node)
			node, err := parseNodeFromNeo4j(nodeData)
			if err != nil {
				continue
			}
			export.Entities = append(export.Entities, node)
			entityIDs = append(entityIDs, node.ID)

			// Add INTERPRETED_THROUGH link
			relData := relValue.(neo4j.Relationship)
			var meta map[string]any
			if propsStr, ok := relData.Props["properties"].(string); ok {
				json.Unmarshal([]byte(propsStr), &meta)
			}

			export.Links = append(export.Links, &SubgraphEdge{
				Source: node.ID,
				Target: lensID,
				Type:   "INTERPRETED_THROUGH",
				Meta:   meta,
			})
		}

		// Optionally get EXTRACTED_FROM links
		if includeExtractedFrom && len(entityIDs) > 0 {
			extractQuery := `
				MATCH (e:Node)-[r:LINK {type: 'EXTRACTED_FROM'}]->(s:Node)
				WHERE e.id IN $entity_ids
				RETURN e.id as entity_id, s.id as source_id, r
			`
			extractResult, err := tx.Run(ctx, extractQuery, map[string]any{"entity_ids": entityIDs})
			if err != nil {
				return nil, err
			}

			for extractResult.Next(ctx) {
				record := extractResult.Record()
				entityID, _ := record.Get("entity_id")
				sourceID, _ := record.Get("source_id")
				relValue, _ := record.Get("r")

				relData := relValue.(neo4j.Relationship)
				var meta map[string]any
				if propsStr, ok := relData.Props["properties"].(string); ok {
					json.Unmarshal([]byte(propsStr), &meta)
				}

				export.Links = append(export.Links, &SubgraphEdge{
					Source: entityID.(string),
					Target: sourceID.(string),
					Type:   "EXTRACTED_FROM",
					Meta:   meta,
				})
			}
		}

		export.Stats.EntityCount = len(export.Entities)
		export.Stats.LinkCount = len(export.Links)

		return export, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*LensExport), nil
}

// ============== Subscription Repository Methods ==============

// CreateSubscriptionNode persists a subscription as a node in the graph
func (r *Repository) CreateSubscriptionNode(ctx context.Context, sub *subscriptions.Subscription) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		patternJSON, err := json.Marshal(sub.Pattern)
		if err != nil {
			return nil, fmt.Errorf("marshaling pattern: %w", err)
		}

		meta := map[string]interface{}{
			"name":        sub.Name,
			"description": sub.Description,
			"pattern":     string(patternJSON),
			"webhook":     sub.Webhook,
			"websocket":   sub.WebSocket,
			"enabled":     sub.Enabled,
			"fire_count":  sub.FireCount,
		}
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return nil, fmt.Errorf("marshaling meta: %w", err)
		}

		query := `
			CREATE (n:Node {
				id: $id,
				type: 'Subscription',
				properties: $properties,
				created: datetime($created),
				modified: datetime($modified),
				deleted: false,
				degree: 0
			})
			RETURN n
		`

		params := map[string]any{
			"id":         "subscription:" + sub.ID,
			"properties": string(metaJSON),
			"created":    sub.Created.Format("2006-01-02T15:04:05Z"),
			"modified":   sub.Modified.Format("2006-01-02T15:04:05Z"),
		}

		_, err = tx.Run(ctx, query, params)
		return nil, err
	})

	return err
}

// UpdateSubscriptionNode updates a subscription node in the graph
func (r *Repository) UpdateSubscriptionNode(ctx context.Context, sub *subscriptions.Subscription) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		patternJSON, err := json.Marshal(sub.Pattern)
		if err != nil {
			return nil, fmt.Errorf("marshaling pattern: %w", err)
		}

		meta := map[string]interface{}{
			"name":        sub.Name,
			"description": sub.Description,
			"pattern":     string(patternJSON),
			"webhook":     sub.Webhook,
			"websocket":   sub.WebSocket,
			"enabled":     sub.Enabled,
			"fire_count":  sub.FireCount,
		}
		if sub.LastFired != nil {
			meta["last_fired"] = sub.LastFired.Format(time.RFC3339)
		}
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return nil, fmt.Errorf("marshaling meta: %w", err)
		}

		query := `
			MATCH (n:Node {id: $id})
			SET n.properties = $properties, n.modified = datetime()
			RETURN n
		`

		params := map[string]any{
			"id":         "subscription:" + sub.ID,
			"properties": string(metaJSON),
		}

		_, err = tx.Run(ctx, query, params)
		return nil, err
	})

	return err
}

// DeleteSubscriptionNode removes a subscription node from the graph
func (r *Repository) DeleteSubscriptionNode(ctx context.Context, id string) error {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (n:Node {id: $id})
			DELETE n
		`
		_, err := tx.Run(ctx, query, map[string]any{"id": "subscription:" + id})
		return nil, err
	})

	return err
}

// LoadSubscriptions loads all subscription nodes from the graph
func (r *Repository) LoadSubscriptions(ctx context.Context) ([]*subscriptions.Subscription, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (n:Node {type: 'Subscription'})
			WHERE (n.deleted IS NULL OR n.deleted = false)
			RETURN n.id as id, n.properties as properties, n.created as created, n.modified as modified
		`

		result, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}

		var subs []*subscriptions.Subscription
		for result.Next(ctx) {
			record := result.Record()

			idVal, _ := record.Get("id")
			id := idVal.(string)
			// Remove "subscription:" prefix
			if len(id) > 13 && id[:13] == "subscription:" {
				id = id[13:]
			}

			propsVal, _ := record.Get("properties")
			propsStr := propsVal.(string)

			var meta map[string]interface{}
			if err := json.Unmarshal([]byte(propsStr), &meta); err != nil {
				continue
			}

			createdVal, _ := record.Get("created")
			modifiedVal, _ := record.Get("modified")

			var created, modified time.Time
			if t, ok := createdVal.(time.Time); ok {
				created = t
			}
			if t, ok := modifiedVal.(time.Time); ok {
				modified = t
			}

			sub := &subscriptions.Subscription{
				ID:       id,
				Created:  created,
				Modified: modified,
			}

			if v, ok := meta["name"].(string); ok {
				sub.Name = v
			}
			if v, ok := meta["description"].(string); ok {
				sub.Description = v
			}
			if v, ok := meta["webhook"].(string); ok {
				sub.Webhook = v
			}
			if v, ok := meta["websocket"].(bool); ok {
				sub.WebSocket = v
			}
			if v, ok := meta["enabled"].(bool); ok {
				sub.Enabled = v
			}
			if v, ok := meta["fire_count"].(float64); ok {
				sub.FireCount = int(v)
			}
			if v, ok := meta["last_fired"].(string); ok {
				if t, err := time.Parse(time.RFC3339, v); err == nil {
					sub.LastFired = &t
				}
			}
			if v, ok := meta["pattern"].(string); ok {
				var pattern subscriptions.SubscriptionPattern
				if err := json.Unmarshal([]byte(v), &pattern); err == nil {
					sub.Pattern = pattern
				}
			}

			subs = append(subs, sub)
		}

		return subs, nil
	})

	if err != nil {
		return nil, err
	}

	return result.([]*subscriptions.Subscription), nil
}

// ExecuteCypherRead executes a read-only Cypher query and returns results
func (r *Repository) ExecuteCypherRead(ctx context.Context, cypher string, params map[string]interface{}) ([]map[string]interface{}, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}

		var results []map[string]interface{}
		for result.Next(ctx) {
			record := result.Record()
			row := make(map[string]interface{})
			for _, key := range record.Keys {
				val, _ := record.Get(key)
				row[key] = val
			}
			results = append(results, row)
		}

		return results, nil
	})

	if err != nil {
		return nil, err
	}

	return result.([]map[string]interface{}), nil
}
