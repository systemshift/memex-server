package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/systemshift/memex/internal/server/api"
	"github.com/systemshift/memex/internal/server/graph"
)

func main() {
	// Load configuration from environment
	neo4jURI := getEnv("NEO4J_URI", "bolt://localhost:7687")
	neo4jUser := getEnv("NEO4J_USER", "neo4j")
	neo4jPassword := getEnv("NEO4J_PASSWORD", "password")
	port := getEnv("PORT", "8080")

	// Initialize Neo4j repository
	ctx := context.Background()
	repo, err := graph.New(ctx, graph.Config{
		URI:      neo4jURI,
		Username: neo4jUser,
		Password: neo4jPassword,
		Database: "neo4j",
	})
	if err != nil {
		log.Fatalf("Failed to connect to Neo4j: %v", err)
	}
	defer repo.Close(ctx)

	log.Println("Connected to Neo4j successfully")

	// Initialize API server
	apiServer := api.New(repo)

	// Setup HTTP router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	// Routes
	r.Get("/health", apiServer.HealthCheck)

	r.Route("/api", func(r chi.Router) {
		r.Post("/ingest", apiServer.Ingest)
		r.Post("/nodes", apiServer.CreateNode)
		r.Get("/nodes", apiServer.ListNodes)
		r.Get("/nodes/{id}", apiServer.GetNode)
		r.Get("/nodes/{id}/links", apiServer.GetLinks)
		r.Post("/links", apiServer.CreateLink)
	})

	// HTTP server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting memex server on http://localhost:%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
