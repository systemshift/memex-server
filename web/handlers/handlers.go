package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"memex/internal/memex/core"
	"memex/internal/memex/storage"
)

// Handler manages HTTP request handling
type Handler struct {
	repo      *storage.Repository
	templates *template.Template
}

// New creates a new Handler instance
func New(repo *storage.Repository) (*Handler, error) {
	// Find template directory
	templateDirs := []string{
		"web/templates",                        // Local development
		"/usr/local/share/memex/web/templates", // System install
		filepath.Join(os.Getenv("HOME"), ".local/share/memex/web/templates"), // User install
	}

	var templatePath string
	for _, dir := range templateDirs {
		if _, err := os.Stat(dir); err == nil {
			templatePath = dir
			break
		}
	}

	if templatePath == "" {
		return nil, fmt.Errorf("template directory not found")
	}

	// Load templates
	tmpl, err := template.ParseGlob(filepath.Join(templatePath, "*.html"))
	if err != nil {
		return nil, err
	}

	return &Handler{
		repo:      repo,
		templates: tmpl,
	}, nil
}

// ServeStatic returns a handler for serving static files
func ServeStatic() (http.Handler, error) {
	// Find static directory
	staticDirs := []string{
		"web/static",                        // Local development
		"/usr/local/share/memex/web/static", // System install
		filepath.Join(os.Getenv("HOME"), ".local/share/memex/web/static"), // User install
	}

	var staticPath string
	for _, dir := range staticDirs {
		if _, err := os.Stat(dir); err == nil {
			staticPath = dir
			break
		}
	}

	if staticPath == "" {
		return nil, fmt.Errorf("static directory not found")
	}

	return http.FileServer(http.Dir(staticPath)), nil
}

// HandleIndex handles the home page
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get all notes
	notes := h.repo.FindByType("note")

	data := map[string]interface{}{
		"Title": "Home",
		"Notes": notes,
	}

	h.templates.ExecuteTemplate(w, "layout.html", data)
}

// HandleAdd handles file addition
func (h *Handler) HandleAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		data := map[string]interface{}{
			"Title": "Add Content",
		}
		h.templates.ExecuteTemplate(w, "add.html", data)
		return
	}

	// Handle POST
	r.ParseMultipartForm(10 << 20) // 10 MB max
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error uploading file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file content
	content := make([]byte, header.Size)
	file.Read(content)

	// Add to repository
	meta := map[string]any{
		"filename": header.Filename,
		"type":     "file",
	}

	_, err = h.repo.Add(content, "file", meta)
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// HandleShow displays object details
func (h *Handler) HandleShow(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/show/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	obj, err := h.repo.Get(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Get links
	links, err := h.repo.GetLinks(id)
	if err != nil {
		links = []core.Link{}
	}

	data := map[string]interface{}{
		"Title":  fmt.Sprintf("Object %s", id[:8]),
		"Object": obj,
		"Links":  links,
	}

	h.templates.ExecuteTemplate(w, "show.html", data)
}

// HandleLink handles creating links between objects
func (h *Handler) HandleLink(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// Get all objects for selection
		objects := h.repo.List()
		data := map[string]interface{}{
			"Title":   "Create Link",
			"Objects": objects,
		}
		h.templates.ExecuteTemplate(w, "link.html", data)
		return
	}

	// Handle POST
	r.ParseForm()
	source := r.Form.Get("source")
	target := r.Form.Get("target")
	linkType := r.Form.Get("type")
	note := r.Form.Get("note")

	meta := map[string]any{}
	if note != "" {
		meta["note"] = note
	}

	err := h.repo.Link(source, target, linkType, meta)
	if err != nil {
		http.Error(w, "Error creating link", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/show/%s", source), http.StatusSeeOther)
}

// HandleSearch handles search requests
func (h *Handler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		data := map[string]interface{}{
			"Title": "Search",
		}
		h.templates.ExecuteTemplate(w, "search.html", data)
		return
	}

	// Handle POST
	r.ParseForm()
	query := make(map[string]any)
	for k, v := range r.Form {
		if k != "" && len(v) > 0 {
			query[k] = v[0]
		}
	}

	results := h.repo.Search(query)
	data := map[string]interface{}{
		"Title":   "Search Results",
		"Results": results,
	}

	h.templates.ExecuteTemplate(w, "search.html", data)
}

// HandleAPIAdd handles file upload via API
func (h *Handler) HandleAPIAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.ParseMultipartForm(10 << 20) // 10 MB max
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error uploading file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file content
	content := make([]byte, header.Size)
	file.Read(content)

	// Add to repository
	meta := map[string]any{
		"filename": header.Filename,
		"type":     "file",
	}

	id, err := h.repo.Add(content, "file", meta)
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id": id,
	})
}

// HandleAPILink handles creating links via API
func (h *Handler) HandleAPILink(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Source string         `json:"source"`
		Target string         `json:"target"`
		Type   string         `json:"type"`
		Meta   map[string]any `json:"meta"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	err := h.repo.Link(req.Source, req.Target, req.Type, req.Meta)
	if err != nil {
		http.Error(w, "Error creating link", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// HandleAPISearch handles search via API
func (h *Handler) HandleAPISearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var query map[string]any
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		http.Error(w, "Invalid query", http.StatusBadRequest)
		return
	}

	results := h.repo.Search(query)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
