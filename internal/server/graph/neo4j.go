package graph

import (
	"context"
	"encoding/json"
	"fmt"

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
				modified: datetime($modified)
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

		node := &core.Node{
			ID:      nodeData.Props["id"].(string),
			Type:    nodeData.Props["type"].(string),
			Content: content,
			Meta:    meta,
		}

		return node, nil
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
		query := `MATCH (n:Node) RETURN n.id as id`

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
func (r *Repository) FilterNodes(ctx context.Context, nodeTypes []string, propertyKey string, propertyValue string) ([]*core.Node, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `MATCH (n:Node)`
		params := make(map[string]any)

		// Add type filter
		if len(nodeTypes) > 0 {
			query += ` WHERE n.type IN $types`
			params["types"] = nodeTypes
		}

		// Add property filter (searches in JSON properties)
		if propertyKey != "" && propertyValue != "" {
			if len(nodeTypes) > 0 {
				query += ` AND`
			} else {
				query += ` WHERE`
			}
			query += ` n.properties CONTAINS $searchValue`
			// Search for the key-value pair in JSON
			params["searchValue"] = fmt.Sprintf(`"%s":"%s"`, propertyKey, propertyValue)
		}

		query += ` RETURN n`

		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var nodes []*core.Node
		for result.Next(ctx) {
			record := result.Record()
			nodeValue, _ := record.Get("n")
			nodeData := nodeValue.(neo4j.Node)

			var meta map[string]any
			if propsStr, ok := nodeData.Props["properties"].(string); ok {
				if err := json.Unmarshal([]byte(propsStr), &meta); err != nil {
					continue
				}
			}

			var content []byte
			if contentStr, ok := nodeData.Props["content"].(string); ok {
				content = []byte(contentStr)
			}

			node := &core.Node{
				ID:      nodeData.Props["id"].(string),
				Type:    nodeData.Props["type"].(string),
				Content: content,
				Meta:    meta,
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
func (r *Repository) SearchNodes(ctx context.Context, searchTerm string) ([]*core.Node, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (n:Node)
			WHERE n.id CONTAINS $term
			   OR n.type CONTAINS $term
			   OR n.properties CONTAINS $term
			   OR n.content CONTAINS $term
			RETURN n
		`

		result, err := tx.Run(ctx, query, map[string]any{"term": searchTerm})
		if err != nil {
			return nil, err
		}

		var nodes []*core.Node
		for result.Next(ctx) {
			record := result.Record()
			nodeValue, _ := record.Get("n")
			nodeData := nodeValue.(neo4j.Node)

			var meta map[string]any
			if propsStr, ok := nodeData.Props["properties"].(string); ok {
				if err := json.Unmarshal([]byte(propsStr), &meta); err != nil {
					continue
				}
			}

			var content []byte
			if contentStr, ok := nodeData.Props["content"].(string); ok {
				content = []byte(contentStr)
			}

			node := &core.Node{
				ID:      nodeData.Props["id"].(string),
				Type:    nodeData.Props["type"].(string),
				Content: content,
				Meta:    meta,
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

// TraverseGraph performs graph traversal from a starting node
func (r *Repository) TraverseGraph(ctx context.Context, startNodeID string, depth int, relationshipTypes []string) (map[string]*core.Node, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH path = (start:Node {id: $start_id})-[r:LINK*1..` + fmt.Sprintf("%d", depth) + `]->(n:Node)
		`

		params := map[string]any{"start_id": startNodeID}

		// Add relationship type filter
		if len(relationshipTypes) > 0 {
			query += ` WHERE ALL(rel in r WHERE rel.type IN $rel_types)`
			params["rel_types"] = relationshipTypes
		}

		query += ` RETURN DISTINCT n`

		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		nodes := make(map[string]*core.Node)
		for result.Next(ctx) {
			record := result.Record()
			nodeValue, _ := record.Get("n")
			nodeData := nodeValue.(neo4j.Node)

			var meta map[string]any
			if propsStr, ok := nodeData.Props["properties"].(string); ok {
				if err := json.Unmarshal([]byte(propsStr), &meta); err != nil {
					continue
				}
			}

			var content []byte
			if contentStr, ok := nodeData.Props["content"].(string); ok {
				content = []byte(contentStr)
			}

			node := &core.Node{
				ID:      nodeData.Props["id"].(string),
				Type:    nodeData.Props["type"].(string),
				Content: content,
				Meta:    meta,
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
