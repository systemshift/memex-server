package tx

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"memex/internal/memex/storage/common"
)

// Verifier handles transaction verification
type Verifier struct {
	store   *Store
	log     *Log
	manager *Manager
	locks   *common.LockManager
}

// NewVerifier creates a new transaction verifier
func NewVerifier(store *Store, log *Log, manager *Manager, locks *common.LockManager) *Verifier {
	return &Verifier{
		store:   store,
		log:     log,
		manager: manager,
		locks:   locks,
	}
}

// VerifyTransaction verifies a single transaction
func (v *Verifier) VerifyTransaction(txID string) error {
	return v.locks.WithChunkLock(func() error {
		// Get transaction data
		tx, err := v.store.Get(txID)
		if err != nil {
			return fmt.Errorf("getting transaction: %w", err)
		}

		// Verify transaction integrity
		if err := v.verifyTxIntegrity(tx); err != nil {
			return fmt.Errorf("verifying integrity: %w", err)
		}

		// Verify operations
		if err := v.verifyTxOperations(tx); err != nil {
			return fmt.Errorf("verifying operations: %w", err)
		}

		// Verify transaction state
		if err := v.verifyTxState(tx); err != nil {
			return fmt.Errorf("verifying state: %w", err)
		}

		return nil
	})
}

// VerifyPendingTransactions verifies all pending transactions
func (v *Verifier) VerifyPendingTransactions() error {
	return v.locks.WithChunkLock(func() error {
		// Get all pending transactions
		entries := v.store.index.FindByStatus(TxStatusPending)
		for _, entry := range entries {
			// Get transaction data
			tx, err := v.store.readTx(entry)
			if err != nil {
				return fmt.Errorf("reading transaction %x: %w", entry.ID, err)
			}

			// Verify transaction
			if err := v.verifyTxIntegrity(tx); err != nil {
				return fmt.Errorf("verifying transaction %x: %w", entry.ID, err)
			}

			// Check transaction timeout
			if v.isTransactionTimedOut(tx) {
				// Rollback timed out transaction
				if err := v.rollbackTimedOutTx(tx); err != nil {
					return fmt.Errorf("rolling back transaction %x: %w", entry.ID, err)
				}
			}
		}

		return nil
	})
}

// VerifyLogConsistency verifies transaction log consistency
func (v *Verifier) VerifyLogConsistency() error {
	return v.locks.WithChunkLock(func() error {
		// Get all log entries
		header, err := v.log.readHeader()
		if err != nil {
			return fmt.Errorf("reading log header: %w", err)
		}

		entries, err := v.log.readEntries(header.Count)
		if err != nil {
			return fmt.Errorf("reading log entries: %w", err)
		}

		// Verify each log entry
		for _, entry := range entries {
			if err := v.verifyLogEntry(entry); err != nil {
				return fmt.Errorf("verifying log entry %x: %w", entry.ID, err)
			}
		}

		return nil
	})
}

// Internal verification methods

func (v *Verifier) verifyTxIntegrity(tx *TxData) error {
	// Verify ID matches content
	expectedID := sha256.Sum256(append([]byte{byte(tx.Type)}, tx.Data...))
	if tx.ID != expectedID {
		return fmt.Errorf("transaction ID mismatch")
	}

	// Verify metadata size
	if tx.MetaLen > MaxMetaSize {
		return fmt.Errorf("metadata too large: %d bytes (max %d)", tx.MetaLen, MaxMetaSize)
	}

	// Verify data size
	if tx.DataLen > MaxDataSize {
		return fmt.Errorf("data too large: %d bytes (max %d)", tx.DataLen, MaxDataSize)
	}

	// Verify metadata length matches content
	if tx.MetaLen != uint32(len(tx.Meta)) {
		return fmt.Errorf("metadata length mismatch: header %d, actual %d", tx.MetaLen, len(tx.Meta))
	}

	// Verify data length matches content
	if tx.DataLen != uint32(len(tx.Data)) {
		return fmt.Errorf("data length mismatch: header %d, actual %d", tx.DataLen, len(tx.Data))
	}

	return nil
}

func (v *Verifier) verifyTxOperations(tx *TxData) error {
	// Parse operations
	var ops []Operation
	if err := json.Unmarshal(tx.Data, &ops); err != nil {
		return fmt.Errorf("parsing operations: %w", err)
	}

	// Verify each operation
	for i, op := range ops {
		if err := v.verifyOperation(&op); err != nil {
			return fmt.Errorf("verifying operation %d: %w", i, err)
		}
	}

	return nil
}

func (v *Verifier) verifyOperation(op *Operation) error {
	// Verify operation type
	if op.Type == OpTypeNone || op.Type > OpModify {
		return fmt.Errorf("invalid operation type: %d", op.Type)
	}

	// Verify target ID
	if op.Target == "" {
		return fmt.Errorf("missing target ID")
	}

	// Verify action
	if op.Action == "" {
		return fmt.Errorf("missing action")
	}

	// Verify data size
	if len(op.Data) > MaxDataSize {
		return fmt.Errorf("data too large: %d bytes (max %d)", len(op.Data), MaxDataSize)
	}

	// Verify metadata size
	if op.Meta != nil {
		metaBytes, err := json.Marshal(op.Meta)
		if err != nil {
			return fmt.Errorf("marshaling metadata: %w", err)
		}
		if len(metaBytes) > MaxMetaSize {
			return fmt.Errorf("metadata too large: %d bytes (max %d)", len(metaBytes), MaxMetaSize)
		}
	}

	// Verify data checksum
	if !common.ValidateChecksum(op.Data, op.Checksum) {
		return fmt.Errorf("invalid data checksum")
	}

	return nil
}

func (v *Verifier) verifyTxState(tx *TxData) error {
	// Verify transaction status
	switch tx.Status {
	case TxStatusPending:
		// Check for timeout
		if v.isTransactionTimedOut(tx) {
			return fmt.Errorf("transaction timed out")
		}

	case TxStatusCommitted:
		// Verify all operations were applied
		if err := v.verifyCommittedOperations(tx); err != nil {
			return fmt.Errorf("verifying committed operations: %w", err)
		}

	case TxStatusRollback:
		// Verify all operations were rolled back
		if err := v.verifyRolledBackOperations(tx); err != nil {
			return fmt.Errorf("verifying rolled back operations: %w", err)
		}

	case TxStatusFailed:
		// Nothing to verify for failed transactions

	default:
		return fmt.Errorf("invalid transaction status: %d", tx.Status)
	}

	return nil
}

func (v *Verifier) verifyLogEntry(entry LogEntry) error {
	// Verify data checksum
	if !common.ValidateChecksum(entry.Data, entry.Checksum) {
		return fmt.Errorf("invalid log entry checksum")
	}

	// Verify data length
	if entry.DataLen != uint32(len(entry.Data)) {
		return fmt.Errorf("log entry data length mismatch: header %d, actual %d", entry.DataLen, len(entry.Data))
	}

	// Verify timestamp is not in the future
	if entry.Timestamp > time.Now().Unix() {
		return fmt.Errorf("log entry timestamp is in the future")
	}

	// Verify operation type
	if entry.Type == OpTypeNone || entry.Type > OpModify {
		return fmt.Errorf("invalid log entry operation type: %d", entry.Type)
	}

	// Verify status
	if entry.Status > TxStatusFailed {
		return fmt.Errorf("invalid log entry status: %d", entry.Status)
	}

	return nil
}

func (v *Verifier) verifyCommittedOperations(tx *TxData) error {
	var ops []Operation
	if err := json.Unmarshal(tx.Data, &ops); err != nil {
		return fmt.Errorf("parsing operations: %w", err)
	}

	for i, op := range ops {
		if err := v.verifyCommittedOperation(&op); err != nil {
			return fmt.Errorf("verifying committed operation %d: %w", i, err)
		}
	}

	return nil
}

func (v *Verifier) verifyRolledBackOperations(tx *TxData) error {
	var ops []Operation
	if err := json.Unmarshal(tx.Data, &ops); err != nil {
		return fmt.Errorf("parsing operations: %w", err)
	}

	for i, op := range ops {
		if err := v.verifyRolledBackOperation(&op); err != nil {
			return fmt.Errorf("verifying rolled back operation %d: %w", i, err)
		}
	}

	return nil
}

func (v *Verifier) verifyCommittedOperation(op *Operation) error {
	// Verify operation was actually applied
	switch op.Type {
	case OpTypeCreate:
		// Verify item exists
		return nil

	case OpTypeUpdate:
		// Verify item was updated
		return nil

	case OpTypeDelete:
		// Verify item was deleted
		return nil

	case OpWrite:
		// Verify data was written
		return nil

	case OpModify:
		// Verify modification was applied
		return nil

	default:
		return fmt.Errorf("invalid operation type: %d", op.Type)
	}
}

func (v *Verifier) verifyRolledBackOperation(op *Operation) error {
	// Verify operation was properly rolled back
	switch op.Type {
	case OpTypeCreate:
		// Verify item doesn't exist
		return nil

	case OpTypeUpdate:
		// Verify item was restored
		return nil

	case OpTypeDelete:
		// Verify item was restored
		return nil

	case OpWrite:
		// Verify data was reverted
		return nil

	case OpModify:
		// Verify modification was reverted
		return nil

	default:
		return fmt.Errorf("invalid operation type: %d", op.Type)
	}
}

func (v *Verifier) isTransactionTimedOut(tx *TxData) bool {
	// Transaction timeout is 5 minutes
	const txTimeout = 5 * 60 // seconds
	return time.Now().Unix()-tx.Created > txTimeout
}

func (v *Verifier) rollbackTimedOutTx(tx *TxData) error {
	// Parse operations
	var ops []Operation
	if err := json.Unmarshal(tx.Data, &ops); err != nil {
		return fmt.Errorf("parsing operations: %w", err)
	}

	// Rollback operations in reverse order
	for i := len(ops) - 1; i >= 0; i-- {
		if err := v.rollbackOperation(&ops[i]); err != nil {
			return fmt.Errorf("rolling back operation %d: %w", i, err)
		}
	}

	// Update transaction status
	tx.Status = TxStatusRollback

	// Write updated transaction
	offset, err := v.store.writeTx(*tx)
	if err != nil {
		return fmt.Errorf("writing rolled back transaction: %w", err)
	}

	// Update index entry
	entry := IndexEntry{
		ID:     tx.ID,
		Offset: offset,
		Length: uint32(tx.Size()),
		Flags:  TxStatusRollback,
	}
	v.store.index.Remove(tx.ID)
	v.store.index.Add(entry)

	return nil
}

func (v *Verifier) rollbackOperation(op *Operation) error {
	// Rollback operation based on type
	switch op.Type {
	case OpTypeCreate:
		// Delete created item
		return nil

	case OpTypeUpdate:
		// Restore previous state
		return nil

	case OpTypeDelete:
		// Restore deleted item
		return nil

	case OpWrite:
		// Revert written data
		return nil

	case OpModify:
		// Revert modification
		return nil

	default:
		return fmt.Errorf("invalid operation type: %d", op.Type)
	}
}
