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

	"memex/internal/memex/core"
	"memex/pkg/memex"
)

//go:embed static/* templates/*
var content embed.FS

// Server handles HTTP requests
type Server struct {
	memex     *memex.Memex
	templates *template.Template
}

// NewServer creates a new server instance
func NewServer(path string) (*Server, error) {
	// Initialize memex
	mx, err := memex.Open(path)
	if err != nil {
		return nil, fmt.Errorf("initializing memex: %w", err)
	}

	// Load templates
	tmpl, err := template.ParseFS(content, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}

	return &Server{
		memex:     mx,
		templates: tmpl,
	}, nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get all objects
	repo, err := s.memex.GetRepository()
	if err != nil {
		log.Printf("Error getting repository: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	root, err := repo.GetRoot()
	if err != nil {
		log.Printf("Error getting root: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []map[string]interface{}
	for _, id := range root.Nodes {
		node, err := repo.GetNode(id)
		if err != nil {
			continue
		}

		// Get links for node
		links, err := repo.GetLinks(id)
		if err != nil {
			links = []core.Link{}
		}

		item := map[string]interface{}{
			"ID":       node.ID,
			"Type":     node.Type,
			"Meta":     node.Meta,
			"Created":  node.Created,
			"Modified": node.Modified,
			"Links":    links,
		}
		items = append(items, item)
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

	// Add to memex
	meta := map[string]any{
		"filename": header.Filename,
		"type":     "file",
	}

	repo, err := s.memex.GetRepository()
	if err != nil {
		log.Printf("Error getting repository: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := repo.AddNode(content, "file", meta); err != nil {
		log.Printf("Error adding file: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect back to index
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id := r.Form.Get("id")
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	repo, err := s.memex.GetRepository()
	if err != nil {
		log.Printf("Error getting repository: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Delete the object
	if err := repo.DeleteNode(id); err != nil {
		log.Printf("Error deleting object: %v", err)
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

	repo, err := s.memex.GetRepository()
	if err != nil {
		log.Printf("Error getting repository: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := repo.AddLink(source, target, linkType, meta); err != nil {
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

	repo, err := s.memex.GetRepository()
	if err != nil {
		log.Printf("Error getting repository: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	results, err := repo.Search(query)
	if err != nil {
		log.Printf("Error searching: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []map[string]interface{}
	for _, node := range results {
		// Get links for node
		links, err := repo.GetLinks(node.ID)
		if err != nil {
			links = []core.Link{}
		}

		item := map[string]interface{}{
			"ID":       node.ID,
			"Type":     node.Type,
			"Meta":     node.Meta,
			"Created":  node.Created,
			"Modified": node.Modified,
			"Links":    links,
		}
		items = append(items, item)
	}

	data := map[string]interface{}{
		"Objects": items,
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
	addr := flag.String("addr", ":3000", "HTTP service address")
	path := flag.String("path", "", "Path to memex repository")
	flag.Parse()

	if *path == "" {
		log.Fatal("Path required")
	}

	// Create server
	server, err := NewServer(*path)
	if err != nil {
		log.Fatal(err)
	}

	// Setup routes
	http.HandleFunc("/", server.handleIndex)
	http.HandleFunc("/add", server.handleAdd)
	http.HandleFunc("/delete", server.handleDelete)
	http.HandleFunc("/link", server.handleLink)
	http.HandleFunc("/search", server.handleSearch)

	// Serve static files
	staticFS, err := fs.Sub(content, "static")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Start server
	log.Printf("Starting server on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
