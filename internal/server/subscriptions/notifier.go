package subscriptions

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// WSConn is an interface for WebSocket connections
// This allows us to avoid importing gorilla/websocket in the types
type WSConn interface {
	WriteJSON(v interface{}) error
	Close() error
}

// Notifier handles sending notifications via webhooks and WebSockets
type Notifier struct {
	httpClient *http.Client
	wsClients  map[string]WSConn // subscription_id -> connection
	mu         sync.RWMutex
}

// NewNotifier creates a new notifier
func NewNotifier() *Notifier {
	return &Notifier{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		wsClients: make(map[string]WSConn),
	}
}

// Close closes all WebSocket connections
func (n *Notifier) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, conn := range n.wsClients {
		conn.Close()
	}
	n.wsClients = make(map[string]WSConn)
}

// RegisterWSClient registers a WebSocket connection for a subscription
func (n *Notifier) RegisterWSClient(subID string, conn WSConn) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Close existing connection if any
	if existing, ok := n.wsClients[subID]; ok {
		existing.Close()
	}

	n.wsClients[subID] = conn
	log.Printf("WebSocket client registered for subscription: %s", subID)
}

// UnregisterWSClient removes a WebSocket connection
func (n *Notifier) UnregisterWSClient(subID string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if conn, ok := n.wsClients[subID]; ok {
		conn.Close()
		delete(n.wsClients, subID)
		log.Printf("WebSocket client unregistered for subscription: %s", subID)
	}
}

// SendWebhook sends a notification via HTTP POST
func (n *Notifier) SendWebhook(url string, notification Notification) error {
	payload, err := json.Marshal(notification)
	if err != nil {
		log.Printf("Failed to marshal notification for webhook: %v", err)
		return err
	}

	// Attempt with retries
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			time.Sleep(time.Duration(attempt*attempt) * time.Second)
		}

		req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Memex-Event", notification.Event.Type)
		req.Header.Set("X-Memex-Subscription", notification.SubscriptionID)

		resp, err := n.httpClient.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("Webhook delivery attempt %d failed: %v", attempt+1, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("Webhook delivered successfully to %s", url)
			return nil
		}

		lastErr = &WebhookError{
			URL:        url,
			StatusCode: resp.StatusCode,
		}
		log.Printf("Webhook delivery attempt %d got status %d", attempt+1, resp.StatusCode)
	}

	log.Printf("Webhook delivery failed after 3 attempts to %s: %v", url, lastErr)
	return lastErr
}

// SendWebSocket sends a notification via WebSocket
func (n *Notifier) SendWebSocket(subID string, notification Notification) error {
	n.mu.RLock()
	conn, ok := n.wsClients[subID]
	n.mu.RUnlock()

	if !ok {
		// No active WebSocket connection, not an error
		return nil
	}

	if err := conn.WriteJSON(notification); err != nil {
		log.Printf("WebSocket send failed for subscription %s: %v", subID, err)
		// Remove failed connection
		n.UnregisterWSClient(subID)
		return err
	}

	log.Printf("WebSocket notification sent for subscription: %s", subID)
	return nil
}

// HasWSClient checks if a subscription has an active WebSocket client
func (n *Notifier) HasWSClient(subID string) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	_, ok := n.wsClients[subID]
	return ok
}

// WebhookError represents a webhook delivery failure
type WebhookError struct {
	URL        string
	StatusCode int
}

func (e *WebhookError) Error() string {
	return "webhook delivery failed"
}
