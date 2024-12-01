package test

import (
	"path/filepath"
	"testing"
	"time"

	"memex/internal/memex/storage"
	"memex/internal/memex/transaction"
)

func TestTransactionSystem(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test.mx")

	// Create repository
	store, err := storage.CreateMX(repoPath)
	if err != nil {
		t.Fatalf("creating repository: %v", err)
	}
	defer store.Close()

	// Create action store
	actionStore, err := transaction.NewActionStore(store)
	if err != nil {
		t.Fatalf("creating action store: %v", err)
	}
	defer actionStore.Close()

	// Test basic action recording
	t.Run("Record Actions", func(t *testing.T) {
		// Add a node
		nodePayload := map[string]any{
			"node_id":  "123",
			"content":  "Test content",
			"type":     "file",
			"filename": "test.txt",
		}
		if err := actionStore.RecordAction(transaction.ActionAddNode, nodePayload); err != nil {
			t.Fatalf("recording node action: %v", err)
		}

		// Add another node
		node2Payload := map[string]any{
			"node_id":  "456",
			"content":  "Another test",
			"type":     "file",
			"filename": "test2.txt",
		}
		if err := actionStore.RecordAction(transaction.ActionAddNode, node2Payload); err != nil {
			t.Fatalf("recording second node action: %v", err)
		}

		// Create a link between nodes
		linkPayload := map[string]any{
			"source": "123",
			"target": "456",
			"type":   "references",
		}
		if err := actionStore.RecordAction(transaction.ActionAddLink, linkPayload); err != nil {
			t.Fatalf("recording link action: %v", err)
		}

		// Verify history length
		history, err := actionStore.GetHistory()
		if err != nil {
			t.Fatalf("getting history: %v", err)
		}
		if len(history) != 3 {
			t.Errorf("expected 3 actions, got %d", len(history))
		}
	})

	// Test action verification
	t.Run("Verify Actions", func(t *testing.T) {
		// Create verifier
		verifier := transaction.NewStateHasher()

		// Get history
		actions, err := actionStore.GetHistory()
		if err != nil {
			t.Fatalf("getting history: %v", err)
		}

		// Verify action chain
		if err := verifier.VerifyActionChain(actions); err != nil {
			t.Errorf("verifying action chain: %v", err)
		}

		// Verify each action's state consistency
		for _, action := range actions {
			if err := verifier.VerifyStateConsistency(action, actionStore); err != nil {
				t.Errorf("verifying state for action %s: %v", action.Type, err)
			}
		}
	})

	// Test history verification
	t.Run("Verify Full History", func(t *testing.T) {
		valid, err := actionStore.VerifyHistory()
		if err != nil {
			t.Fatalf("verifying history: %v", err)
		}
		if !valid {
			t.Error("history verification failed")
		}
	})

	// Test action timestamps
	t.Run("Action Timestamps", func(t *testing.T) {
		history, err := actionStore.GetHistory()
		if err != nil {
			t.Fatalf("getting history: %v", err)
		}

		prevTime := time.Time{}
		for i, action := range history {
			if action.Timestamp.Before(prevTime) {
				t.Errorf("action %d timestamp before previous action", i)
			}
			prevTime = action.Timestamp
		}
	})

	// Test invalid actions
	t.Run("Invalid Actions", func(t *testing.T) {
		// Try to record action with missing required payload
		invalidPayload := map[string]any{
			// Missing node_id
			"content": "Test content",
		}
		if err := actionStore.RecordAction(transaction.ActionAddNode, invalidPayload); err == nil {
			t.Error("expected error for invalid payload, got nil")
		}

		// Try to create link with non-existent nodes
		invalidLinkPayload := map[string]any{
			"source": "nonexistent1",
			"target": "nonexistent2",
			"type":   "references",
		}
		if err := actionStore.RecordAction(transaction.ActionAddLink, invalidLinkPayload); err == nil {
			t.Error("expected error for invalid link, got nil")
		}
	})
}
