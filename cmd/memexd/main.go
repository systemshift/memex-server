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

	"memex/internal/memex/storage"
)

//go:embed static/* templates/*
var content embed.FS

// Server handles HTTP requests
type Server struct {
	memex     *storage.MXStore
	templates *template.Template
}

// NewServer creates a new server instance
func NewServer(path string) (*Server, error) {
	// Initialize memex
	mx, err := storage.OpenMX(path)
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

	// Get all nodes
	var items []map[string]interface{}
	for _, entry := range s.memex.Nodes() {
		node, err := s.memex.GetNode(fmt.Sprintf("%x", entry.ID[:]))
		if err != nil {
			continue
		}

		// Get links for node
		links, err := s.memex.GetLinks(fmt.Sprintf("%x", entry.ID[:]))
		if err != nil {
			links = []storage.Link{}
		}

		item := map[string]interface{}{
			"ID":       fmt.Sprintf("%x", entry.ID[:]),
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

	if _, err := s.memex.AddNode(content, "file", meta); err != nil {
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

	// Delete the object
	if err := s.memex.DeleteNode(id); err != nil {
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

	if err := s.memex.AddLink(source, target, linkType, meta); err != nil {
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

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Get all nodes and filter
	var items []map[string]interface{}
	for _, entry := range s.memex.Nodes() {
		node, err := s.memex.GetNode(fmt.Sprintf("%x", entry.ID[:]))
		if err != nil {
			continue
		}

		// Simple text search in metadata
		matched := false
		for _, v := range node.Meta {
			if str, ok := v.(string); ok {
				if strings.Contains(strings.ToLower(str), strings.ToLower(query)) {
					matched = true
					break
				}
			}
		}

		if !matched {
			continue
		}

		// Get links for node
		links, err := s.memex.GetLinks(fmt.Sprintf("%x", entry.ID[:]))
		if err != nil {
			links = []storage.Link{}
		}

		item := map[string]interface{}{
			"ID":       fmt.Sprintf("%x", entry.ID[:]),
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
		"Query":   query,
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
