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
	objects := s.memex.List()
	var items []memex.Object
	for _, id := range objects {
		obj, err := s.memex.Get(id)
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

	// Add to memex
	meta := map[string]any{
		"filename": header.Filename,
		"type":     "file",
	}

	if _, err := s.memex.Add(content, "file", meta); err != nil {
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
	if err := s.memex.Delete(id); err != nil {
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

	if err := s.memex.Link(source, target, linkType, meta); err != nil {
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

	results := s.memex.Search(query)
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
	dir := flag.String("dir", ".", "Directory to store data")
	flag.Parse()

	// Create server
	server, err := NewServer(*dir)
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
	mux.HandleFunc("/delete", server.handleDelete)
	mux.HandleFunc("/link", server.handleLink)
	mux.HandleFunc("/search", server.handleSearch)

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting server on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
