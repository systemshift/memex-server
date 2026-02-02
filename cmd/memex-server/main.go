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
	"github.com/systemshift/memex/internal/server/subscriptions"
)

func main() {
	// Load configuration from environment
	backend := getEnv("MEMEX_BACKEND", "sqlite")
	port := getEnv("PORT", "8080")

	ctx := context.Background()
	var repo graph.Repository
	var err error

	switch backend {
	case "sqlite":
		sqlitePath := getEnv("SQLITE_PATH", "./memex.db")
		log.Printf("Using SQLite backend: %s", sqlitePath)
		repo, err = graph.NewSQLite(ctx, sqlitePath)
		if err != nil {
			log.Fatalf("Failed to open SQLite database: %v", err)
		}
	case "neo4j":
		neo4jURI := getEnv("NEO4J_URI", "bolt://localhost:7687")
		neo4jUser := getEnv("NEO4J_USER", "neo4j")
		neo4jPassword := getEnv("NEO4J_PASSWORD", "password")

		log.Printf("Using Neo4j backend: %s", neo4jURI)
		repo, err = graph.NewNeo4j(ctx, graph.Config{
			URI:      neo4jURI,
			Username: neo4jUser,
			Password: neo4jPassword,
			Database: "neo4j",
		})
		if err != nil {
			log.Fatalf("Failed to connect to Neo4j: %v", err)
		}
	default:
		log.Fatalf("Unknown backend: %s (use 'sqlite' or 'neo4j')", backend)
	}
	defer repo.Close(ctx)

	log.Println("Connected to database successfully")

	// Create indexes for performance
	if err := repo.EnsureIndexes(ctx); err != nil {
		log.Printf("Warning: Failed to create indexes: %v", err)
	} else {
		log.Println("Database indexes ensured")
	}

	// Initialize subscription manager
	subMgr := subscriptions.NewManager(repo)
	if err := subMgr.Start(ctx); err != nil {
		log.Printf("Warning: Failed to start subscription manager: %v", err)
	}
	defer subMgr.Stop()

	// Wire up event emission from repository to subscription manager
	repo.SetEventEmitter(subMgr.GetEmitter())

	// Initialize API server
	apiServer := api.New(repo, subMgr)

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
		r.Get("/nodes/{id}/history", apiServer.GetNodeHistory)
		r.Patch("/nodes/{id}", apiServer.UpdateNode)
		r.Delete("/nodes/{id}", apiServer.DeleteNode)
		r.Get("/nodes/{id}/links", apiServer.GetLinks)
		r.Post("/links", apiServer.CreateLink)
		r.Delete("/links", apiServer.DeleteLink)

		// Query endpoints
		r.Get("/query/filter", apiServer.QueryFilter)
		r.Get("/query/search", apiServer.QuerySearch)
		r.Get("/query/traverse", apiServer.QueryTraverse)
		r.Get("/query/subgraph", apiServer.QuerySubgraph)
		r.Get("/query/attention_subgraph", apiServer.QueryAttentionSubgraph)
		r.Get("/query/by_lens", apiServer.QueryByLens)

		// Graph exploration
		r.Get("/graph/map", apiServer.GraphMap)
		r.Get("/graph/export", apiServer.ExportLens)

		// Attention edge endpoints
		r.Post("/edges/attention", apiServer.UpdateAttentionEdge)
		r.Post("/edges/attention/prune", apiServer.PruneAttentionEdges)

		// Lens endpoints
		r.Post("/lenses", apiServer.CreateLens)
		r.Get("/lenses", apiServer.ListLenses)
		r.Get("/lenses/{id}", apiServer.GetLens)
		r.Patch("/lenses/{id}", apiServer.UpdateLens)
		r.Delete("/lenses/{id}", apiServer.DeleteLens)
		r.Get("/lenses/{id}/entities", apiServer.GetLensEntities)

		// Subscription endpoints
		r.Post("/subscriptions", apiServer.CreateSubscription)
		r.Get("/subscriptions", apiServer.ListSubscriptions)
		r.Get("/subscriptions/{id}", apiServer.GetSubscription)
		r.Patch("/subscriptions/{id}", apiServer.UpdateSubscription)
		r.Delete("/subscriptions/{id}", apiServer.DeleteSubscription)
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
