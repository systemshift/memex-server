package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"

	"memex/pkg/memex"
)

// Handler manages HTTP request handling
type Handler struct {
	repo      *memex.Repository
	templates *template.Template
}

// New creates a new Handler instance
func New(repo *memex.Repository) (*Handler, error) {
	// Load templates
	tmpl, err := template.ParseGlob(filepath.Join("web", "templates", "*.html"))
	if err != nil {
		return nil, err
	}

	return &Handler{
		repo:      repo,
		templates: tmpl,
	}, nil
}

// HandleIndex handles the home page
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement index page
}

// HandleAdd handles file addition
func (h *Handler) HandleAdd(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement file addition
}

// HandleCommit handles commit creation
func (h *Handler) HandleCommit(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement commit creation
}

// HandleHistory handles viewing commit history
func (h *Handler) HandleHistory(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement history viewing
}
