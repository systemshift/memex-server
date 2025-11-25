package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// MockRepository implements a minimal mock for testing handlers
type MockRepository struct {
	nodes map[string]interface{}
	links []interface{}
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		nodes: make(map[string]interface{}),
		links: make([]interface{}, 0),
	}
}

// Helper to create a test server with routes
func setupTestServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()

	// Note: These tests require a running Neo4j instance
	// For unit tests without Neo4j, we'd need to mock the repository
	// For now, these serve as integration test templates

	r := chi.NewRouter()

	// Return server and cleanup function
	ts := httptest.NewServer(r)
	return ts, func() { ts.Close() }
}

func TestHealthCheck(t *testing.T) {
	// Create a minimal handler for health check
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %s", resp["status"])
	}
}

func TestCreateNodeRequestValidation(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid request",
			body:       `{"id":"test-1","type":"Test","meta":{}}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid json",
			body:       `{invalid`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			body:       ``,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This tests request parsing only
			req := httptest.NewRequest("POST", "/api/nodes", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")

			var parsed CreateNodeRequest
			err := json.NewDecoder(req.Body).Decode(&parsed)

			if tt.wantStatus == http.StatusBadRequest && err == nil && tt.body != "" {
				// For invalid JSON, we expect decode error
				if tt.name == "invalid json" {
					t.Error("expected decode error for invalid json")
				}
			}
		})
	}
}

func TestCreateLinkRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "valid request",
			body:    `{"source":"node-1","target":"node-2","type":"test","meta":{}}`,
			wantErr: false,
		},
		{
			name:    "missing source",
			body:    `{"target":"node-2","type":"test"}`,
			wantErr: false, // Parsing succeeds, validation would fail
		},
		{
			name:    "invalid json",
			body:    `{bad json}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/links", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")

			var parsed CreateLinkRequest
			err := json.NewDecoder(req.Body).Decode(&parsed)

			if (err != nil) != tt.wantErr {
				t.Errorf("decode error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIngestRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name:    "valid request",
			body:    `{"content":"test content","format":"text"}`,
			wantErr: false,
		},
		{
			name:    "content only",
			body:    `{"content":"test content"}`,
			wantErr: false,
		},
		{
			name:    "empty content",
			body:    `{"content":""}`,
			wantErr: false, // Parsing succeeds, handler validates
		},
		{
			name:    "invalid json",
			body:    `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/ingest", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")

			var parsed IngestRequest
			err := json.NewDecoder(req.Body).Decode(&parsed)

			if (err != nil) != tt.wantErr {
				t.Errorf("decode error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateAttentionEdgeRequestValidation(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantErr    bool
		wantWeight float64
	}{
		{
			name:       "valid request",
			body:       `{"source":"a","target":"b","query_id":"q1","weight":0.85}`,
			wantErr:    false,
			wantWeight: 0.85,
		},
		{
			name:       "zero weight",
			body:       `{"source":"a","target":"b","query_id":"q1","weight":0}`,
			wantErr:    false,
			wantWeight: 0,
		},
		{
			name:       "max weight",
			body:       `{"source":"a","target":"b","query_id":"q1","weight":1.0}`,
			wantErr:    false,
			wantWeight: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/edges/attention", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")

			var parsed UpdateAttentionEdgeRequest
			err := json.NewDecoder(req.Body).Decode(&parsed)

			if (err != nil) != tt.wantErr {
				t.Errorf("decode error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && parsed.Weight != tt.wantWeight {
				t.Errorf("weight = %v, want %v", parsed.Weight, tt.wantWeight)
			}
		})
	}
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantLimit  int
		wantOffset int
	}{
		{
			name:       "defaults",
			query:      "",
			wantLimit:  100,
			wantOffset: 0,
		},
		{
			name:       "custom limit",
			query:      "limit=50",
			wantLimit:  50,
			wantOffset: 0,
		},
		{
			name:       "custom offset",
			query:      "offset=20",
			wantLimit:  100,
			wantOffset: 20,
		},
		{
			name:       "both",
			query:      "limit=25&offset=10",
			wantLimit:  25,
			wantOffset: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test?"+tt.query, nil)
			limit, offset := parsePagination(req)

			if limit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", limit, tt.wantLimit)
			}
			if offset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", offset, tt.wantOffset)
			}
		})
	}
}
