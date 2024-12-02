package test

import (
	"path/filepath"
	"testing"
	"time"

	"memex/internal/memex/storage"
	"memex/internal/memex/storage/tx"
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
	txStore := tx.NewStore(store.GetFile(), store.GetLockManager())
	txLog, err := tx.NewLog(tmpDir, store.GetLockManager())
	if err != nil {
		t.Fatalf("creating transaction log: %v", err)
	}
	txManager := tx.NewManager(store.GetFile(), store.GetLockManager())
	txVerifier := tx.NewVerifier(txStore, txLog, txManager, store.GetLockManager())

	// Test basic transaction operations
	t.Run("Basic Operations", func(t *testing.T) {
		// Begin transaction
		txID, err := txStore.Begin(tx.TxTypeNode, map[string]any{
			"description": "Add test node",
		})
		if err != nil {
			t.Fatalf("beginning transaction: %v", err)
		}

		// Create operation
		op, err := tx.NewOperation(tx.OpTypeCreate, "test-node", "create", []byte("test content"), map[string]any{
			"type": "note",
		})
		if err != nil {
			t.Fatalf("creating operation: %v", err)
		}

		// Add operation to transaction
		if err := txStore.AddOperation(txID, op); err != nil {
			t.Fatalf("adding operation: %v", err)
		}

		// Commit transaction
		if err := txStore.Commit(txID); err != nil {
			t.Fatalf("committing transaction: %v", err)
		}

		// Verify transaction
		if err := txVerifier.VerifyTransaction(txID); err != nil {
			t.Errorf("verifying transaction: %v", err)
		}
	})

	// Test transaction rollback
	t.Run("Transaction Rollback", func(t *testing.T) {
		// Begin transaction
		txID, err := txStore.Begin(tx.TxTypeNode, nil)
		if err != nil {
			t.Fatalf("beginning transaction: %v", err)
		}

		// Create operation
		op, err := tx.NewOperation(tx.OpTypeCreate, "test-node-2", "create", []byte("test content 2"), nil)
		if err != nil {
			t.Fatalf("creating operation: %v", err)
		}

		// Add operation to transaction
		if err := txStore.AddOperation(txID, op); err != nil {
			t.Fatalf("adding operation: %v", err)
		}

		// Rollback transaction
		if err := txStore.Rollback(txID); err != nil {
			t.Fatalf("rolling back transaction: %v", err)
		}

		// Verify transaction status
		transaction, err := txStore.Get(txID)
		if err != nil {
			t.Fatalf("getting transaction: %v", err)
		}
		if transaction.Status != tx.TxStatusRollback {
			t.Errorf("expected transaction status %d, got %d", tx.TxStatusRollback, transaction.Status)
		}
	})

	// Test transaction logging
	t.Run("Transaction Logging", func(t *testing.T) {
		// Begin transaction
		txID, err := txStore.Begin(tx.TxTypeNode, nil)
		if err != nil {
			t.Fatalf("beginning transaction: %v", err)
		}

		// Get transaction
		transaction, err := txStore.Get(txID)
		if err != nil {
			t.Fatalf("getting transaction: %v", err)
		}

		// Log transaction
		if err := txLog.LogTransaction(transaction); err != nil {
			t.Fatalf("logging transaction: %v", err)
		}

		// Verify log consistency
		if err := txVerifier.VerifyLogConsistency(); err != nil {
			t.Errorf("verifying log consistency: %v", err)
		}
	})

	// Test pending transactions
	t.Run("Pending Transactions", func(t *testing.T) {
		// Begin transaction but don't commit
		txID, err := txStore.Begin(tx.TxTypeNode, nil)
		if err != nil {
			t.Fatalf("beginning transaction: %v", err)
		}

		// Wait a bit to ensure transaction times out
		time.Sleep(6 * time.Second)

		// Verify pending transactions
		if err := txVerifier.VerifyPendingTransactions(); err != nil {
			t.Errorf("verifying pending transactions: %v", err)
		}

		// Transaction should be rolled back due to timeout
		transaction, err := txStore.Get(txID)
		if err != nil {
			t.Fatalf("getting transaction: %v", err)
		}
		if transaction.Status != tx.TxStatusRollback {
			t.Errorf("expected transaction status %d, got %d", tx.TxStatusRollback, transaction.Status)
		}
	})
}
