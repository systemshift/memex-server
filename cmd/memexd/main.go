package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/systemshift/memex/internal/memex"
	"github.com/systemshift/memex/internal/memex/core"
	"github.com/systemshift/memex/internal/memex/repository"
)

// VersionResponse represents version information
type VersionResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
}

// Server handles HTTP requests and manages the repository
type Server struct {
	repo     core.Repository
	template *template.Template
}

// GraphResponse represents the graph visualization data
type GraphResponse struct {
	Nodes []NodeResponse `json:"nodes"`
	Links []LinkResponse `json:"links"`
}

// NodeResponse represents a node in the graph visualization
type NodeResponse struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Meta     map[string]interface{} `json:"meta"`
	Created  string                 `json:"created"`
	Modified string                 `json:"modified"`
}

// LinkResponse represents a link in the graph visualization
type LinkResponse struct {
	Source   string                 `json:"source"`
	Target   string                 `json:"target"`
	Type     string                 `json:"type"`
	Meta     map[string]interface{} `json:"meta"`
	Created  string                 `json:"created"`
	Modified string                 `json:"modified"`
}

func main() {
	// Parse command line flags
	addr := flag.String("addr", ":3000", "HTTP service address")
	repoPath := flag.String("repo", "", "Repository path")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Println(memex.BuildInfo())
		os.Exit(0)
	}

	if *repoPath == "" {
		log.Fatal("Repository path required")
	}

	// Initialize repository
	repo, err := repository.Open(*repoPath)
	if err != nil {
		log.Fatalf("Error opening repository: %v", err)
	}
	defer repo.Close()

	// Parse templates
	tmpl, err := template.ParseGlob("cmd/memexd/templates/*.html")
	if err != nil {
		log.Fatalf("Error parsing templates: %v", err)
	}

	// Create server
	server := &Server{
		repo:     repo,
		template: tmpl,
	}

	// Create router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Routes
	r.Get("/", server.handleIndex)
	r.Route("/api", func(r chi.Router) {
		r.Get("/version", server.handleVersion)
		r.Get("/graph", server.handleGraph)
		r.Get("/nodes/{id}", server.handleGetNode)
		r.Get("/nodes/{id}/content", server.handleGetContent)
	})

	// Static files
	workDir, _ := os.Getwd()
	filesDir := http.Dir(filepath.Join(workDir, "cmd/memexd/static"))
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(filesDir)))

	// Start server
	log.Printf("Starting server on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, r))
}

// handleIndex serves the main page
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if err := s.template.ExecuteTemplate(w, "index.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// handleGraph returns the graph data for visualization
func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	// Get all nodes
	nodeIDs, err := s.repo.ListNodes()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error listing nodes: %v", err), http.StatusInternalServerError)
		return
	}

	response := GraphResponse{
		Nodes: make([]NodeResponse, 0, len(nodeIDs)),
		Links: make([]LinkResponse, 0),
	}

	// Process each node
	for _, id := range nodeIDs {
		node, err := s.repo.GetNode(id)
		if err != nil {
			log.Printf("Error getting node %s: %v", id, err)
			continue
		}

		// Add node to response
		response.Nodes = append(response.Nodes, NodeResponse{
			ID:       node.ID,
			Type:     node.Type,
			Meta:     node.Meta,
			Created:  node.Created.Format("2006-01-02 15:04:05"),
			Modified: node.Modified.Format("2006-01-02 15:04:05"),
		})

		// Get and process links
		links, err := s.repo.GetLinks(node.ID)
		if err != nil {
			log.Printf("Error getting links for node %s: %v", id, err)
			continue
		}

		for _, link := range links {
			response.Links = append(response.Links, LinkResponse{
				Source:   link.Source,
				Target:   link.Target,
				Type:     link.Type,
				Meta:     link.Meta,
				Created:  link.Created.Format("2006-01-02 15:04:05"),
				Modified: link.Modified.Format("2006-01-02 15:04:05"),
			})
		}
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("Error encoding response: %v", err), http.StatusInternalServerError)
		return
	}
}

// handleGetNode returns information about a specific node
func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Node ID required", http.StatusBadRequest)
		return
	}

	node, err := s.repo.GetNode(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting node: %v", err), http.StatusNotFound)
		return
	}

	// Create response with formatted timestamps
	response := struct {
		ID       string                 `json:"id"`
		Type     string                 `json:"type"`
		Content  string                 `json:"content"`
		Meta     map[string]interface{} `json:"meta"`
		Created  string                 `json:"created"`
		Modified string                 `json:"modified"`
	}{
		ID:       node.ID,
		Type:     node.Type,
		Content:  string(node.Content),
		Meta:     node.Meta,
		Created:  node.Created.Format("2006-01-02 15:04:05"),
		Modified: node.Modified.Format("2006-01-02 15:04:05"),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("Error encoding response: %v", err), http.StatusInternalServerError)
		return
	}
}

// handleGetContent returns the content of a node
func (s *Server) handleGetContent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Node ID required", http.StatusBadRequest)
		return
	}

	// Get node first to check type and get metadata
	node, err := s.repo.GetNode(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting node: %v", err), http.StatusNotFound)
		return
	}

	// Get content
	content, err := s.repo.GetContent(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting content: %v", err), http.StatusNotFound)
		return
	}

	// Set content type if available in metadata
	if contentType, ok := node.Meta["content-type"].(string); ok {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	// Set filename for download if available
	if filename, ok := node.Meta["filename"].(string); ok {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	}

	w.Write(content)
}

// handleVersion returns version information
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	info := strings.Split(memex.BuildInfo(), "\n")
	version := strings.TrimPrefix(info[0], "Version: ")
	commit := strings.TrimPrefix(info[1], "Commit: ")
	date := strings.TrimPrefix(info[2], "Build Date: ")

	response := VersionResponse{
		Version:   version,
		Commit:    commit,
		BuildDate: date,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("Error encoding response: %v", err), http.StatusInternalServerError)
		return
	}
}
