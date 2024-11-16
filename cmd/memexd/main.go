package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"memex/internal/memex"
	"memex/internal/memex/core"
	"memex/internal/memex/storage"
)

//go:embed static/* templates/*
var content embed.FS

// Server handles HTTP requests
type Server struct {
	repo      *storage.Repository
	templates *template.Template
}

// NewServer creates a new server instance
func NewServer() (*Server, error) {
	// Initialize repository
	repo, err := memex.GetRepository()
	if err != nil {
		return nil, fmt.Errorf("initializing repository: %w", err)
	}

	// Load templates
	tmpl, err := template.ParseFS(content, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}

	return &Server{
		repo:      repo,
		templates: tmpl,
	}, nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get all objects
	objects := s.repo.List()
	var items []core.Object
	for _, id := range objects {
		obj, err := s.repo.Get(id)
		if err != nil {
			continue
		}
		items = append(items, obj)
	}

	data := map[string]interface{}{
		"Objects": items,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file content
	content := make([]byte, header.Size)
	if _, err := file.Read(content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Add to repository
	meta := map[string]any{
		"filename": header.Filename,
		"type":     "file",
	}

	if _, err := s.repo.Add(content, "file", meta); err != nil {
		log.Printf("Error adding file: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect back to index
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	source := r.Form.Get("source")
	target := r.Form.Get("target")
	linkType := r.Form.Get("type")
	note := r.Form.Get("note")

	meta := map[string]any{}
	if note != "" {
		meta["note"] = note
	}

	if err := s.repo.Link(source, target, linkType, meta); err != nil {
		log.Printf("Error creating link: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := make(map[string]any)
	for k, v := range r.URL.Query() {
		if k != "" && len(v) > 0 {
			query[k] = strings.Join(v, " ")
		}
	}

	results := s.repo.Search(query)
	data := map[string]interface{}{
		"Objects": results,
		"Query":   r.URL.Query().Get("q"),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func main() {
	// Parse flags
	port := flag.Int("port", 8080, "Port to listen on")
	flag.Parse()

	// Create server
	server, err := NewServer()
	if err != nil {
		log.Fatalf("Error creating server: %v", err)
	}

	// Create static file server
	static, err := fs.Sub(content, "static")
	if err != nil {
		log.Fatalf("Error setting up static files: %v", err)
	}
	staticHandler := http.FileServer(http.FS(static))

	// Set up routes
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", staticHandler))
	mux.HandleFunc("/", server.handleIndex)
	mux.HandleFunc("/add", server.handleAdd)
	mux.HandleFunc("/link", server.handleLink)
	mux.HandleFunc("/search", server.handleSearch)

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting server on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
