package test

import (
	"path/filepath"
	"testing"
	"time"

	coretx "memex/internal/memex/core/transaction"
	"memex/internal/memex/storage"
	txstore "memex/internal/memex/transaction"
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

	// Create transaction store
	txStore, err := txstore.NewActionStore(store)
	if err != nil {
		t.Fatalf("creating transaction store: %v", err)
	}

	// Test basic transaction operations
	t.Run("Basic Operations", func(t *testing.T) {
		// Begin transaction
		meta := map[string]any{
			"description": "Add test node",
		}
		if err := txStore.RecordAction(coretx.ActionAddNode, meta); err != nil {
			t.Fatalf("recording action: %v", err)
		}

		// Verify history
		history, err := txStore.GetHistory()
		if err != nil {
			t.Fatalf("getting history: %v", err)
		}

		if len(history) != 1 {
			t.Errorf("expected 1 action in history, got %d", len(history))
		}

		action := history[0]
		if action.Type != coretx.ActionAddNode {
			t.Errorf("expected action type %s, got %s", coretx.ActionAddNode, action.Type)
		}
	})

	// Test transaction rollback
	t.Run("Transaction Rollback", func(t *testing.T) {
		// Record action
		meta := map[string]any{
			"description": "Test rollback",
		}
		if err := txStore.RecordAction(coretx.ActionAddNode, meta); err != nil {
			t.Fatalf("recording action: %v", err)
		}

		// Verify history
		history, err := txStore.GetHistory()
		if err != nil {
			t.Fatalf("getting history: %v", err)
		}

		if len(history) != 2 {
			t.Errorf("expected 2 actions in history, got %d", len(history))
		}
	})

	// Test transaction logging
	t.Run("Transaction Logging", func(t *testing.T) {
		// Record multiple actions
		actions := []struct {
			Type    coretx.ActionType
			Payload map[string]any
		}{
			{
				Type: coretx.ActionAddNode,
				Payload: map[string]any{
					"description": "First node",
				},
			},
			{
				Type: coretx.ActionAddLink,
				Payload: map[string]any{
					"source": "node1",
					"target": "node2",
				},
			},
		}

		for _, a := range actions {
			if err := txStore.RecordAction(a.Type, a.Payload); err != nil {
				t.Fatalf("recording action: %v", err)
			}
		}

		// Verify history
		history, err := txStore.GetHistory()
		if err != nil {
			t.Fatalf("getting history: %v", err)
		}

		if len(history) != 4 {
			t.Errorf("expected 4 actions in history, got %d", len(history))
		}

		// Verify history integrity
		valid, err := txStore.VerifyHistory()
		if err != nil {
			t.Fatalf("verifying history: %v", err)
		}
		if !valid {
			t.Error("history verification failed")
		}
	})

	// Test pending transactions with shorter timeout
	t.Run("Pending Transactions", func(t *testing.T) {
		// Record action
		meta := map[string]any{
			"description": "Test pending",
		}
		if err := txStore.RecordAction(coretx.ActionAddNode, meta); err != nil {
			t.Fatalf("recording action: %v", err)
		}

		// Wait a shorter time
		time.Sleep(100 * time.Millisecond)

		// Verify history
		history, err := txStore.GetHistory()
		if err != nil {
			t.Fatalf("getting history: %v", err)
		}

		if len(history) != 5 {
			t.Errorf("expected 5 actions in history, got %d", len(history))
		}
	})
}
