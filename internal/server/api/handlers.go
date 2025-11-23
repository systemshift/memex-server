package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/systemshift/memex/internal/memex/core"
	"github.com/systemshift/memex/internal/server/graph"
)

// Server holds the HTTP server dependencies
type Server struct {
	repo *graph.Repository
}

// New creates a new API server
func New(repo *graph.Repository) *Server {
	return &Server{repo: repo}
}

// CreateNodeRequest is the request body for creating a node
type CreateNodeRequest struct {
	ID   string                 `json:"id"`
	Type string                 `json:"type"`
	Meta map[string]interface{} `json:"meta"`
}

// CreateNodeResponse is the response for creating a node
type CreateNodeResponse struct {
	ID      string    `json:"id"`
	Created time.Time `json:"created"`
}

// CreateNode handles POST /api/nodes
func (s *Server) CreateNode(w http.ResponseWriter, r *http.Request) {
	var req CreateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	now := time.Now()
	node := &core.Node{
		ID:       req.ID,
		Type:     req.Type,
		Meta:     req.Meta,
		Created:  now,
		Modified: now,
	}

	if err := s.repo.CreateNode(r.Context(), node); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := CreateNodeResponse{
		ID:      node.ID,
		Created: node.Created,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetNode handles GET /api/nodes/{id}
func (s *Server) GetNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	node, err := s.repo.GetNode(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

// CreateLinkRequest is the request body for creating a link
type CreateLinkRequest struct {
	Source string                 `json:"source"`
	Target string                 `json:"target"`
	Type   string                 `json:"type"`
	Meta   map[string]interface{} `json:"meta"`
}

// CreateLink handles POST /api/links
func (s *Server) CreateLink(w http.ResponseWriter, r *http.Request) {
	var req CreateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	now := time.Now()
	link := &core.Link{
		Source:   req.Source,
		Target:   req.Target,
		Type:     req.Type,
		Meta:     req.Meta,
		Created:  now,
		Modified: now,
	}

	if err := s.repo.CreateLink(r.Context(), link); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(link)
}

// GetLinks handles GET /api/nodes/{id}/links
func (s *Server) GetLinks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	links, err := s.repo.GetLinks(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(links)
}

// ListNodes handles GET /api/nodes
func (s *Server) ListNodes(w http.ResponseWriter, r *http.Request) {
	ids, err := s.repo.ListNodes(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": ids,
		"count": len(ids),
	})
}

// HealthCheck handles GET /health
func (s *Server) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// IngestRequest is the request body for ingesting content
type IngestRequest struct {
	Content string `json:"content"`
	Format  string `json:"format,omitempty"` // e.g. "text", "git-log", "json"
}

// IngestResponse is the response for ingesting content
type IngestResponse struct {
	SourceID string    `json:"source_id"`
	Created  time.Time `json:"created"`
}

// Ingest handles POST /api/ingest
func (s *Server) Ingest(w http.ResponseWriter, r *http.Request) {
	var req IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	// Compute SHA256 hash of content
	hash := sha256.Sum256([]byte(req.Content))
	hashStr := hex.EncodeToString(hash[:])
	sourceID := "sha256:" + hashStr

	now := time.Now()

	// Check if source already exists (dedup)
	existing, err := s.repo.GetNode(r.Context(), sourceID)
	if err == nil && existing != nil {
		// Source already exists, return existing ID
		resp := IngestResponse{
			SourceID: existing.ID,
			Created:  existing.Created,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Create new Source node
	node := &core.Node{
		ID:       sourceID,
		Type:     "Source",
		Content:  []byte(req.Content),
		Meta: map[string]interface{}{
			"format":      req.Format,
			"ingested_at": now.Format(time.RFC3339),
			"size_bytes":  len(req.Content),
		},
		Created:  now,
		Modified: now,
	}

	if err := s.repo.CreateNode(r.Context(), node); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Record transaction
	if err := s.recordTransaction(r.Context(), "ingest_source", map[string]interface{}{
		"source_id": sourceID,
		"format":    req.Format,
		"size":      len(req.Content),
	}); err != nil {
		// Log but don't fail the request
		// Transaction recording is for audit, not critical path
	}

	resp := IngestResponse{
		SourceID: node.ID,
		Created:  node.Created,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// recordTransaction creates a transaction record node
func (s *Server) recordTransaction(ctx context.Context, operation string, details map[string]interface{}) error {
	now := time.Now()
	txID := "tx-" + now.Format("20060102150405.000000")

	txNode := &core.Node{
		ID:   txID,
		Type: "Transaction",
		Meta: map[string]interface{}{
			"operation": operation,
			"details":   details,
			"timestamp": now.Format(time.RFC3339Nano),
		},
		Created:  now,
		Modified: now,
	}

	return s.repo.CreateNode(ctx, txNode)
}

// parsePagination extracts limit and offset from query parameters
func parsePagination(r *http.Request) (limit int, offset int) {
	limit = 100 // default limit
	offset = 0

	query := r.URL.Query()
	if l := query.Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if o := query.Get("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}

	return limit, offset
}

// QueryFilter handles GET /api/query/filter
func (s *Server) QueryFilter(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()
	types := query["type"]           // can have multiple: ?type=Person&type=Concept
	propertyKey := query.Get("key")  // e.g., ?key=extractor
	propertyValue := query.Get("value") // e.g., ?value=openai
	limit, offset := parsePagination(r)

	nodes, err := s.repo.FilterNodes(r.Context(), types, propertyKey, propertyValue, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodes,
		"count": len(nodes),
	})
}

// QuerySearch handles GET /api/query/search
func (s *Server) QuerySearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "query parameter 'q' is required", http.StatusBadRequest)
		return
	}
	limit, offset := parsePagination(r)

	nodes, err := s.repo.SearchNodes(r.Context(), q, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodes,
		"count": len(nodes),
		"query": q,
	})
}

// DeleteNode handles DELETE /api/nodes/{id}
func (s *Server) DeleteNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Check query parameter for force delete (bypasses Source layer protection)
	force := r.URL.Query().Get("force") == "true"

	if err := s.repo.DeleteNode(r.Context(), id, force); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tombstoned": true,
		"id":         id,
		"force":      force,
		"message":    "Node marked as deleted (tombstone). Maintains DAG integrity.",
	})
}

// DeleteLink handles DELETE /api/links
func (s *Server) DeleteLink(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	source := query.Get("source")
	target := query.Get("target")
	linkType := query.Get("type")

	if source == "" || target == "" || linkType == "" {
		http.Error(w, "source, target, and type query parameters required", http.StatusBadRequest)
		return
	}

	if err := s.repo.DeleteLink(r.Context(), source, target, linkType); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deleted": map[string]string{
			"source": source,
			"target": target,
			"type":   linkType,
		},
	})
}

// QueryTraverse handles GET /api/query/traverse
func (s *Server) QueryTraverse(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	startNodeID := query.Get("start")
	if startNodeID == "" {
		http.Error(w, "query parameter 'start' is required", http.StatusBadRequest)
		return
	}

	// Default depth is 2
	depth := 2
	if d := query.Get("depth"); d != "" {
		var err error
		if _, err = fmt.Sscanf(d, "%d", &depth); err != nil {
			http.Error(w, "invalid depth parameter", http.StatusBadRequest)
			return
		}
	}

	// Optional relationship type filters
	relationshipTypes := query["rel_type"]
	limit, offset := parsePagination(r)

	nodes, err := s.repo.TraverseGraph(r.Context(), startNodeID, depth, relationshipTypes, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodes,
		"count": len(nodes),
		"start": startNodeID,
		"depth": depth,
	})
}

// QuerySubgraph handles GET /api/query/subgraph
// Returns nodes + ALL edges within a k-hop neighborhood
func (s *Server) QuerySubgraph(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	startNodeID := query.Get("start")
	if startNodeID == "" {
		http.Error(w, "query parameter 'start' is required", http.StatusBadRequest)
		return
	}

	// Default depth is 2
	depth := 2
	if d := query.Get("depth"); d != "" {
		var err error
		if _, err = fmt.Sscanf(d, "%d", &depth); err != nil {
			http.Error(w, "invalid depth parameter", http.StatusBadRequest)
			return
		}
	}

	// Optional relationship type filters
	relationshipTypes := query["rel_type"]

	subgraph, err := s.repo.GetSubgraph(r.Context(), startNodeID, depth, relationshipTypes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subgraph)
}

// UpdateAttentionEdgeRequest is the request body for updating attention edges
type UpdateAttentionEdgeRequest struct {
	Source   string  `json:"source"`
	Target   string  `json:"target"`
	QueryID  string  `json:"query_id"`
	Weight   float64 `json:"weight"`
}

// UpdateAttentionEdge handles POST /api/edges/attention
// Allows ML pipeline to persist attention patterns to the DAG
func (s *Server) UpdateAttentionEdge(w http.ResponseWriter, r *http.Request) {
	var req UpdateAttentionEdgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Source == "" || req.Target == "" {
		http.Error(w, "source and target are required", http.StatusBadRequest)
		return
	}

	if req.Weight < 0 || req.Weight > 1 {
		http.Error(w, "weight must be between 0 and 1", http.StatusBadRequest)
		return
	}

	if err := s.repo.UpdateAttentionEdge(r.Context(), req.Source, req.Target, req.QueryID, req.Weight); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Attention edge updated",
		"source":  req.Source,
		"target":  req.Target,
		"weight":  req.Weight,
	})
}

// QueryAttentionSubgraph handles GET /api/query/attention_subgraph
// Returns subgraph following high-weight attention edges
func (s *Server) QueryAttentionSubgraph(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	startNodeID := query.Get("start")
	if startNodeID == "" {
		http.Error(w, "query parameter 'start' is required", http.StatusBadRequest)
		return
	}

	// Default min weight is 0.5
	minWeight := 0.5
	if mw := query.Get("min_weight"); mw != "" {
		var err error
		if _, err = fmt.Sscanf(mw, "%f", &minWeight); err != nil {
			http.Error(w, "invalid min_weight parameter", http.StatusBadRequest)
			return
		}
	}

	// Default max nodes is 50
	maxNodes := 50
	if mn := query.Get("max_nodes"); mn != "" {
		var err error
		if _, err = fmt.Sscanf(mn, "%d", &maxNodes); err != nil {
			http.Error(w, "invalid max_nodes parameter", http.StatusBadRequest)
			return
		}
	}

	subgraph, err := s.repo.GetAttentionSubgraph(r.Context(), startNodeID, minWeight, maxNodes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subgraph)
}

// PruneAttentionEdges handles POST /api/edges/attention/prune
// Removes weak attention edges to maintain DAG quality
func (s *Server) PruneAttentionEdges(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Default min weight is 0.3
	minWeight := 0.3
	if mw := query.Get("min_weight"); mw != "" {
		var err error
		if _, err = fmt.Sscanf(mw, "%f", &minWeight); err != nil {
			http.Error(w, "invalid min_weight parameter", http.StatusBadRequest)
			return
		}
	}

	// Default min query count is 2
	minQueryCount := 2
	if mc := query.Get("min_query_count"); mc != "" {
		var err error
		if _, err = fmt.Sscanf(mc, "%d", &minQueryCount); err != nil {
			http.Error(w, "invalid min_query_count parameter", http.StatusBadRequest)
			return
		}
	}

	deletedCount, err := s.repo.PruneWeakAttentionEdges(r.Context(), minWeight, minQueryCount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"deleted_count": deletedCount,
		"message":       fmt.Sprintf("Pruned %d weak attention edges", deletedCount),
	})
}
