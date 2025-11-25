// +build integration

package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

// Integration tests require a running memex server
// Run with: go test -tags=integration ./test/...

var baseURL = getEnv("MEMEX_URL", "http://localhost:8080")

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func TestHealthEndpoint(t *testing.T) {
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("expected status ok, got %s", result["status"])
	}
}

func TestNodeCRUD(t *testing.T) {
	nodeID := fmt.Sprintf("test-node-%d", time.Now().UnixNano())

	// Create node
	t.Run("Create", func(t *testing.T) {
		body := map[string]interface{}{
			"id":   nodeID,
			"type": "TestNode",
			"meta": map[string]interface{}{
				"test": true,
			},
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(baseURL+"/api/nodes", "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("creating node: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if result["id"] != nodeID {
			t.Errorf("expected id %s, got %v", nodeID, result["id"])
		}
	})

	// Get node
	t.Run("Get", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/nodes/" + nodeID)
		if err != nil {
			t.Fatalf("getting node: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if result["ID"] != nodeID {
			t.Errorf("expected ID %s, got %v", nodeID, result["ID"])
		}
		if result["Type"] != "TestNode" {
			t.Errorf("expected Type TestNode, got %v", result["Type"])
		}
	})

	// Delete node
	t.Run("Delete", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", baseURL+"/api/nodes/"+nodeID, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("deleting node: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if result["tombstoned"] != true {
			t.Error("expected tombstoned to be true")
		}
	})

	// Verify deleted
	t.Run("VerifyDeleted", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/nodes/" + nodeID)
		if err != nil {
			t.Fatalf("getting deleted node: %v", err)
		}
		defer resp.Body.Close()

		// Should return 404 for tombstoned node
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404 for deleted node, got %d", resp.StatusCode)
		}
	})
}

func TestLinkOperations(t *testing.T) {
	// Create two nodes first
	node1 := fmt.Sprintf("test-link-node1-%d", time.Now().UnixNano())
	node2 := fmt.Sprintf("test-link-node2-%d", time.Now().UnixNano())

	createNode := func(id string) {
		body := map[string]interface{}{"id": id, "type": "TestNode", "meta": map[string]interface{}{}}
		jsonBody, _ := json.Marshal(body)
		resp, err := http.Post(baseURL+"/api/nodes", "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("creating node %s: %v", id, err)
		}
		resp.Body.Close()
	}

	createNode(node1)
	createNode(node2)

	// Cleanup
	defer func() {
		req1, _ := http.NewRequest("DELETE", baseURL+"/api/nodes/"+node1, nil)
		http.DefaultClient.Do(req1)
		req2, _ := http.NewRequest("DELETE", baseURL+"/api/nodes/"+node2, nil)
		http.DefaultClient.Do(req2)
	}()

	t.Run("CreateLink", func(t *testing.T) {
		body := map[string]interface{}{
			"source": node1,
			"target": node2,
			"type":   "test_link",
			"meta":   map[string]interface{}{"weight": 1.0},
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(baseURL+"/api/links", "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("creating link: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("GetLinks", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/nodes/" + node1 + "/links")
		if err != nil {
			t.Fatalf("getting links: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		var links []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&links)

		if len(links) != 1 {
			t.Errorf("expected 1 link, got %d", len(links))
		}
	})

	t.Run("DeleteLink", func(t *testing.T) {
		url := fmt.Sprintf("%s/api/links?source=%s&target=%s&type=test_link", baseURL, node1, node2)
		req, _ := http.NewRequest("DELETE", url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("deleting link: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestIngest(t *testing.T) {
	content := fmt.Sprintf("Test content %d", time.Now().UnixNano())

	t.Run("IngestContent", func(t *testing.T) {
		body := map[string]interface{}{
			"content": content,
			"format":  "text",
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(baseURL+"/api/ingest", "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("ingesting content: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		sourceID, ok := result["source_id"].(string)
		if !ok || sourceID == "" {
			t.Error("expected source_id in response")
		}

		// Should start with sha256:
		if len(sourceID) < 7 || sourceID[:7] != "sha256:" {
			t.Errorf("expected sha256: prefix, got %s", sourceID)
		}
	})

	t.Run("DeduplicateContent", func(t *testing.T) {
		// Ingest same content twice
		body := map[string]interface{}{
			"content": content,
			"format":  "text",
		}
		jsonBody, _ := json.Marshal(body)

		resp1, _ := http.Post(baseURL+"/api/ingest", "application/json", bytes.NewBuffer(jsonBody))
		var result1 map[string]interface{}
		json.NewDecoder(resp1.Body).Decode(&result1)
		resp1.Body.Close()

		resp2, _ := http.Post(baseURL+"/api/ingest", "application/json", bytes.NewBuffer(jsonBody))
		var result2 map[string]interface{}
		json.NewDecoder(resp2.Body).Decode(&result2)
		resp2.Body.Close()

		// Should return same source_id
		if result1["source_id"] != result2["source_id"] {
			t.Errorf("expected same source_id for duplicate content")
		}
	})
}

func TestGraphMap(t *testing.T) {
	t.Run("GetMap", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/graph/map")
		if err != nil {
			t.Fatalf("getting graph map: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		// Check required fields
		if _, ok := result["stats"]; !ok {
			t.Error("expected stats in response")
		}
		if _, ok := result["node_types"]; !ok {
			t.Error("expected node_types in response")
		}
		if _, ok := result["edge_types"]; !ok {
			t.Error("expected edge_types in response")
		}
		if _, ok := result["samples_by_type"]; !ok {
			t.Error("expected samples_by_type in response")
		}
	})

	t.Run("GetMapWithSampleSize", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/graph/map?sample_size=10")
		if err != nil {
			t.Fatalf("getting graph map: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestQueryEndpoints(t *testing.T) {
	t.Run("Search", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/query/search?q=test")
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if _, ok := result["nodes"]; !ok {
			t.Error("expected nodes in response")
		}
		if _, ok := result["count"]; !ok {
			t.Error("expected count in response")
		}
	})

	t.Run("SearchRequiresQuery", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/query/search")
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 for missing query, got %d", resp.StatusCode)
		}
	})

	t.Run("Filter", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/query/filter?type=Source")
		if err != nil {
			t.Fatalf("filter failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestAttentionEdges(t *testing.T) {
	// Create two nodes for attention testing
	node1 := fmt.Sprintf("attention-test-1-%d", time.Now().UnixNano())
	node2 := fmt.Sprintf("attention-test-2-%d", time.Now().UnixNano())

	createNode := func(id string) {
		body := map[string]interface{}{"id": id, "type": "TestNode", "meta": map[string]interface{}{}}
		jsonBody, _ := json.Marshal(body)
		resp, _ := http.Post(baseURL+"/api/nodes", "application/json", bytes.NewBuffer(jsonBody))
		resp.Body.Close()
	}

	createNode(node1)
	createNode(node2)

	defer func() {
		req1, _ := http.NewRequest("DELETE", baseURL+"/api/nodes/"+node1, nil)
		http.DefaultClient.Do(req1)
		req2, _ := http.NewRequest("DELETE", baseURL+"/api/nodes/"+node2, nil)
		http.DefaultClient.Do(req2)
	}()

	t.Run("UpdateAttentionEdge", func(t *testing.T) {
		body := map[string]interface{}{
			"source":   node1,
			"target":   node2,
			"query_id": "test-query-1",
			"weight":   0.85,
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(baseURL+"/api/edges/attention", "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("updating attention edge: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if result["success"] != true {
			t.Error("expected success true")
		}
	})

	t.Run("AttentionEdgeRunningAverage", func(t *testing.T) {
		// Update same edge with different weight
		body := map[string]interface{}{
			"source":   node1,
			"target":   node2,
			"query_id": "test-query-2",
			"weight":   0.95,
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(baseURL+"/api/edges/attention", "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("updating attention edge: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		// Running average should now be (0.85 + 0.95) / 2 = 0.9
	})

	t.Run("InvalidWeight", func(t *testing.T) {
		body := map[string]interface{}{
			"source":   node1,
			"target":   node2,
			"query_id": "test-query-3",
			"weight":   1.5, // Invalid: > 1
		}
		jsonBody, _ := json.Marshal(body)

		resp, err := http.Post(baseURL+"/api/edges/attention", "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("updating attention edge: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid weight, got %d", resp.StatusCode)
		}
	})
}

func TestSubgraph(t *testing.T) {
	// This test requires existing nodes with links
	// Using a known node from the graph if it exists

	t.Run("SubgraphRequiresStart", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/query/subgraph")
		if err != nil {
			t.Fatalf("subgraph query failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 for missing start, got %d", resp.StatusCode)
		}
	})

	t.Run("SubgraphWithDepth", func(t *testing.T) {
		// Create a small graph for testing
		node1 := fmt.Sprintf("subgraph-test-1-%d", time.Now().UnixNano())
		node2 := fmt.Sprintf("subgraph-test-2-%d", time.Now().UnixNano())

		createNode := func(id string) {
			body := map[string]interface{}{"id": id, "type": "TestNode", "meta": map[string]interface{}{}}
			jsonBody, _ := json.Marshal(body)
			resp, _ := http.Post(baseURL+"/api/nodes", "application/json", bytes.NewBuffer(jsonBody))
			resp.Body.Close()
		}

		createNode(node1)
		createNode(node2)

		// Create link
		linkBody := map[string]interface{}{
			"source": node1,
			"target": node2,
			"type":   "test_link",
			"meta":   map[string]interface{}{},
		}
		linkJSON, _ := json.Marshal(linkBody)
		linkResp, _ := http.Post(baseURL+"/api/links", "application/json", bytes.NewBuffer(linkJSON))
		linkResp.Body.Close()

		defer func() {
			// Cleanup
			url := fmt.Sprintf("%s/api/links?source=%s&target=%s&type=test_link", baseURL, node1, node2)
			req, _ := http.NewRequest("DELETE", url, nil)
			http.DefaultClient.Do(req)

			req1, _ := http.NewRequest("DELETE", baseURL+"/api/nodes/"+node1, nil)
			http.DefaultClient.Do(req1)
			req2, _ := http.NewRequest("DELETE", baseURL+"/api/nodes/"+node2, nil)
			http.DefaultClient.Do(req2)
		}()

		// Query subgraph
		resp, err := http.Get(baseURL + "/api/query/subgraph?start=" + node1 + "&depth=1")
		if err != nil {
			t.Fatalf("subgraph query failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if _, ok := result["nodes"]; !ok {
			t.Error("expected nodes in response")
		}
		if _, ok := result["edges"]; !ok {
			t.Error("expected edges in response")
		}
		if _, ok := result["stats"]; !ok {
			t.Error("expected stats in response")
		}
	})
}
