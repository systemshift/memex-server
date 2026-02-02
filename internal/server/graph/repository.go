package graph

import (
	"context"
	"time"

	"github.com/systemshift/memex/internal/memex/core"
	"github.com/systemshift/memex/internal/server/subscriptions"
)

// Repository defines the interface for graph storage backends.
// Both SQLite and Neo4j implement this interface.
type Repository interface {
	// Lifecycle
	Close(ctx context.Context) error
	EnsureIndexes(ctx context.Context) error
	SetEventEmitter(emitter func(subscriptions.Event))

	// Core node operations (Phase 1 - TUI support)
	CreateNode(ctx context.Context, node *core.Node) error
	GetNode(ctx context.Context, id string) (*core.Node, error)
	GetLinks(ctx context.Context, nodeID string) ([]*core.Link, error)
	SearchNodes(ctx context.Context, searchTerm string, limit int, offset int) ([]*core.Node, error)
	FilterNodes(ctx context.Context, nodeTypes []string, propertyKey string, propertyValue string, limit int, offset int) ([]*core.Node, error)
	TraverseGraph(ctx context.Context, startNodeID string, depth int, relationshipTypes []string, limit int, offset int) (map[string]*core.Node, error)

	// Link operations
	CreateLink(ctx context.Context, link *core.Link) error
	DeleteLink(ctx context.Context, sourceID string, targetID string, linkType string) error

	// Node listing
	ListNodes(ctx context.Context) ([]string, error)

	// Version operations
	GetNodeAtVersion(ctx context.Context, id string, version int) (*core.Node, error)
	GetNodeAtTime(ctx context.Context, id string, asOf time.Time) (*core.Node, error)
	GetNodeHistory(ctx context.Context, id string) ([]core.VersionInfo, error)

	// Update operations
	UpdateNodeMeta(ctx context.Context, id string, meta map[string]any) error
	UpdateNodeMetaWithNote(ctx context.Context, id string, meta map[string]any, changeNote, changedBy string) error

	// Delete operations
	DeleteNode(ctx context.Context, nodeID string, force bool) error

	// Attention edge operations
	UpdateAttentionEdge(ctx context.Context, source, target, queryID string, weight float64) error
	GetAttentionSubgraph(ctx context.Context, startNodeID string, minWeight float64, maxNodes int) (*Subgraph, error)
	PruneWeakAttentionEdges(ctx context.Context, minWeight float64, minQueryCount int) (int, error)

	// Graph exploration
	GetGraphMap(ctx context.Context, sampleSize int) (*GraphMap, error)
	GetSubgraph(ctx context.Context, startNodeID string, depth int, relationshipTypes []string) (*Subgraph, error)

	// Lens operations
	GetEntitiesInterpretedThrough(ctx context.Context, lensID string) ([]*core.Node, error)
	CreateInterpretedThroughLink(ctx context.Context, entityID, lensID string, meta map[string]interface{}) error
	QueryByLens(ctx context.Context, lensID string, pattern string, limit int, offset int) ([]*core.Node, error)
	ExportLens(ctx context.Context, lensID string, includeExtractedFrom bool) (*LensExport, error)

	// Subscription persistence (implements subscriptions.Repository)
	CreateSubscriptionNode(ctx context.Context, sub *subscriptions.Subscription) error
	UpdateSubscriptionNode(ctx context.Context, sub *subscriptions.Subscription) error
	DeleteSubscriptionNode(ctx context.Context, id string) error
	LoadSubscriptions(ctx context.Context) ([]*subscriptions.Subscription, error)

	// Raw query (Neo4j only - SQLite returns error)
	ExecuteCypherRead(ctx context.Context, cypher string, params map[string]interface{}) ([]map[string]interface{}, error)
}
