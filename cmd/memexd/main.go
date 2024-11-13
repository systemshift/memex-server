package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"memex/internal/memex"
	"memex/web/handlers"
)

func main() {
	// Command line flags
	port := flag.Int("port", 8080, "Port to listen on")
	flag.Parse()

	// Initialize repository
	repo, err := memex.GetRepository()
	if err != nil {
		log.Fatalf("Error initializing repository: %v", err)
	}

	// Create handler
	handler, err := handlers.New(repo)
	if err != nil {
		log.Fatalf("Error creating handler: %v", err)
	}

	// Static file server
	staticHandler, err := handlers.ServeStatic()
	if err != nil {
		log.Fatalf("Error setting up static file server: %v", err)
	}
	http.Handle("/static/", http.StripPrefix("/static/", staticHandler))

	// Routes
	http.HandleFunc("/", handler.HandleIndex)
	http.HandleFunc("/add", handler.HandleAdd)
	http.HandleFunc("/show/", handler.HandleShow)
	http.HandleFunc("/link", handler.HandleLink)
	http.HandleFunc("/search", handler.HandleSearch)
	http.HandleFunc("/api/add", handler.HandleAPIAdd)
	http.HandleFunc("/api/link", handler.HandleAPILink)
	http.HandleFunc("/api/search", handler.HandleAPISearch)

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting server on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
