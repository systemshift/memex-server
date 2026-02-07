package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/systemshift/memex/internal/memex/core"
	"github.com/systemshift/memex/internal/server/subscriptions"
)

// SQLiteRepository implements Repository using SQLite
type SQLiteRepository struct {
	db           *sql.DB
	eventEmitter func(subscriptions.Event)
}

// NewSQLite creates a new SQLite repository
func NewSQLite(ctx context.Context, dbPath string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}

	// Verify connectivity
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("connecting to sqlite: %w", err)
	}

	repo := &SQLiteRepository{db: db}

	// Apply pragmas for optimal performance
	for _, pragma := range allPragmas() {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return nil, fmt.Errorf("setting pragma: %w", err)
		}
	}

	// Create schema
	for _, stmt := range allSchemaStatements() {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return nil, fmt.Errorf("creating schema: %w", err)
		}
	}

	return repo, nil
}

// Close closes the SQLite connection
func (r *SQLiteRepository) Close(ctx context.Context) error {
	return r.db.Close()
}

// SetEventEmitter sets the callback for emitting events
func (r *SQLiteRepository) SetEventEmitter(emitter func(subscriptions.Event)) {
	r.eventEmitter = emitter
}

// emit sends an event to the subscription manager if one is registered
func (r *SQLiteRepository) emit(event subscriptions.Event) {
	if r.eventEmitter != nil {
		r.eventEmitter(event)
	}
}

// EnsureIndexes creates necessary indexes (already created in schema)
func (r *SQLiteRepository) EnsureIndexes(ctx context.Context) error {
	// Indexes are created in NewSQLite via allSchemaStatements()
	// This method exists to satisfy the interface
	return nil
}

// CreateNode creates a new node in the graph
func (r *SQLiteRepository) CreateNode(ctx context.Context, node *core.Node) error {
	// Set version fields for new nodes
	node.Version = 1
	node.VersionID = node.ID + ":v1"
	node.IsCurrent = true

	metaJSON, err := json.Marshal(node.Meta)
	if err != nil {
		return fmt.Errorf("marshaling meta: %w", err)
	}

	query := `
		INSERT INTO nodes (version_id, id, version, is_current, type, content, properties, created_at, modified_at, deleted, degree)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0)
	`

	_, err = r.db.ExecContext(ctx, query,
		node.VersionID,
		node.ID,
		node.Version,
		boolToInt(node.IsCurrent),
		node.Type,
		string(node.Content),
		string(metaJSON),
		node.Created.Format(time.RFC3339),
		node.Modified.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting node: %w", err)
	}

	// Emit event
	r.emit(subscriptions.Event{
		ID:        uuid.New().String(),
		Type:      subscriptions.EventNodeCreated,
		Timestamp: time.Now(),
		NodeID:    node.ID,
		NodeType:  node.Type,
		Meta:      node.Meta,
	})

	return nil
}

// GetNode retrieves the current version of a node by ID
func (r *SQLiteRepository) GetNode(ctx context.Context, id string) (*core.Node, error) {
	query := `
		SELECT version_id, id, version, is_current, type, content, properties,
		       created_at, modified_at, deleted, deleted_at, change_note, changed_by, degree
		FROM nodes
		WHERE id = ? AND is_current = 1 AND deleted = 0
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanNode(row)
}

// GetNodeAtVersion retrieves a specific version of a node
func (r *SQLiteRepository) GetNodeAtVersion(ctx context.Context, id string, version int) (*core.Node, error) {
	query := `
		SELECT version_id, id, version, is_current, type, content, properties,
		       created_at, modified_at, deleted, deleted_at, change_note, changed_by, degree
		FROM nodes
		WHERE id = ? AND version = ?
	`

	row := r.db.QueryRowContext(ctx, query, id, version)
	return r.scanNode(row)
}

// GetNodeAtTime retrieves the version of a node that was current at a specific time
func (r *SQLiteRepository) GetNodeAtTime(ctx context.Context, id string, asOf time.Time) (*core.Node, error) {
	query := `
		SELECT version_id, id, version, is_current, type, content, properties,
		       created_at, modified_at, deleted, deleted_at, change_note, changed_by, degree
		FROM nodes
		WHERE id = ? AND modified_at <= ?
		ORDER BY version DESC
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, id, asOf.Format(time.RFC3339))
	return r.scanNode(row)
}

// GetNodeHistory returns all versions of a node ordered by version descending
func (r *SQLiteRepository) GetNodeHistory(ctx context.Context, id string) ([]core.VersionInfo, error) {
	query := `
		SELECT version, version_id, modified_at, change_note, changed_by, is_current
		FROM nodes
		WHERE id = ?
		ORDER BY version DESC
	`

	rows, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []core.VersionInfo
	for rows.Next() {
		var vi core.VersionInfo
		var modifiedAt string
		var changeNote, changedBy sql.NullString
		var isCurrent int

		if err := rows.Scan(&vi.Version, &vi.VersionID, &modifiedAt, &changeNote, &changedBy, &isCurrent); err != nil {
			return nil, err
		}

		if t, err := time.Parse(time.RFC3339, modifiedAt); err == nil {
			vi.Modified = t
		}
		if changeNote.Valid {
			vi.ChangeNote = changeNote.String
		}
		if changedBy.Valid {
			vi.ChangedBy = changedBy.String
		}
		vi.IsCurrent = isCurrent == 1

		versions = append(versions, vi)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("node not found: %s", id)
	}

	return versions, nil
}

// GetLinks retrieves all links for a node
func (r *SQLiteRepository) GetLinks(ctx context.Context, nodeID string) ([]*core.Link, error) {
	query := `
		SELECT source_id, target_id, type, properties, created_at, modified_at
		FROM links
		WHERE source_id = ?
	`

	rows, err := r.db.QueryContext(ctx, query, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []*core.Link
	for rows.Next() {
		link, err := r.scanLink(rows)
		if err != nil {
			continue
		}
		links = append(links, link)
	}

	return links, nil
}

// SearchNodes performs full-text search using FTS5
func (r *SQLiteRepository) SearchNodes(ctx context.Context, searchTerm string, limit int, offset int) ([]*core.Node, error) {
	// Escape special FTS5 characters and wrap in quotes for phrase search
	escapedTerm := strings.ReplaceAll(searchTerm, "\"", "\"\"")
	ftsQuery := fmt.Sprintf("\"%s\"", escapedTerm)

	query := `
		SELECT n.version_id, n.id, n.version, n.is_current, n.type, n.content, n.properties,
		       n.created_at, n.modified_at, n.deleted, n.deleted_at, n.change_note, n.changed_by, n.degree
		FROM nodes n
		JOIN nodes_fts fts ON n.rowid = fts.rowid
		WHERE nodes_fts MATCH ?
		  AND n.is_current = 1 AND n.deleted = 0
		ORDER BY rank
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, ftsQuery, limit, offset)
	if err != nil {
		// Fallback to LIKE search if FTS fails
		return r.searchNodesLike(ctx, searchTerm, limit, offset)
	}
	defer rows.Close()

	return r.scanNodes(rows)
}

// searchNodesLike is a fallback search using LIKE
func (r *SQLiteRepository) searchNodesLike(ctx context.Context, searchTerm string, limit int, offset int) ([]*core.Node, error) {
	likeTerm := "%" + searchTerm + "%"
	query := `
		SELECT version_id, id, version, is_current, type, content, properties,
		       created_at, modified_at, deleted, deleted_at, change_note, changed_by, degree
		FROM nodes
		WHERE is_current = 1 AND deleted = 0
		  AND (id LIKE ? OR type LIKE ? OR properties LIKE ? OR content LIKE ?)
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, likeTerm, likeTerm, likeTerm, likeTerm, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanNodes(rows)
}

// FilterNodes returns nodes matching filter criteria
func (r *SQLiteRepository) FilterNodes(ctx context.Context, nodeTypes []string, propertyKey string, propertyValue string, limit int, offset int) ([]*core.Node, error) {
	query := `
		SELECT version_id, id, version, is_current, type, content, properties,
		       created_at, modified_at, deleted, deleted_at, change_note, changed_by, degree
		FROM nodes
		WHERE is_current = 1 AND deleted = 0
	`
	args := []interface{}{}

	if len(nodeTypes) > 0 {
		placeholders := make([]string, len(nodeTypes))
		for i, t := range nodeTypes {
			placeholders[i] = "?"
			args = append(args, t)
		}
		query += " AND type IN (" + strings.Join(placeholders, ",") + ")"
	}

	if propertyKey != "" && propertyValue != "" {
		searchValue := fmt.Sprintf(`"%s":"%s"`, propertyKey, propertyValue)
		query += " AND properties LIKE ?"
		args = append(args, "%"+searchValue+"%")
	}

	query += " LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanNodes(rows)
}

// TraverseGraph performs graph traversal using recursive CTE
func (r *SQLiteRepository) TraverseGraph(ctx context.Context, startNodeID string, depth int, relationshipTypes []string, limit int, offset int) (map[string]*core.Node, error) {
	// Build the recursive CTE query
	relTypeFilter := ""
	args := []interface{}{startNodeID, depth}

	if len(relationshipTypes) > 0 {
		placeholders := make([]string, len(relationshipTypes))
		for i, t := range relationshipTypes {
			placeholders[i] = "?"
			args = append(args, t)
		}
		relTypeFilter = " AND l.type IN (" + strings.Join(placeholders, ",") + ")"
	}

	query := fmt.Sprintf(`
		WITH RECURSIVE traverse(id, depth) AS (
			SELECT ?, 0
			UNION ALL
			SELECT l.target_id, t.depth + 1
			FROM traverse t
			JOIN links l ON l.source_id = t.id
			WHERE t.depth < ?%s
		)
		SELECT DISTINCT n.version_id, n.id, n.version, n.is_current, n.type, n.content, n.properties,
		       n.created_at, n.modified_at, n.deleted, n.deleted_at, n.change_note, n.changed_by, n.degree
		FROM traverse t
		JOIN nodes n ON n.id = t.id
		WHERE n.is_current = 1 AND n.deleted = 0
		LIMIT ? OFFSET ?
	`, relTypeFilter)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes, err := r.scanNodes(rows)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*core.Node)
	for _, node := range nodes {
		result[node.ID] = node
	}

	return result, nil
}

// CreateLink creates a relationship between two nodes
func (r *SQLiteRepository) CreateLink(ctx context.Context, link *core.Link) error {
	metaJSON, err := json.Marshal(link.Meta)
	if err != nil {
		return fmt.Errorf("marshaling meta: %w", err)
	}

	query := `
		INSERT INTO links (source_id, target_id, type, properties, created_at, modified_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.ExecContext(ctx, query,
		link.Source,
		link.Target,
		link.Type,
		string(metaJSON),
		link.Created.Format(time.RFC3339),
		link.Modified.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting link: %w", err)
	}

	// Update degree counts
	r.updateDegree(ctx, link.Source, 1)
	r.updateDegree(ctx, link.Target, 1)

	// Emit event
	r.emit(subscriptions.Event{
		ID:         uuid.New().String(),
		Type:       subscriptions.EventLinkCreated,
		Timestamp:  time.Now(),
		LinkSource: link.Source,
		LinkTarget: link.Target,
		LinkType:   link.Type,
		Meta:       link.Meta,
	})

	return nil
}

// DeleteLink deletes a specific relationship between two nodes
func (r *SQLiteRepository) DeleteLink(ctx context.Context, sourceID string, targetID string, linkType string) error {
	query := `DELETE FROM links WHERE source_id = ? AND target_id = ? AND type = ?`

	result, err := r.db.ExecContext(ctx, query, sourceID, targetID, linkType)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return fmt.Errorf("link not found: %s -[%s]-> %s", sourceID, linkType, targetID)
	}

	// Update degree counts
	r.updateDegree(ctx, sourceID, -1)
	r.updateDegree(ctx, targetID, -1)

	// Emit event
	r.emit(subscriptions.Event{
		ID:         uuid.New().String(),
		Type:       subscriptions.EventLinkDeleted,
		Timestamp:  time.Now(),
		LinkSource: sourceID,
		LinkTarget: targetID,
		LinkType:   linkType,
	})

	return nil
}

// ListNodes returns all node IDs
func (r *SQLiteRepository) ListNodes(ctx context.Context) ([]string, error) {
	query := `
		SELECT DISTINCT id FROM nodes
		WHERE is_current = 1 AND deleted = 0
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// UpdateNodeMeta creates a new version of a node with updated metadata
func (r *SQLiteRepository) UpdateNodeMeta(ctx context.Context, id string, meta map[string]any) error {
	return r.UpdateNodeMetaWithNote(ctx, id, meta, "", "")
}

// UpdateNodeMetaWithNote creates a new version with a change note
func (r *SQLiteRepository) UpdateNodeMetaWithNote(ctx context.Context, id string, meta map[string]any, changeNote, changedBy string) error {
	// Get current version
	current, err := r.GetNode(ctx, id)
	if err != nil {
		return fmt.Errorf("node not found: %s", id)
	}

	// Merge meta
	existingMeta := current.Meta
	if existingMeta == nil {
		existingMeta = make(map[string]any)
	}
	for k, v := range meta {
		existingMeta[k] = v
	}

	metaJSON, err := json.Marshal(existingMeta)
	if err != nil {
		return fmt.Errorf("marshaling meta: %w", err)
	}

	newVersion := current.Version + 1
	newVersionID := id + ":v" + fmt.Sprintf("%d", newVersion)
	now := time.Now()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Mark current version as no longer current
	_, err = tx.ExecContext(ctx, `UPDATE nodes SET is_current = 0 WHERE id = ? AND is_current = 1`, id)
	if err != nil {
		return fmt.Errorf("marking old version: %w", err)
	}

	// Create new version
	degree := 0
	if d, ok := current.Meta["degree"]; ok {
		if dVal, ok := d.(float64); ok {
			degree = int(dVal)
		}
	}

	query := `
		INSERT INTO nodes (version_id, id, version, is_current, type, content, properties,
		                   created_at, modified_at, deleted, degree, change_note, changed_by)
		VALUES (?, ?, ?, 1, ?, ?, ?, ?, ?, 0, ?, ?, ?)
	`
	_, err = tx.ExecContext(ctx, query,
		newVersionID,
		id,
		newVersion,
		current.Type,
		string(current.Content),
		string(metaJSON),
		current.Created.Format(time.RFC3339),
		now.Format(time.RFC3339),
		degree,
		changeNote,
		changedBy,
	)
	if err != nil {
		return fmt.Errorf("creating new version: %w", err)
	}

	// Create version chain link
	_, err = tx.ExecContext(ctx, `INSERT INTO version_chain (newer_version_id, older_version_id) VALUES (?, ?)`,
		newVersionID, current.VersionID)
	if err != nil {
		return fmt.Errorf("creating version link: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Emit event
	r.emit(subscriptions.Event{
		ID:        uuid.New().String(),
		Type:      subscriptions.EventNodeUpdated,
		Timestamp: time.Now(),
		NodeID:    id,
		NodeType:  current.Type,
		Meta: map[string]any{
			"version":      newVersion,
			"prev_version": current.Version,
			"change_note":  changeNote,
			"updated_meta": meta,
		},
	})

	return nil
}

// DeleteNode marks a node as deleted (tombstone)
func (r *SQLiteRepository) DeleteNode(ctx context.Context, nodeID string, force bool) error {
	// Check if node exists
	current, err := r.GetNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("node not found or already deleted: %s", nodeID)
	}

	// Protect Source layer unless force=true
	if !force && len(nodeID) > 7 && nodeID[:7] == "sha256:" {
		return fmt.Errorf("cannot delete Source layer node (content-addressed): %s", nodeID)
	}

	if force {
		// Hard delete
		_, err := r.db.ExecContext(ctx, `DELETE FROM nodes WHERE id = ?`, nodeID)
		if err != nil {
			return err
		}
		_, err = r.db.ExecContext(ctx, `DELETE FROM links WHERE source_id = ? OR target_id = ?`, nodeID, nodeID)
		if err != nil {
			return err
		}
	} else {
		// Soft delete: create tombstone version
		newVersion := current.Version + 1
		newVersionID := nodeID + ":v" + fmt.Sprintf("%d", newVersion)
		now := time.Now()

		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		// Mark current as not current
		_, err = tx.ExecContext(ctx, `UPDATE nodes SET is_current = 0 WHERE id = ? AND is_current = 1`, nodeID)
		if err != nil {
			return err
		}

		// Create tombstone
		query := `
			INSERT INTO nodes (version_id, id, version, is_current, type, content, properties,
			                   created_at, modified_at, deleted, deleted_at, degree, change_note)
			VALUES (?, ?, ?, 1, ?, '', '{}', ?, ?, 1, ?, 0, 'Deleted')
		`
		_, err = tx.ExecContext(ctx, query,
			newVersionID,
			nodeID,
			newVersion,
			current.Type,
			current.Created.Format(time.RFC3339),
			now.Format(time.RFC3339),
			now.Format(time.RFC3339),
		)
		if err != nil {
			return err
		}

		// Version chain
		_, err = tx.ExecContext(ctx, `INSERT INTO version_chain (newer_version_id, older_version_id) VALUES (?, ?)`,
			newVersionID, current.VersionID)
		if err != nil {
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	// Emit event
	r.emit(subscriptions.Event{
		ID:        uuid.New().String(),
		Type:      subscriptions.EventNodeDeleted,
		Timestamp: time.Now(),
		NodeID:    nodeID,
	})

	return nil
}

// GetSubgraph extracts a subgraph centered on a start node
func (r *SQLiteRepository) GetSubgraph(ctx context.Context, startNodeID string, depth int, relationshipTypes []string) (*Subgraph, error) {
	// Get nodes within depth hops
	nodesMap, err := r.TraverseGraph(ctx, startNodeID, depth, relationshipTypes, 1000, 0)
	if err != nil {
		return nil, err
	}

	// Collect node IDs
	nodeIDs := make([]string, 0, len(nodesMap))
	for id := range nodesMap {
		nodeIDs = append(nodeIDs, id)
	}

	// Get edges between these nodes
	var edges []*SubgraphEdge
	if len(nodeIDs) > 0 {
		placeholders := make([]string, len(nodeIDs))
		args := make([]interface{}, len(nodeIDs)*2)
		for i, id := range nodeIDs {
			placeholders[i] = "?"
			args[i] = id
			args[i+len(nodeIDs)] = id
		}

		query := fmt.Sprintf(`
			SELECT source_id, target_id, type, properties
			FROM links
			WHERE source_id IN (%s) AND target_id IN (%s)
		`, strings.Join(placeholders, ","), strings.Join(placeholders, ","))

		if len(relationshipTypes) > 0 {
			typePlaceholders := make([]string, len(relationshipTypes))
			for i, t := range relationshipTypes {
				typePlaceholders[i] = "?"
				args = append(args, t)
			}
			query += " AND type IN (" + strings.Join(typePlaceholders, ",") + ")"
		}

		rows, err := r.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var sourceID, targetID, linkType string
			var propsStr sql.NullString
			if err := rows.Scan(&sourceID, &targetID, &linkType, &propsStr); err != nil {
				continue
			}

			var meta map[string]interface{}
			if propsStr.Valid {
				json.Unmarshal([]byte(propsStr.String), &meta)
			}

			edges = append(edges, &SubgraphEdge{
				Source: sourceID,
				Target: targetID,
				Type:   linkType,
				Meta:   meta,
			})
		}
	}

	// Convert map to slice
	nodes := make([]*core.Node, 0, len(nodesMap))
	for _, node := range nodesMap {
		nodes = append(nodes, node)
	}

	return &Subgraph{
		Nodes: nodes,
		Edges: edges,
		Stats: SubgraphStats{
			NodeCount: len(nodes),
			EdgeCount: len(edges),
			Depth:     depth,
		},
	}, nil
}

// UpdateAttentionEdge creates or updates an attention-weighted edge
func (r *SQLiteRepository) UpdateAttentionEdge(ctx context.Context, source, target, queryID string, weight float64) error {
	// Check if ATTENDED edge exists
	var propsStr sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT properties FROM links WHERE source_id = ? AND target_id = ? AND type = 'ATTENDED'`,
		source, target).Scan(&propsStr)

	if err == sql.ErrNoRows {
		// Create new edge
		meta := map[string]interface{}{
			"weight":        weight,
			"query_count":   1,
			"last_updated":  time.Now().Format(time.RFC3339),
			"last_query_id": queryID,
		}
		metaJSON, _ := json.Marshal(meta)

		_, err = r.db.ExecContext(ctx,
			`INSERT INTO links (source_id, target_id, type, properties, created_at, modified_at) VALUES (?, ?, 'ATTENDED', ?, ?, ?)`,
			source, target, string(metaJSON), time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339))
		return err
	} else if err != nil {
		return err
	}

	// Update existing edge
	var meta map[string]interface{}
	if propsStr.Valid {
		json.Unmarshal([]byte(propsStr.String), &meta)
	} else {
		meta = make(map[string]interface{})
	}

	currentWeight := 0.0
	currentCount := 0.0
	if w, ok := meta["weight"].(float64); ok {
		currentWeight = w
	}
	if c, ok := meta["query_count"].(float64); ok {
		currentCount = c
	}

	newWeight := (currentWeight*currentCount + weight) / (currentCount + 1)
	newCount := currentCount + 1

	meta["weight"] = newWeight
	meta["query_count"] = newCount
	meta["last_updated"] = time.Now().Format(time.RFC3339)
	meta["last_query_id"] = queryID

	metaJSON, _ := json.Marshal(meta)

	_, err = r.db.ExecContext(ctx,
		`UPDATE links SET properties = ?, modified_at = ? WHERE source_id = ? AND target_id = ? AND type = 'ATTENDED'`,
		string(metaJSON), time.Now().Format(time.RFC3339), source, target)
	return err
}

// GetAttentionSubgraph extracts nodes connected by high-weight attention edges
func (r *SQLiteRepository) GetAttentionSubgraph(ctx context.Context, startNodeID string, minWeight float64, maxNodes int) (*Subgraph, error) {
	// Get nodes connected by ATTENDED edges
	query := `
		SELECT DISTINCT n.version_id, n.id, n.version, n.is_current, n.type, n.content, n.properties,
		       n.created_at, n.modified_at, n.deleted, n.deleted_at, n.change_note, n.changed_by, n.degree,
		       l.properties as link_props
		FROM links l
		JOIN nodes n ON (n.id = l.target_id OR n.id = l.source_id) AND n.id != ?
		WHERE (l.source_id = ? OR l.target_id = ?)
		  AND l.type = 'ATTENDED'
		  AND n.is_current = 1 AND n.deleted = 0
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, startNodeID, startNodeID, startNodeID, maxNodes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodeMap := make(map[string]*core.Node)
	for rows.Next() {
		var versionID, id, nodeType string
		var version, isCurrent, deleted, degree int
		var content, properties, createdAt, modifiedAt string
		var deletedAt, changeNote, changedBy, linkProps sql.NullString

		if err := rows.Scan(&versionID, &id, &version, &isCurrent, &nodeType, &content, &properties,
			&createdAt, &modifiedAt, &deleted, &deletedAt, &changeNote, &changedBy, &degree, &linkProps); err != nil {
			continue
		}

		// Filter by weight
		if linkProps.Valid {
			var meta map[string]interface{}
			json.Unmarshal([]byte(linkProps.String), &meta)
			if w, ok := meta["weight"].(float64); ok && w < minWeight {
				continue
			}
		}

		node := &core.Node{
			VersionID: versionID,
			ID:        id,
			Version:   version,
			IsCurrent: isCurrent == 1,
			Type:      nodeType,
			Content:   []byte(content),
			Deleted:   deleted == 1,
		}

		if properties != "" {
			json.Unmarshal([]byte(properties), &node.Meta)
		}
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			node.Created = t
		}
		if t, err := time.Parse(time.RFC3339, modifiedAt); err == nil {
			node.Modified = t
		}
		if deletedAt.Valid {
			if t, err := time.Parse(time.RFC3339, deletedAt.String); err == nil {
				node.DeletedAt = t
			}
		}
		if changeNote.Valid {
			node.ChangeNote = changeNote.String
		}
		if changedBy.Valid {
			node.ChangedBy = changedBy.String
		}

		nodeMap[id] = node
	}

	// Get edges between these nodes
	nodeIDs := make([]string, 0, len(nodeMap))
	for id := range nodeMap {
		nodeIDs = append(nodeIDs, id)
	}

	var edges []*SubgraphEdge
	if len(nodeIDs) > 0 {
		placeholders := make([]string, len(nodeIDs))
		args := make([]interface{}, len(nodeIDs)*2)
		for i, id := range nodeIDs {
			placeholders[i] = "?"
			args[i] = id
			args[i+len(nodeIDs)] = id
		}

		edgeQuery := fmt.Sprintf(`
			SELECT source_id, target_id, type, properties
			FROM links
			WHERE source_id IN (%s) AND target_id IN (%s) AND type = 'ATTENDED'
		`, strings.Join(placeholders, ","), strings.Join(placeholders, ","))

		edgeRows, err := r.db.QueryContext(ctx, edgeQuery, args...)
		if err == nil {
			defer edgeRows.Close()
			for edgeRows.Next() {
				var sourceID, targetID, linkType string
				var propsStr sql.NullString
				if err := edgeRows.Scan(&sourceID, &targetID, &linkType, &propsStr); err != nil {
					continue
				}

				var meta map[string]interface{}
				if propsStr.Valid {
					json.Unmarshal([]byte(propsStr.String), &meta)
				}

				edges = append(edges, &SubgraphEdge{
					Source: sourceID,
					Target: targetID,
					Type:   linkType,
					Meta:   meta,
				})
			}
		}
	}

	nodes := make([]*core.Node, 0, len(nodeMap))
	for _, node := range nodeMap {
		nodes = append(nodes, node)
	}

	return &Subgraph{
		Nodes: nodes,
		Edges: edges,
		Stats: SubgraphStats{
			NodeCount: len(nodes),
			EdgeCount: len(edges),
			Depth:     2,
		},
	}, nil
}

// PruneWeakAttentionEdges removes attention edges with low weight or query count
func (r *SQLiteRepository) PruneWeakAttentionEdges(ctx context.Context, minWeight float64, minQueryCount int) (int, error) {
	// Get all ATTENDED edges
	rows, err := r.db.QueryContext(ctx, `SELECT id, properties FROM links WHERE type = 'ATTENDED'`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var idsToDelete []int64
	for rows.Next() {
		var id int64
		var propsStr sql.NullString
		if err := rows.Scan(&id, &propsStr); err != nil {
			continue
		}

		if propsStr.Valid {
			var meta map[string]interface{}
			json.Unmarshal([]byte(propsStr.String), &meta)

			shouldDelete := false
			if w, ok := meta["weight"].(float64); ok && w < minWeight {
				shouldDelete = true
			}
			if c, ok := meta["query_count"].(float64); ok && int(c) < minQueryCount {
				shouldDelete = true
			}

			if shouldDelete {
				idsToDelete = append(idsToDelete, id)
			}
		}
	}

	// Delete collected edges
	if len(idsToDelete) > 0 {
		placeholders := make([]string, len(idsToDelete))
		args := make([]interface{}, len(idsToDelete))
		for i, id := range idsToDelete {
			placeholders[i] = "?"
			args[i] = id
		}

		_, err = r.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM links WHERE id IN (%s)`, strings.Join(placeholders, ",")), args...)
		if err != nil {
			return 0, err
		}
	}

	return len(idsToDelete), nil
}

// GetGraphMap returns a high-level map of the graph
func (r *SQLiteRepository) GetGraphMap(ctx context.Context, sampleSize int) (*GraphMap, error) {
	graphMap := &GraphMap{
		NodeTypes:     make(map[string]int),
		EdgeTypes:     make(map[string]int),
		SamplesByType: make(map[string][]string),
	}

	// Count nodes by type
	rows, err := r.db.QueryContext(ctx, `
		SELECT type, COUNT(*) as count FROM nodes
		WHERE is_current = 1 AND deleted = 0
		GROUP BY type
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totalNodes := 0
	for rows.Next() {
		var nodeType string
		var count int
		if err := rows.Scan(&nodeType, &count); err != nil {
			continue
		}
		graphMap.NodeTypes[nodeType] = count
		totalNodes += count
	}
	graphMap.Stats.TotalNodes = totalNodes

	// Count edges by type
	edgeRows, err := r.db.QueryContext(ctx, `SELECT type, COUNT(*) as count FROM links GROUP BY type`)
	if err != nil {
		return nil, err
	}
	defer edgeRows.Close()

	totalEdges := 0
	for edgeRows.Next() {
		var edgeType string
		var count int
		if err := edgeRows.Scan(&edgeType, &count); err != nil {
			continue
		}
		graphMap.EdgeTypes[edgeType] = count
		totalEdges += count
	}
	graphMap.Stats.TotalEdges = totalEdges

	// Get top connected nodes
	topRows, err := r.db.QueryContext(ctx, `
		SELECT id, type, degree FROM nodes
		WHERE is_current = 1 AND deleted = 0 AND degree > 0
		ORDER BY degree DESC
		LIMIT ?
	`, sampleSize)
	if err != nil {
		return nil, err
	}
	defer topRows.Close()

	for topRows.Next() {
		var id, nodeType string
		var degree int
		if err := topRows.Scan(&id, &nodeType, &degree); err != nil {
			continue
		}
		graphMap.TopConnected = append(graphMap.TopConnected, NodeSummary{
			ID:     id,
			Type:   nodeType,
			Degree: degree,
		})
	}

	// Get samples per type
	for nodeType := range graphMap.NodeTypes {
		sampleRows, err := r.db.QueryContext(ctx, `
			SELECT id FROM nodes WHERE type = ? AND is_current = 1 AND deleted = 0 LIMIT ?
		`, nodeType, sampleSize)
		if err != nil {
			continue
		}

		var samples []string
		for sampleRows.Next() {
			var id string
			if err := sampleRows.Scan(&id); err != nil {
				continue
			}
			samples = append(samples, id)
		}
		sampleRows.Close()
		graphMap.SamplesByType[nodeType] = samples
	}

	return graphMap, nil
}

// GetEntitiesInterpretedThrough returns entities linked to a lens via INTERPRETED_THROUGH
func (r *SQLiteRepository) GetEntitiesInterpretedThrough(ctx context.Context, lensID string) ([]*core.Node, error) {
	query := `
		SELECT n.version_id, n.id, n.version, n.is_current, n.type, n.content, n.properties,
		       n.created_at, n.modified_at, n.deleted, n.deleted_at, n.change_note, n.changed_by, n.degree,
		       l.properties as link_props
		FROM nodes n
		JOIN links l ON l.source_id = n.id
		WHERE l.target_id = ? AND l.type = 'INTERPRETED_THROUGH'
		  AND n.is_current = 1 AND n.deleted = 0
	`

	rows, err := r.db.QueryContext(ctx, query, lensID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*core.Node
	for rows.Next() {
		var versionID, id, nodeType string
		var version, isCurrent, deleted, degree int
		var content, properties, createdAt, modifiedAt string
		var deletedAt, changeNote, changedBy, linkProps sql.NullString

		if err := rows.Scan(&versionID, &id, &version, &isCurrent, &nodeType, &content, &properties,
			&createdAt, &modifiedAt, &deleted, &deletedAt, &changeNote, &changedBy, &degree, &linkProps); err != nil {
			continue
		}

		node := &core.Node{
			VersionID: versionID,
			ID:        id,
			Version:   version,
			IsCurrent: isCurrent == 1,
			Type:      nodeType,
			Content:   []byte(content),
			Deleted:   deleted == 1,
		}

		if properties != "" {
			json.Unmarshal([]byte(properties), &node.Meta)
		}
		if node.Meta == nil {
			node.Meta = make(map[string]interface{})
		}
		if linkProps.Valid {
			var linkMeta map[string]interface{}
			if json.Unmarshal([]byte(linkProps.String), &linkMeta) == nil {
				node.Meta["_interpretation"] = linkMeta
			}
		}
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			node.Created = t
		}
		if t, err := time.Parse(time.RFC3339, modifiedAt); err == nil {
			node.Modified = t
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

// CreateInterpretedThroughLink creates an INTERPRETED_THROUGH link
func (r *SQLiteRepository) CreateInterpretedThroughLink(ctx context.Context, entityID, lensID string, meta map[string]interface{}) error {
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

// QueryByLens returns entities interpreted through a lens with optional pattern filter
func (r *SQLiteRepository) QueryByLens(ctx context.Context, lensID string, pattern string, limit int, offset int) ([]*core.Node, error) {
	query := `
		SELECT n.version_id, n.id, n.version, n.is_current, n.type, n.content, n.properties,
		       n.created_at, n.modified_at, n.deleted, n.deleted_at, n.change_note, n.changed_by, n.degree,
		       l.properties as link_props
		FROM nodes n
		JOIN links l ON l.source_id = n.id
		WHERE l.target_id = ? AND l.type = 'INTERPRETED_THROUGH'
		  AND n.is_current = 1 AND n.deleted = 0
	`
	args := []interface{}{lensID}

	if pattern != "" {
		query += " AND l.properties LIKE ?"
		args = append(args, "%"+pattern+"%")
	}

	query += " LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*core.Node
	for rows.Next() {
		var versionID, id, nodeType string
		var version, isCurrent, deleted, degree int
		var content, properties, createdAt, modifiedAt string
		var deletedAt, changeNote, changedBy, linkProps sql.NullString

		if err := rows.Scan(&versionID, &id, &version, &isCurrent, &nodeType, &content, &properties,
			&createdAt, &modifiedAt, &deleted, &deletedAt, &changeNote, &changedBy, &degree, &linkProps); err != nil {
			continue
		}

		node := &core.Node{
			VersionID: versionID,
			ID:        id,
			Version:   version,
			IsCurrent: isCurrent == 1,
			Type:      nodeType,
			Content:   []byte(content),
			Deleted:   deleted == 1,
		}

		if properties != "" {
			json.Unmarshal([]byte(properties), &node.Meta)
		}
		if node.Meta == nil {
			node.Meta = make(map[string]interface{})
		}
		if linkProps.Valid {
			var linkMeta map[string]interface{}
			if json.Unmarshal([]byte(linkProps.String), &linkMeta) == nil {
				node.Meta["_interpretation"] = linkMeta
			}
		}
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			node.Created = t
		}
		if t, err := time.Parse(time.RFC3339, modifiedAt); err == nil {
			node.Modified = t
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

// ExportLens returns a complete export of a lens and its entities
func (r *SQLiteRepository) ExportLens(ctx context.Context, lensID string, includeExtractedFrom bool) (*LensExport, error) {
	export := &LensExport{}

	// Get the lens node
	lens, err := r.GetNode(ctx, lensID)
	if err != nil {
		return nil, fmt.Errorf("lens not found: %s", lensID)
	}
	export.Lens = lens

	// Get entities
	entities, err := r.GetEntitiesInterpretedThrough(ctx, lensID)
	if err != nil {
		return nil, err
	}
	export.Entities = entities

	// Get INTERPRETED_THROUGH links
	for _, entity := range entities {
		export.Links = append(export.Links, &SubgraphEdge{
			Source: entity.ID,
			Target: lensID,
			Type:   "INTERPRETED_THROUGH",
			Meta:   entity.Meta["_interpretation"].(map[string]interface{}),
		})
	}

	// Get EXTRACTED_FROM links if requested
	if includeExtractedFrom && len(entities) > 0 {
		entityIDs := make([]string, len(entities))
		for i, e := range entities {
			entityIDs[i] = e.ID
		}

		placeholders := make([]string, len(entityIDs))
		args := make([]interface{}, len(entityIDs))
		for i, id := range entityIDs {
			placeholders[i] = "?"
			args[i] = id
		}

		query := fmt.Sprintf(`
			SELECT source_id, target_id, properties FROM links
			WHERE source_id IN (%s) AND type = 'EXTRACTED_FROM'
		`, strings.Join(placeholders, ","))

		rows, err := r.db.QueryContext(ctx, query, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var sourceID, targetID string
				var propsStr sql.NullString
				if err := rows.Scan(&sourceID, &targetID, &propsStr); err != nil {
					continue
				}

				var meta map[string]interface{}
				if propsStr.Valid {
					json.Unmarshal([]byte(propsStr.String), &meta)
				}

				export.Links = append(export.Links, &SubgraphEdge{
					Source: sourceID,
					Target: targetID,
					Type:   "EXTRACTED_FROM",
					Meta:   meta,
				})
			}
		}
	}

	export.Stats.EntityCount = len(export.Entities)
	export.Stats.LinkCount = len(export.Links)

	return export, nil
}

// Subscription persistence methods

// CreateSubscriptionNode persists a subscription as a node
func (r *SQLiteRepository) CreateSubscriptionNode(ctx context.Context, sub *subscriptions.Subscription) error {
	patternJSON, err := json.Marshal(sub.Pattern)
	if err != nil {
		return fmt.Errorf("marshaling pattern: %w", err)
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
		return fmt.Errorf("marshaling meta: %w", err)
	}

	node := &core.Node{
		ID:       "subscription:" + sub.ID,
		Type:     "Subscription",
		Meta:     meta,
		Created:  sub.Created,
		Modified: sub.Modified,
	}
	node.Content = metaJSON

	return r.CreateNode(ctx, node)
}

// UpdateSubscriptionNode updates a subscription node
func (r *SQLiteRepository) UpdateSubscriptionNode(ctx context.Context, sub *subscriptions.Subscription) error {
	patternJSON, err := json.Marshal(sub.Pattern)
	if err != nil {
		return fmt.Errorf("marshaling pattern: %w", err)
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

	return r.UpdateNodeMeta(ctx, "subscription:"+sub.ID, meta)
}

// DeleteSubscriptionNode removes a subscription node
func (r *SQLiteRepository) DeleteSubscriptionNode(ctx context.Context, id string) error {
	return r.DeleteNode(ctx, "subscription:"+id, true)
}

// LoadSubscriptions loads all subscription nodes
func (r *SQLiteRepository) LoadSubscriptions(ctx context.Context) ([]*subscriptions.Subscription, error) {
	nodes, err := r.FilterNodes(ctx, []string{"Subscription"}, "", "", 1000, 0)
	if err != nil {
		return nil, err
	}

	var subs []*subscriptions.Subscription
	for _, node := range nodes {
		id := node.ID
		if len(id) > 13 && id[:13] == "subscription:" {
			id = id[13:]
		}

		sub := &subscriptions.Subscription{
			ID:       id,
			Created:  node.Created,
			Modified: node.Modified,
		}

		if node.Meta != nil {
			if v, ok := node.Meta["name"].(string); ok {
				sub.Name = v
			}
			if v, ok := node.Meta["description"].(string); ok {
				sub.Description = v
			}
			if v, ok := node.Meta["webhook"].(string); ok {
				sub.Webhook = v
			}
			if v, ok := node.Meta["websocket"].(bool); ok {
				sub.WebSocket = v
			}
			if v, ok := node.Meta["enabled"].(bool); ok {
				sub.Enabled = v
			}
			if v, ok := node.Meta["fire_count"].(float64); ok {
				sub.FireCount = int(v)
			}
			if v, ok := node.Meta["last_fired"].(string); ok {
				if t, err := time.Parse(time.RFC3339, v); err == nil {
					sub.LastFired = &t
				}
			}
			if v, ok := node.Meta["pattern"].(string); ok {
				var pattern subscriptions.SubscriptionPattern
				if err := json.Unmarshal([]byte(v), &pattern); err == nil {
					sub.Pattern = pattern
				}
			}
		}

		subs = append(subs, sub)
	}

	return subs, nil
}

// ExecuteCypherRead returns error - Cypher not supported in SQLite
func (r *SQLiteRepository) ExecuteCypherRead(ctx context.Context, cypher string, params map[string]interface{}) ([]map[string]interface{}, error) {
	return nil, fmt.Errorf("Cypher queries are not supported with SQLite backend. Use Neo4j backend for Cypher support")
}

// Helper functions

func (r *SQLiteRepository) scanNode(row *sql.Row) (*core.Node, error) {
	var versionID, id, nodeType string
	var version, isCurrent, deleted, degree int
	var content, properties, createdAt, modifiedAt string
	var deletedAt, changeNote, changedBy sql.NullString

	err := row.Scan(&versionID, &id, &version, &isCurrent, &nodeType, &content, &properties,
		&createdAt, &modifiedAt, &deleted, &deletedAt, &changeNote, &changedBy, &degree)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("node not found")
		}
		return nil, err
	}

	node := &core.Node{
		VersionID: versionID,
		ID:        id,
		Version:   version,
		IsCurrent: isCurrent == 1,
		Type:      nodeType,
		Content:   []byte(content),
		Deleted:   deleted == 1,
	}

	if properties != "" {
		json.Unmarshal([]byte(properties), &node.Meta)
	}
	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		node.Created = t
	}
	if t, err := time.Parse(time.RFC3339, modifiedAt); err == nil {
		node.Modified = t
	}
	if deletedAt.Valid {
		if t, err := time.Parse(time.RFC3339, deletedAt.String); err == nil {
			node.DeletedAt = t
		}
	}
	if changeNote.Valid {
		node.ChangeNote = changeNote.String
	}
	if changedBy.Valid {
		node.ChangedBy = changedBy.String
	}

	return node, nil
}

func (r *SQLiteRepository) scanNodes(rows *sql.Rows) ([]*core.Node, error) {
	var nodes []*core.Node
	for rows.Next() {
		var versionID, id, nodeType string
		var version, isCurrent, deleted, degree int
		var content, properties, createdAt, modifiedAt string
		var deletedAt, changeNote, changedBy sql.NullString

		if err := rows.Scan(&versionID, &id, &version, &isCurrent, &nodeType, &content, &properties,
			&createdAt, &modifiedAt, &deleted, &deletedAt, &changeNote, &changedBy, &degree); err != nil {
			continue
		}

		node := &core.Node{
			VersionID: versionID,
			ID:        id,
			Version:   version,
			IsCurrent: isCurrent == 1,
			Type:      nodeType,
			Content:   []byte(content),
			Deleted:   deleted == 1,
		}

		if properties != "" {
			json.Unmarshal([]byte(properties), &node.Meta)
		}
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			node.Created = t
		}
		if t, err := time.Parse(time.RFC3339, modifiedAt); err == nil {
			node.Modified = t
		}
		if deletedAt.Valid {
			if t, err := time.Parse(time.RFC3339, deletedAt.String); err == nil {
				node.DeletedAt = t
			}
		}
		if changeNote.Valid {
			node.ChangeNote = changeNote.String
		}
		if changedBy.Valid {
			node.ChangedBy = changedBy.String
		}

		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (r *SQLiteRepository) scanLink(rows *sql.Rows) (*core.Link, error) {
	var sourceID, targetID, linkType string
	var properties, createdAt, modifiedAt string

	if err := rows.Scan(&sourceID, &targetID, &linkType, &properties, &createdAt, &modifiedAt); err != nil {
		return nil, err
	}

	link := &core.Link{
		Source: sourceID,
		Target: targetID,
		Type:   linkType,
	}

	if properties != "" {
		json.Unmarshal([]byte(properties), &link.Meta)
	}
	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		link.Created = t
	}
	if t, err := time.Parse(time.RFC3339, modifiedAt); err == nil {
		link.Modified = t
	}

	return link, nil
}

func (r *SQLiteRepository) updateDegree(ctx context.Context, nodeID string, delta int) {
	r.db.ExecContext(ctx, `
		UPDATE nodes SET degree = degree + ? WHERE id = ? AND is_current = 1
	`, delta, nodeID)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
