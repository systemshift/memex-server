package subscriptions

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EventEmitter is a function that receives events from the repository
type EventEmitter func(Event)

// Repository interface for subscription persistence
type Repository interface {
	CreateSubscriptionNode(ctx context.Context, sub *Subscription) error
	UpdateSubscriptionNode(ctx context.Context, sub *Subscription) error
	DeleteSubscriptionNode(ctx context.Context, id string) error
	LoadSubscriptions(ctx context.Context) ([]*Subscription, error)
	ExecuteCypherRead(ctx context.Context, cypher string, params map[string]interface{}) ([]map[string]interface{}, error)
}

// Manager handles subscription lifecycle and event processing
type Manager struct {
	repo          Repository
	subscriptions map[string]*Subscription
	eventChan     chan Event
	notifier      *Notifier
	matcher       *Matcher
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// NewManager creates a new subscription manager
func NewManager(repo Repository) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		repo:          repo,
		subscriptions: make(map[string]*Subscription),
		eventChan:     make(chan Event, 1000), // Buffered to avoid blocking writes
		ctx:           ctx,
		cancel:        cancel,
	}
	m.notifier = NewNotifier()
	m.matcher = NewMatcher(repo)
	return m
}

// Start begins processing events
func (m *Manager) Start(ctx context.Context) error {
	// Load existing subscriptions from storage
	if err := m.loadSubscriptions(ctx); err != nil {
		log.Printf("Warning: failed to load subscriptions: %v", err)
	}

	// Start event processing goroutine
	m.wg.Add(1)
	go m.processEvents()

	log.Printf("Subscription manager started with %d subscriptions", len(m.subscriptions))
	return nil
}

// Stop gracefully shuts down the manager
func (m *Manager) Stop() {
	m.cancel()
	close(m.eventChan)
	m.wg.Wait()
	m.notifier.Close()
	log.Println("Subscription manager stopped")
}

// EmitEvent sends an event to be processed (called by repository)
func (m *Manager) EmitEvent(event Event) {
	// Non-blocking send - drop events if channel is full
	select {
	case m.eventChan <- event:
	default:
		log.Printf("Warning: event channel full, dropping event %s", event.ID)
	}
}

// GetEmitter returns a function that can be used to emit events
func (m *Manager) GetEmitter() EventEmitter {
	return m.EmitEvent
}

// Register adds a new subscription
func (m *Manager) Register(ctx context.Context, req *CreateSubscriptionRequest) (*Subscription, error) {
	sub := &Subscription{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Pattern:     req.Pattern,
		Webhook:     req.Webhook,
		WebSocket:   req.WebSocket,
		Enabled:     true,
		Created:     time.Now(),
		Modified:    time.Now(),
		FireCount:   0,
	}

	// Validate
	if sub.Name == "" {
		return nil, fmt.Errorf("subscription name is required")
	}
	if sub.Webhook == "" && !sub.WebSocket {
		return nil, fmt.Errorf("subscription must have webhook URL or websocket enabled")
	}

	// Persist to storage
	if err := m.repo.CreateSubscriptionNode(ctx, sub); err != nil {
		return nil, fmt.Errorf("failed to persist subscription: %w", err)
	}

	// Add to in-memory cache
	m.mu.Lock()
	m.subscriptions[sub.ID] = sub
	m.mu.Unlock()

	log.Printf("Registered subscription: %s (%s)", sub.ID, sub.Name)
	return sub, nil
}

// Unregister removes a subscription
func (m *Manager) Unregister(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.subscriptions[id]; !exists {
		return fmt.Errorf("subscription not found: %s", id)
	}

	// Remove from storage
	if err := m.repo.DeleteSubscriptionNode(ctx, id); err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	// Remove from cache
	delete(m.subscriptions, id)

	// Clean up any WebSocket connections
	m.notifier.UnregisterWSClient(id)

	log.Printf("Unregistered subscription: %s", id)
	return nil
}

// Update modifies an existing subscription
func (m *Manager) Update(ctx context.Context, id string, req *UpdateSubscriptionRequest) (*Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sub, exists := m.subscriptions[id]
	if !exists {
		return nil, fmt.Errorf("subscription not found: %s", id)
	}

	// Apply updates
	if req.Name != nil {
		sub.Name = *req.Name
	}
	if req.Description != nil {
		sub.Description = *req.Description
	}
	if req.Pattern != nil {
		sub.Pattern = *req.Pattern
	}
	if req.Webhook != nil {
		sub.Webhook = *req.Webhook
	}
	if req.WebSocket != nil {
		sub.WebSocket = *req.WebSocket
	}
	if req.Enabled != nil {
		sub.Enabled = *req.Enabled
	}
	sub.Modified = time.Now()

	// Persist changes
	if err := m.repo.UpdateSubscriptionNode(ctx, sub); err != nil {
		return nil, fmt.Errorf("failed to update subscription: %w", err)
	}

	return sub, nil
}

// Get returns a subscription by ID
func (m *Manager) Get(id string) (*Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sub, exists := m.subscriptions[id]
	if !exists {
		return nil, fmt.Errorf("subscription not found: %s", id)
	}
	return sub, nil
}

// List returns all subscriptions
func (m *Manager) List() []*Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Subscription, 0, len(m.subscriptions))
	for _, sub := range m.subscriptions {
		result = append(result, sub)
	}
	return result
}

// RegisterWSClient registers a WebSocket connection for a subscription
func (m *Manager) RegisterWSClient(subID string, conn WSConn) error {
	m.mu.RLock()
	_, exists := m.subscriptions[subID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("subscription not found: %s", subID)
	}

	m.notifier.RegisterWSClient(subID, conn)
	return nil
}

// UnregisterWSClient removes a WebSocket connection
func (m *Manager) UnregisterWSClient(subID string) {
	m.notifier.UnregisterWSClient(subID)
}

// processEvents is the main event processing loop
func (m *Manager) processEvents() {
	defer m.wg.Done()

	for event := range m.eventChan {
		m.handleEvent(event)
	}
}

// handleEvent processes a single event against all subscriptions
func (m *Manager) handleEvent(event Event) {
	m.mu.RLock()
	subs := make([]*Subscription, 0, len(m.subscriptions))
	for _, sub := range m.subscriptions {
		if sub.Enabled {
			subs = append(subs, sub)
		}
	}
	m.mu.RUnlock()

	for _, sub := range subs {
		go m.evaluateSubscription(event, sub)
	}
}

// evaluateSubscription checks if an event matches a subscription and fires notification
func (m *Manager) evaluateSubscription(event Event, sub *Subscription) {
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	matched, results := m.matcher.Match(ctx, event, sub.Pattern)
	if !matched {
		return
	}

	// Create notification
	now := time.Now()
	notification := Notification{
		SubscriptionID:   sub.ID,
		SubscriptionName: sub.Name,
		Event:            event,
		MatchedAt:        now,
		QueryResults:     results,
	}

	// Update subscription state
	m.mu.Lock()
	if s, exists := m.subscriptions[sub.ID]; exists {
		s.LastFired = &now
		s.FireCount++
	}
	m.mu.Unlock()

	// Send notifications
	if sub.Webhook != "" {
		go m.notifier.SendWebhook(sub.Webhook, notification)
	}
	if sub.WebSocket {
		go m.notifier.SendWebSocket(sub.ID, notification)
	}

	log.Printf("Subscription %s fired for event %s", sub.ID, event.Type)
}

// loadSubscriptions loads all subscriptions from storage into memory
func (m *Manager) loadSubscriptions(ctx context.Context) error {
	subs, err := m.repo.LoadSubscriptions(ctx)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sub := range subs {
		m.subscriptions[sub.ID] = sub
	}

	return nil
}

// subscriptionToMeta converts a subscription to a map for storage
func subscriptionToMeta(sub *Subscription) map[string]interface{} {
	patternJSON, _ := json.Marshal(sub.Pattern)
	meta := map[string]interface{}{
		"name":        sub.Name,
		"description": sub.Description,
		"pattern":     string(patternJSON),
		"webhook":     sub.Webhook,
		"websocket":   sub.WebSocket,
		"enabled":     sub.Enabled,
		"fire_count":  sub.FireCount,
	}
	if sub.LastFired != nil {
		meta["last_fired"] = sub.LastFired.Format(time.RFC3339)
	}
	return meta
}

// metaToSubscription converts storage metadata back to a subscription
func metaToSubscription(id string, meta map[string]interface{}, created, modified time.Time) (*Subscription, error) {
	sub := &Subscription{
		ID:       id,
		Created:  created,
		Modified: modified,
	}

	if v, ok := meta["name"].(string); ok {
		sub.Name = v
	}
	if v, ok := meta["description"].(string); ok {
		sub.Description = v
	}
	if v, ok := meta["webhook"].(string); ok {
		sub.Webhook = v
	}
	if v, ok := meta["websocket"].(bool); ok {
		sub.WebSocket = v
	}
	if v, ok := meta["enabled"].(bool); ok {
		sub.Enabled = v
	}
	if v, ok := meta["fire_count"].(float64); ok {
		sub.FireCount = int(v)
	}
	if v, ok := meta["last_fired"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			sub.LastFired = &t
		}
	}
	if v, ok := meta["pattern"].(string); ok {
		var pattern SubscriptionPattern
		if err := json.Unmarshal([]byte(v), &pattern); err == nil {
			sub.Pattern = pattern
		}
	}

	return sub, nil
}
