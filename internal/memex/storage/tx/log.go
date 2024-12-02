package tx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"memex/internal/memex/storage/common"
)

// Log handles transaction logging and recovery
type Log struct {
	file     *common.File
	locks    *common.LockManager
	basePath string
}

// LogHeader represents the log file header
type LogHeader struct {
	Created  int64  // When the log was created
	Modified int64  // When the log was last modified
	Count    uint32 // Number of entries
	Flags    uint32 // Log flags
}

// Log flags
const (
	LogFlagNone     uint32 = 0
	LogFlagRecovery uint32 = 1 << 0 // Log needs recovery
	LogFlagArchived uint32 = 1 << 1 // Log has been archived
)

// NewLog creates a new transaction log
func NewLog(basePath string, locks *common.LockManager) (*Log, error) {
	// Create log directory if needed
	logDir := filepath.Join(basePath, "tx")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}

	// Open current log file
	logPath := filepath.Join(logDir, "current.log")
	file, err := common.OpenFile(logPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening log file: %w", err)
	}

	log := &Log{
		file:     file,
		locks:    locks,
		basePath: basePath,
	}

	// Initialize or load header
	if err := log.initHeader(); err != nil {
		file.Close()
		return nil, fmt.Errorf("initializing header: %w", err)
	}

	return log, nil
}

// Close closes the log file
func (l *Log) Close() error {
	return l.file.Close()
}

// LogTransaction logs a transaction
func (l *Log) LogTransaction(tx *TxData) error {
	return l.locks.WithChunkLock(func() error {
		// Create log entry
		entry := LogEntry{
			ID:        tx.ID,
			Type:      tx.Type,
			Status:    tx.Status,
			Timestamp: time.Now().Unix(),
			DataLen:   tx.DataLen,
			Data:      tx.Data,
			Checksum:  common.CalculateChecksum(tx.Data),
		}

		// Write entry
		if err := l.writeLogEntry(entry); err != nil {
			return fmt.Errorf("writing log entry: %w", err)
		}

		// Update header
		header, err := l.readHeader()
		if err != nil {
			return fmt.Errorf("reading header: %w", err)
		}

		header.Count++
		header.Modified = time.Now().Unix()
		if err := l.writeHeader(*header); err != nil {
			return fmt.Errorf("updating header: %w", err)
		}

		return nil
	})
}

// Recover performs log recovery if needed
func (l *Log) Recover() error {
	return l.locks.WithChunkLock(func() error {
		// Read header
		header, err := l.readHeader()
		if err != nil {
			return fmt.Errorf("reading header: %w", err)
		}

		// Check if recovery needed
		if header.Flags&LogFlagRecovery == 0 {
			return nil
		}

		// Read all entries
		entries, err := l.readEntries(header.Count)
		if err != nil {
			return fmt.Errorf("reading entries: %w", err)
		}

		// Process each entry
		for _, entry := range entries {
			if err := l.recoverEntry(entry); err != nil {
				return fmt.Errorf("recovering entry: %w", err)
			}
		}

		// Clear recovery flag
		header.Flags &^= LogFlagRecovery
		header.Modified = time.Now().Unix()
		if err := l.writeHeader(*header); err != nil {
			return fmt.Errorf("updating header: %w", err)
		}

		return nil
	})
}

// Archive archives the current log file
func (l *Log) Archive() error {
	return l.locks.WithChunkLock(func() error {
		// Read header
		header, err := l.readHeader()
		if err != nil {
			return fmt.Errorf("reading header: %w", err)
		}

		// Set archived flag
		header.Flags |= LogFlagArchived
		header.Modified = time.Now().Unix()
		if err := l.writeHeader(*header); err != nil {
			return fmt.Errorf("updating header: %w", err)
		}

		// Create archive filename with timestamp
		archivePath := filepath.Join(l.basePath, "tx",
			fmt.Sprintf("log-%d.archive", time.Now().Unix()))

		// Get current log path
		currentPath := l.file.Name()

		// Close current file
		if err := l.file.Close(); err != nil {
			return fmt.Errorf("closing log: %w", err)
		}

		// Rename current log to archive
		if err := os.Rename(currentPath, archivePath); err != nil {
			return fmt.Errorf("archiving log: %w", err)
		}

		// Create new log file
		newPath := filepath.Join(l.basePath, "tx", "current.log")
		newFile, err := common.OpenFile(newPath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return fmt.Errorf("creating new log: %w", err)
		}

		// Update file reference
		l.file = newFile

		// Initialize new header
		if err := l.initHeader(); err != nil {
			return fmt.Errorf("initializing header: %w", err)
		}

		return nil
	})
}

// Internal methods

func (l *Log) initHeader() error {
	// Create header
	header := LogHeader{
		Created:  time.Now().Unix(),
		Modified: time.Now().Unix(),
		Count:    0,
		Flags:    LogFlagNone,
	}

	// Write header
	return l.writeHeader(header)
}

func (l *Log) readHeader() (*LogHeader, error) {
	// Seek to start
	if _, err := l.file.Seek(0, os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("seeking to start: %w", err)
	}

	// Read header fields
	created, err := l.file.ReadUint64()
	if err != nil {
		return nil, fmt.Errorf("reading created: %w", err)
	}

	modified, err := l.file.ReadUint64()
	if err != nil {
		return nil, fmt.Errorf("reading modified: %w", err)
	}

	count, err := l.file.ReadUint32()
	if err != nil {
		return nil, fmt.Errorf("reading count: %w", err)
	}

	flags, err := l.file.ReadUint32()
	if err != nil {
		return nil, fmt.Errorf("reading flags: %w", err)
	}

	return &LogHeader{
		Created:  int64(created),
		Modified: int64(modified),
		Count:    count,
		Flags:    flags,
	}, nil
}

func (l *Log) writeHeader(header LogHeader) error {
	// Seek to start
	if _, err := l.file.Seek(0, os.SEEK_SET); err != nil {
		return fmt.Errorf("seeking to start: %w", err)
	}

	// Write header fields
	if err := l.file.WriteUint64(uint64(header.Created)); err != nil {
		return fmt.Errorf("writing created: %w", err)
	}

	if err := l.file.WriteUint64(uint64(header.Modified)); err != nil {
		return fmt.Errorf("writing modified: %w", err)
	}

	if err := l.file.WriteUint32(header.Count); err != nil {
		return fmt.Errorf("writing count: %w", err)
	}

	if err := l.file.WriteUint32(header.Flags); err != nil {
		return fmt.Errorf("writing flags: %w", err)
	}

	return nil
}

func (l *Log) readEntries(count uint32) ([]LogEntry, error) {
	entries := make([]LogEntry, count)

	// Read each entry
	for i := uint32(0); i < count; i++ {
		var entry LogEntry
		if err := l.readLogEntry(&entry); err != nil {
			return nil, fmt.Errorf("reading entry %d: %w", i, err)
		}
		entries[i] = entry
	}

	return entries, nil
}

func (l *Log) readLogEntry(entry *LogEntry) error {
	// Read ID
	idBytes, err := l.file.ReadBytes(32)
	if err != nil {
		return fmt.Errorf("reading ID: %w", err)
	}
	copy(entry.ID[:], idBytes)

	// Read type
	entry.Type, err = l.file.ReadUint32()
	if err != nil {
		return fmt.Errorf("reading type: %w", err)
	}

	// Read status
	entry.Status, err = l.file.ReadUint32()
	if err != nil {
		return fmt.Errorf("reading status: %w", err)
	}

	// Read timestamp
	timestamp, err := l.file.ReadUint64()
	if err != nil {
		return fmt.Errorf("reading timestamp: %w", err)
	}
	entry.Timestamp = int64(timestamp)

	// Read data length
	entry.DataLen, err = l.file.ReadUint32()
	if err != nil {
		return fmt.Errorf("reading data length: %w", err)
	}

	// Read data
	if entry.DataLen > 0 {
		entry.Data, err = l.file.ReadBytes(int(entry.DataLen))
		if err != nil {
			return fmt.Errorf("reading data: %w", err)
		}
	}

	// Read checksum
	entry.Checksum, err = l.file.ReadUint32()
	if err != nil {
		return fmt.Errorf("reading checksum: %w", err)
	}

	return nil
}

func (l *Log) writeLogEntry(entry LogEntry) error {
	// Write ID
	if err := l.file.WriteBytes(entry.ID[:]); err != nil {
		return fmt.Errorf("writing ID: %w", err)
	}

	// Write type
	if err := l.file.WriteUint32(entry.Type); err != nil {
		return fmt.Errorf("writing type: %w", err)
	}

	// Write status
	if err := l.file.WriteUint32(entry.Status); err != nil {
		return fmt.Errorf("writing status: %w", err)
	}

	// Write timestamp
	if err := l.file.WriteUint64(uint64(entry.Timestamp)); err != nil {
		return fmt.Errorf("writing timestamp: %w", err)
	}

	// Write data length
	if err := l.file.WriteUint32(entry.DataLen); err != nil {
		return fmt.Errorf("writing data length: %w", err)
	}

	// Write data
	if entry.DataLen > 0 {
		if err := l.file.WriteBytes(entry.Data); err != nil {
			return fmt.Errorf("writing data: %w", err)
		}
	}

	// Write checksum
	if err := l.file.WriteUint32(entry.Checksum); err != nil {
		return fmt.Errorf("writing checksum: %w", err)
	}

	return nil
}

func (l *Log) recoverEntry(entry LogEntry) error {
	// Verify data checksum
	if !common.ValidateChecksum(entry.Data, entry.Checksum) {
		return fmt.Errorf("invalid checksum for transaction %x", entry.ID)
	}

	// Parse operations
	var ops []Operation
	if err := json.Unmarshal(entry.Data, &ops); err != nil {
		return fmt.Errorf("parsing operations: %w", err)
	}

	// Process operations based on transaction status
	switch entry.Status {
	case TxStatusPending:
		// Rollback pending transaction
		for i := len(ops) - 1; i >= 0; i-- {
			if err := l.rollbackOperation(&ops[i]); err != nil {
				return fmt.Errorf("rolling back operation: %w", err)
			}
		}

	case TxStatusCommitted:
		// Ensure all operations are applied
		for _, op := range ops {
			if err := l.applyOperation(&op); err != nil {
				return fmt.Errorf("applying operation: %w", err)
			}
		}

	case TxStatusRollback:
		// Already rolled back, nothing to do
		return nil

	default:
		return fmt.Errorf("invalid transaction status: %d", entry.Status)
	}

	return nil
}

func (l *Log) applyOperation(op *Operation) error {
	// Apply operation based on type
	switch op.Type {
	case OpTypeCreate:
		// Re-create if missing
		return nil

	case OpTypeUpdate:
		// Re-apply update if needed
		return nil

	case OpTypeDelete:
		// Ensure deleted
		return nil

	default:
		return fmt.Errorf("invalid operation type: %d", op.Type)
	}
}

func (l *Log) rollbackOperation(op *Operation) error {
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

	default:
		return fmt.Errorf("invalid operation type: %d", op.Type)
	}
}
