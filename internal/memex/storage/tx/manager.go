package tx

import (
	"crypto/sha256"
	"fmt"
	"os"
	"time"

	"memex/internal/memex/storage/common"
)

// Manager handles transactions and logging
type Manager struct {
	file  *common.File
	locks *common.LockManager
	log   []LogEntry
}

// NewManager creates a new transaction manager
func NewManager(file *common.File, locks *common.LockManager) *Manager {
	return &Manager{
		file:  file,
		locks: locks,
		log:   make([]LogEntry, 0),
	}
}

// Begin starts a new transaction
func (m *Manager) Begin() (*Transaction, error) {
	return &Transaction{
		manager: m,
		ops:     make([]LogEntry, 0),
	}, nil
}

// LoadLog loads the transaction log from the file
func (m *Manager) LoadLog(offset uint64, count uint32) error {
	return m.locks.WithChunkLock(func() error {
		// Seek to log start
		if _, err := m.file.Seek(int64(offset), os.SEEK_SET); err != nil {
			return fmt.Errorf("seeking to log: %w", err)
		}

		// Read log entries
		m.log = make([]LogEntry, count)
		for i := uint32(0); i < count; i++ {
			var entry LogEntry
			if err := m.readLogEntry(&entry); err != nil {
				return fmt.Errorf("reading log entry: %w", err)
			}
			m.log[i] = entry
		}

		return nil
	})
}

// SaveLog saves the transaction log to the file
func (m *Manager) SaveLog(offset uint64) error {
	return m.locks.WithChunkLock(func() error {
		// Seek to log start
		if _, err := m.file.Seek(int64(offset), os.SEEK_SET); err != nil {
			return fmt.Errorf("seeking to log: %w", err)
		}

		// Write log entries
		for _, entry := range m.log {
			if err := m.writeLogEntry(entry); err != nil {
				return fmt.Errorf("writing log entry: %w", err)
			}
		}

		return nil
	})
}

// Internal methods

func (m *Manager) readLogEntry(entry *LogEntry) error {
	// Read ID
	idBytes, err := m.file.ReadBytes(32)
	if err != nil {
		return fmt.Errorf("reading ID: %w", err)
	}
	copy(entry.ID[:], idBytes)

	// Read timestamp
	timestamp, err := m.file.ReadUint64()
	if err != nil {
		return fmt.Errorf("reading timestamp: %w", err)
	}
	entry.Timestamp = int64(timestamp)

	// Read type
	opType, err := m.file.ReadUint32()
	if err != nil {
		return fmt.Errorf("reading type: %w", err)
	}
	entry.Type = opType

	// Read status
	status, err := m.file.ReadUint32()
	if err != nil {
		return fmt.Errorf("reading status: %w", err)
	}
	entry.Status = status

	// Read data length
	dataLen, err := m.file.ReadUint32()
	if err != nil {
		return fmt.Errorf("reading data length: %w", err)
	}
	entry.DataLen = dataLen

	// Read data
	if entry.DataLen > 0 {
		entry.Data, err = m.file.ReadBytes(int(entry.DataLen))
		if err != nil {
			return fmt.Errorf("reading data: %w", err)
		}
	}

	// Read checksum
	checksum, err := m.file.ReadUint32()
	if err != nil {
		return fmt.Errorf("reading checksum: %w", err)
	}
	entry.Checksum = checksum

	return nil
}

func (m *Manager) writeLogEntry(entry LogEntry) error {
	// Write ID
	if err := m.file.WriteBytes(entry.ID[:]); err != nil {
		return fmt.Errorf("writing ID: %w", err)
	}

	// Write timestamp
	if err := m.file.WriteUint64(uint64(entry.Timestamp)); err != nil {
		return fmt.Errorf("writing timestamp: %w", err)
	}

	// Write type
	if err := m.file.WriteUint32(entry.Type); err != nil {
		return fmt.Errorf("writing type: %w", err)
	}

	// Write status
	if err := m.file.WriteUint32(entry.Status); err != nil {
		return fmt.Errorf("writing status: %w", err)
	}

	// Write data length
	if err := m.file.WriteUint32(entry.DataLen); err != nil {
		return fmt.Errorf("writing data length: %w", err)
	}

	// Write data
	if entry.DataLen > 0 {
		if err := m.file.WriteBytes(entry.Data); err != nil {
			return fmt.Errorf("writing data: %w", err)
		}
	}

	// Write checksum
	if err := m.file.WriteUint32(entry.Checksum); err != nil {
		return fmt.Errorf("writing checksum: %w", err)
	}

	return nil
}

// Transaction represents an atomic operation sequence
type Transaction struct {
	manager *Manager
	ops     []LogEntry
}

// Write records a write operation
func (tx *Transaction) Write(data []byte) error {
	// Create log entry
	entry := LogEntry{
		ID:        sha256.Sum256(data),
		Timestamp: time.Now().Unix(),
		Type:      OpWrite,
		Status:    TxStatusPending,
		DataLen:   uint32(len(data)),
		Data:      data,
		Checksum:  common.CalculateChecksum(data),
	}

	// Add to transaction ops
	tx.ops = append(tx.ops, entry)

	return nil
}

// Commit finalizes the transaction
func (tx *Transaction) Commit() error {
	// Update status for all ops
	for i := range tx.ops {
		tx.ops[i].Status = TxStatusCommitted
	}

	// Add all ops to log
	tx.manager.log = append(tx.manager.log, tx.ops...)

	// Clear transaction ops
	tx.ops = tx.ops[:0]

	return nil
}

// Rollback aborts the transaction
func (tx *Transaction) Rollback() error {
	// Update status for all ops
	for i := range tx.ops {
		tx.ops[i].Status = TxStatusRollback
	}

	// Add all ops to log
	tx.manager.log = append(tx.manager.log, tx.ops...)

	// Clear transaction ops
	tx.ops = tx.ops[:0]

	return nil
}

// Verify checks transaction integrity
func (tx *Transaction) Verify() error {
	for _, op := range tx.ops {
		// Verify data checksum
		if !common.ValidateChecksum(op.Data, op.Checksum) {
			return fmt.Errorf("checksum mismatch for operation %x", op.ID)
		}
	}

	return nil
}
