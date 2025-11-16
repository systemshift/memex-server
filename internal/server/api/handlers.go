package api

import (
	"encoding/json"
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

	// Create a Source node
	// TODO: Use content hash as ID instead of generated ID
	now := time.Now()
	sourceID := "source-" + time.Now().Format("20060102150405")

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

	resp := IngestResponse{
		SourceID: node.ID,
		Created:  node.Created,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
