package subscriptions

import (
	"time"
)

// Event represents a change in the graph that can trigger subscriptions
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // node.created, node.updated, node.deleted, link.created, link.deleted
	Timestamp time.Time              `json:"timestamp"`

	// Node event fields
	NodeID   string `json:"node_id,omitempty"`
	NodeType string `json:"node_type,omitempty"`

	// Link event fields
	LinkSource string `json:"link_source,omitempty"`
	LinkTarget string `json:"link_target,omitempty"`
	LinkType   string `json:"link_type,omitempty"`

	// Context
	Meta map[string]interface{} `json:"meta,omitempty"`
}

// Event type constants
const (
	EventNodeCreated = "node.created"
	EventNodeUpdated = "node.updated"
	EventNodeDeleted = "node.deleted"
	EventLinkCreated = "link.created"
	EventLinkDeleted = "link.deleted"
)

// SubscriptionPattern defines what events a subscription matches
type SubscriptionPattern struct {
	// Simple matching (evaluated in Go, fast)
	EventTypes []string               `json:"event_types,omitempty"` // Match specific event types
	NodeTypes  []string               `json:"node_types,omitempty"`  // Match specific node types
	LinkTypes  []string               `json:"link_types,omitempty"`  // Match specific link types
	MetaMatch  map[string]interface{} `json:"meta_match,omitempty"`  // Match metadata fields

	// Advanced matching (Cypher query, evaluated against Neo4j)
	Cypher string `json:"cypher,omitempty"`
}

// Subscription represents a standing query that fires when patterns match
type Subscription struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// What to match
	Pattern SubscriptionPattern `json:"pattern"`

	// How to notify
	Webhook   string `json:"webhook,omitempty"`   // URL to POST notifications
	WebSocket bool   `json:"websocket,omitempty"` // Push via WebSocket connection

	// State
	Enabled   bool       `json:"enabled"`
	Created   time.Time  `json:"created"`
	Modified  time.Time  `json:"modified"`
	LastFired *time.Time `json:"last_fired,omitempty"`
	FireCount int        `json:"fire_count"`
}

// Notification is sent when a subscription pattern matches
type Notification struct {
	SubscriptionID   string    `json:"subscription_id"`
	SubscriptionName string    `json:"subscription_name"`
	Event            Event     `json:"event"`
	MatchedAt        time.Time `json:"matched_at"`

	// For Cypher patterns, include query results
	QueryResults []map[string]interface{} `json:"query_results,omitempty"`
}

// CreateSubscriptionRequest is the API request to create a subscription
type CreateSubscriptionRequest struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Pattern     SubscriptionPattern `json:"pattern"`
	Webhook     string              `json:"webhook,omitempty"`
	WebSocket   bool                `json:"websocket,omitempty"`
}

// UpdateSubscriptionRequest is the API request to update a subscription
type UpdateSubscriptionRequest struct {
	Name        *string              `json:"name,omitempty"`
	Description *string              `json:"description,omitempty"`
	Pattern     *SubscriptionPattern `json:"pattern,omitempty"`
	Webhook     *string              `json:"webhook,omitempty"`
	WebSocket   *bool                `json:"websocket,omitempty"`
	Enabled     *bool                `json:"enabled,omitempty"`
}

// SubscriptionResponse is the API response for subscription operations
type SubscriptionResponse struct {
	Subscription *Subscription `json:"subscription,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// ListSubscriptionsResponse is the API response for listing subscriptions
type ListSubscriptionsResponse struct {
	Subscriptions []*Subscription `json:"subscriptions"`
	Count         int             `json:"count"`
}
