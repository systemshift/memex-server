package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"github.com/systemshift/memex/internal/memex/storage"
)

type Server struct {
	memex    *storage.MXStore
	template *template.Template
}

type GraphData struct {
	Nodes []NodeData `json:"nodes"`
	Edges []EdgeData `json:"edges"`
}

type NodeData struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Meta    map[string]any `json:"meta"`
	Created string         `json:"created"`
}

type EdgeData struct {
	Source string         `json:"source"`
	Target string         `json:"target"`
	Type   string         `json:"type"`
	Meta   map[string]any `json:"meta"`
}

func main() {
	addr := flag.String("addr", ":3000", "HTTP service address")
	repo := flag.String("repo", "", "Repository path")
	flag.Parse()

	if *repo == "" {
		log.Fatal("Repository path required")
	}

	// Open repository
	store, err := storage.OpenMX(*repo)
	if err != nil {
		log.Fatalf("Error opening repository: %v", err)
	}
	defer store.Close()

	// Parse templates
	tmpl, err := template.ParseGlob("cmd/memexd/templates/*.html")
	if err != nil {
		log.Fatalf("Error parsing templates: %v", err)
	}

	// Create server
	server := &Server{
		memex:    store,
		template: tmpl,
	}

	// Setup routes
	http.HandleFunc("/", server.handleIndex)
	http.HandleFunc("/api/graph", server.handleGraph)
	http.HandleFunc("/api/content/", server.handleContent)
	http.HandleFunc("/node/", server.handleNode)

	// Serve static files
	fs := http.FileServer(http.Dir("cmd/memexd/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Start server
	log.Printf("Starting server on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if err := s.template.ExecuteTemplate(w, "index.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	// Get all nodes
	var graph GraphData
	for _, entry := range s.memex.Nodes() {
		node, err := s.memex.GetNode(fmt.Sprintf("%x", entry.ID[:]))
		if err != nil {
			continue
		}

		// Add node
		graph.Nodes = append(graph.Nodes, NodeData{
			ID:      node.ID,
			Type:    node.Type,
			Meta:    node.Meta,
			Created: node.Created.Format("2006-01-02 15:04:05"),
		})

		// Get links
		links, err := s.memex.GetLinks(node.ID)
		if err != nil {
			continue
		}

		// Add edges
		for _, link := range links {
			graph.Edges = append(graph.Edges, EdgeData{
				Source: node.ID,
				Target: link.Target,
				Type:   link.Type,
				Meta:   link.Meta,
			})
		}
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(graph); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleContent(w http.ResponseWriter, r *http.Request) {
	// Get content hash from URL
	hash := filepath.Base(r.URL.Path)

	// Reconstruct content from chunks
	content, err := s.memex.ReconstructContent(hash)
	if err != nil {
		http.Error(w, "Content not found", http.StatusNotFound)
		return
	}

	// Write response
	w.Header().Set("Content-Type", "text/plain")
	w.Write(content)
}

func (s *Server) handleNode(w http.ResponseWriter, r *http.Request) {
	// Get node ID from URL
	id := filepath.Base(r.URL.Path)

	// Get node
	node, err := s.memex.GetNode(id)
	if err != nil {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	// If file, serve content
	if node.Type == "file" {
		if contentHash, ok := node.Meta["content"].(string); ok {
			content, err := s.memex.ReconstructContent(contentHash)
			if err != nil {
				http.Error(w, "Content not found", http.StatusNotFound)
				return
			}

			// Set filename for download
			if filename, ok := node.Meta["filename"].(string); ok {
				w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
			}

			w.Write(content)
			return
		}
	}

	// Otherwise show node info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}
