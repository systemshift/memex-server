package tx

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"memex/internal/memex/storage/common"
)

// Store manages transactions in the repository
type Store struct {
	file  *common.File
	index *Index
	locks *common.LockManager
}

// NewStore creates a new transaction store
func NewStore(file *common.File, locks *common.LockManager) *Store {
	return &Store{
		file:  file,
		index: NewIndex(),
		locks: locks,
	}
}

// Begin starts a new transaction
func (s *Store) Begin(txType uint32, meta map[string]any) (string, error) {
	return s.locks.WithChunkLockString(func() (string, error) {
		// Create transaction data
		tx := TxData{
			Type:    txType,
			Created: time.Now().Unix(),
			Status:  TxStatusPending,
		}

		// Marshal metadata
		if meta != nil {
			metaBytes, err := json.Marshal(meta)
			if err != nil {
				return "", fmt.Errorf("marshaling metadata: %w", err)
			}

			if len(metaBytes) > MaxMetaSize {
				return "", fmt.Errorf("metadata too large (max %d bytes)", MaxMetaSize)
			}

			tx.MetaLen = uint32(len(metaBytes))
			tx.Meta = metaBytes
		}

		// Create ID
		if err := tx.CreateID(); err != nil {
			return "", fmt.Errorf("creating transaction ID: %w", err)
		}

		// Write transaction data
		offset, err := s.writeTx(tx)
		if err != nil {
			return "", fmt.Errorf("writing transaction: %w", err)
		}

		// Create index entry
		entry := IndexEntry{
			ID:     tx.ID,
			Offset: offset,
			Length: uint32(tx.Size()),
			Flags:  TxStatusPending,
		}

		// Add to index
		s.index.Add(entry)

		return fmt.Sprintf("%x", tx.ID), nil
	})
}

// AddOperation adds an operation to a transaction
func (s *Store) AddOperation(txID string, op *Operation) error {
	return s.locks.WithChunkLock(func() error {
		// Find transaction
		entry, found := s.index.FindByString(txID)
		if !found {
			return fmt.Errorf("transaction not found: %s", txID)
		}

		// Check transaction status
		if entry.Flags&0x0F != TxStatusPending {
			return fmt.Errorf("transaction is not pending")
		}

		// Read transaction data
		tx, err := s.readTx(entry)
		if err != nil {
			return fmt.Errorf("reading transaction: %w", err)
		}

		// Validate operation
		if err := op.Validate(); err != nil {
			return fmt.Errorf("validating operation: %w", err)
		}

		// Marshal operation
		opData, err := op.Marshal()
		if err != nil {
			return fmt.Errorf("marshaling operation: %w", err)
		}

		// Append operation to transaction data
		tx.Data = append(tx.Data, opData...)
		tx.DataLen = uint32(len(tx.Data))

		// Write updated transaction
		offset, err := s.writeTx(*tx)
		if err != nil {
			return fmt.Errorf("writing transaction: %w", err)
		}

		// Update index entry
		entry.Offset = offset
		entry.Length = uint32(tx.Size())
		s.index.Remove(tx.ID)
		s.index.Add(entry)

		return nil
	})
}

// Commit commits a transaction
func (s *Store) Commit(txID string) error {
	return s.locks.WithChunkLock(func() error {
		// Find transaction
		entry, found := s.index.FindByString(txID)
		if !found {
			return fmt.Errorf("transaction not found: %s", txID)
		}

		// Check transaction status
		if entry.Flags&0x0F != TxStatusPending {
			return fmt.Errorf("transaction is not pending")
		}

		// Update status
		if !s.index.UpdateStatus(entry.ID, TxStatusCommitted) {
			return fmt.Errorf("failed to update transaction status")
		}

		return nil
	})
}

// Rollback rolls back a transaction
func (s *Store) Rollback(txID string) error {
	return s.locks.WithChunkLock(func() error {
		// Find transaction
		entry, found := s.index.FindByString(txID)
		if !found {
			return fmt.Errorf("transaction not found: %s", txID)
		}

		// Check transaction status
		if entry.Flags&0x0F != TxStatusPending {
			return fmt.Errorf("transaction is not pending")
		}

		// Update status
		if !s.index.UpdateStatus(entry.ID, TxStatusRollback) {
			return fmt.Errorf("failed to update transaction status")
		}

		return nil
	})
}

// Get retrieves a transaction by ID
func (s *Store) Get(txID string) (*TxData, error) {
	var result *TxData
	err := s.locks.WithChunkLock(func() error {
		// Find transaction
		entry, found := s.index.FindByString(txID)
		if !found {
			return fmt.Errorf("transaction not found: %s", txID)
		}

		// Read transaction data
		tx, err := s.readTx(entry)
		if err != nil {
			return fmt.Errorf("reading transaction: %w", err)
		}

		result = tx
		return nil
	})
	return result, err
}

// LoadIndex loads the transaction index from the file
func (s *Store) LoadIndex(offset uint64, count uint32) error {
	return s.locks.WithChunkLock(func() error {
		return s.index.Load(s.file, offset, count)
	})
}

// SaveIndex saves the transaction index to the file
func (s *Store) SaveIndex(offset uint64) error {
	return s.locks.WithChunkLock(func() error {
		return s.index.Save(s.file, offset)
	})
}

// Internal methods

func (s *Store) writeTx(tx TxData) (uint64, error) {
	// Get current position
	pos, err := s.file.Seek(0, os.SEEK_END)
	if err != nil {
		return 0, fmt.Errorf("seeking to end: %w", err)
	}

	// Write transaction magic
	if err := s.file.WriteUint32(common.TxMagic); err != nil {
		return 0, fmt.Errorf("writing magic: %w", err)
	}

	// Write transaction ID
	if err := s.file.WriteBytes(tx.ID[:]); err != nil {
		return 0, fmt.Errorf("writing ID: %w", err)
	}

	// Write transaction type
	if err := s.file.WriteUint32(tx.Type); err != nil {
		return 0, fmt.Errorf("writing type: %w", err)
	}

	// Write timestamp
	if err := s.file.WriteUint64(uint64(tx.Created)); err != nil {
		return 0, fmt.Errorf("writing created: %w", err)
	}

	// Write status
	if err := s.file.WriteUint32(tx.Status); err != nil {
		return 0, fmt.Errorf("writing status: %w", err)
	}

	// Write metadata length
	if err := s.file.WriteUint32(tx.MetaLen); err != nil {
		return 0, fmt.Errorf("writing meta length: %w", err)
	}

	// Write metadata
	if tx.MetaLen > 0 {
		if err := s.file.WriteBytes(tx.Meta); err != nil {
			return 0, fmt.Errorf("writing metadata: %w", err)
		}
	}

	// Write data length
	if err := s.file.WriteUint32(tx.DataLen); err != nil {
		return 0, fmt.Errorf("writing data length: %w", err)
	}

	// Write data
	if tx.DataLen > 0 {
		if err := s.file.WriteBytes(tx.Data); err != nil {
			return 0, fmt.Errorf("writing data: %w", err)
		}
	}

	return uint64(pos), nil
}

func (s *Store) readTx(entry IndexEntry) (*TxData, error) {
	// Seek to transaction start
	if _, err := s.file.Seek(int64(entry.Offset), os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("seeking to transaction: %w", err)
	}

	// Read and verify magic
	magic, err := s.file.ReadUint32()
	if err != nil {
		return nil, fmt.Errorf("reading magic: %w", err)
	}
	if magic != common.TxMagic {
		return nil, fmt.Errorf("invalid transaction magic")
	}

	// Read transaction data
	tx := &TxData{}

	// Read transaction ID
	idBytes, err := s.file.ReadBytes(32)
	if err != nil {
		return nil, fmt.Errorf("reading ID: %w", err)
	}
	copy(tx.ID[:], idBytes)

	// Read transaction type
	tx.Type, err = s.file.ReadUint32()
	if err != nil {
		return nil, fmt.Errorf("reading type: %w", err)
	}

	// Read timestamp
	created, err := s.file.ReadUint64()
	if err != nil {
		return nil, fmt.Errorf("reading created: %w", err)
	}
	tx.Created = int64(created)

	// Read status
	tx.Status, err = s.file.ReadUint32()
	if err != nil {
		return nil, fmt.Errorf("reading status: %w", err)
	}

	// Read metadata length
	tx.MetaLen, err = s.file.ReadUint32()
	if err != nil {
		return nil, fmt.Errorf("reading meta length: %w", err)
	}

	// Read metadata
	if tx.MetaLen > 0 {
		tx.Meta, err = s.file.ReadBytes(int(tx.MetaLen))
		if err != nil {
			return nil, fmt.Errorf("reading metadata: %w", err)
		}
	}

	// Read data length
	tx.DataLen, err = s.file.ReadUint32()
	if err != nil {
		return nil, fmt.Errorf("reading data length: %w", err)
	}

	// Read data
	if tx.DataLen > 0 {
		tx.Data, err = s.file.ReadBytes(int(tx.DataLen))
		if err != nil {
			return nil, fmt.Errorf("reading data: %w", err)
		}
	}

	return tx, nil
}
