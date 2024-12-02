package tx

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// TxData represents a transaction's binary format
type TxData struct {
	ID      [32]byte // SHA-256 hash identifier
	Type    uint32   // Transaction type
	Created int64    // Unix timestamp
	Status  uint32   // Transaction status
	MetaLen uint32   // Length of metadata JSON
	Meta    []byte   // Metadata as JSON
	DataLen uint32   // Length of operation data
	Data    []byte   // Operation data
}

// Operation represents a transaction operation
type Operation struct {
	Type     uint32         `json:"type"`     // Operation type
	Target   string         `json:"target"`   // Target ID (node/link/chunk)
	Action   string         `json:"action"`   // Action to perform
	Data     []byte         `json:"data"`     // Operation data
	Meta     map[string]any `json:"meta"`     // Operation metadata
	Checksum uint32         `json:"checksum"` // Data checksum
}

// IndexEntry represents a transaction index entry
type IndexEntry struct {
	ID     [32]byte // SHA-256 hash identifier
	Offset uint64   // File offset to data
	Length uint32   // Length of data
	Flags  uint32   // Entry flags
}

// LogEntry represents a transaction log entry
type LogEntry struct {
	ID        [32]byte // Entry ID
	Type      uint32   // Operation type
	Status    uint32   // Transaction status
	Timestamp int64    // Unix timestamp
	DataLen   uint32   // Length of operation data
	Data      []byte   // Operation data
	Checksum  uint32   // Data checksum
	Offset    uint64   // File offset of affected data
	Length    uint32   // Length of affected data
}

// Constants for transaction types
const (
	TxTypeNone    uint32 = 0
	TxTypeNode    uint32 = 1 // Node operations
	TxTypeLink    uint32 = 2 // Link operations
	TxTypeChunk   uint32 = 3 // Chunk operations
	TxTypeComplex uint32 = 4 // Multiple operations
)

// Constants for transaction status
const (
	TxStatusPending   uint32 = 0 // Transaction is pending
	TxStatusCommitted uint32 = 1 // Transaction is committed
	TxStatusRollback  uint32 = 2 // Transaction is rolled back
	TxStatusFailed    uint32 = 3 // Transaction failed
)

// Constants for operation types
const (
	OpTypeNone   uint32 = 0
	OpTypeCreate uint32 = 1
	OpTypeUpdate uint32 = 2
	OpTypeDelete uint32 = 3
	OpWrite      uint32 = 4 // Write operation
	OpModify     uint32 = 5 // Modify operation
)

// Constants for size limits
const (
	MaxMetaSize = 4096    // Maximum metadata size in bytes (4KB)
	MaxDataSize = 1 << 20 // Maximum data size in bytes (1MB)
)

// Size returns the size of TxData in bytes
func (t *TxData) Size() int {
	return 32 + // ID
		4 + // Type
		8 + // Created
		4 + // Status
		4 + // MetaLen
		len(t.Meta) + // Meta
		4 + // DataLen
		len(t.Data) // Data
}

// CreateID generates a transaction ID from its data
func (t *TxData) CreateID() error {
	// Create ID from type, timestamp, and data
	data := make([]byte, 0, 16+len(t.Data))
	data = append(data, byte(t.Type))
	data = append(data, byte(t.Created))
	data = append(data, t.Data...)

	// Calculate hash
	t.ID = sha256.Sum256(data)
	return nil
}

// Marshal converts an Operation to JSON bytes
func (o *Operation) Marshal() ([]byte, error) {
	return json.Marshal(o)
}

// Unmarshal converts JSON bytes to an Operation
func (o *Operation) Unmarshal(data []byte) error {
	return json.Unmarshal(data, o)
}

// Validate validates an Operation
func (o *Operation) Validate() error {
	// Check operation type
	if o.Type == OpTypeNone {
		return fmt.Errorf("invalid operation type")
	}

	// Check target ID
	if o.Target == "" {
		return fmt.Errorf("missing target ID")
	}

	// Check action
	if o.Action == "" {
		return fmt.Errorf("missing action")
	}

	// Check data size
	if len(o.Data) > MaxDataSize {
		return fmt.Errorf("data too large: %d bytes (max %d)", len(o.Data), MaxDataSize)
	}

	// Check metadata size
	if o.Meta != nil {
		metaBytes, err := json.Marshal(o.Meta)
		if err != nil {
			return fmt.Errorf("marshaling metadata: %w", err)
		}
		if len(metaBytes) > MaxMetaSize {
			return fmt.Errorf("metadata too large: %d bytes (max %d)", len(metaBytes), MaxMetaSize)
		}
	}

	return nil
}

// NewOperation creates a new operation
func NewOperation(opType uint32, target string, action string, data []byte, meta map[string]any) (*Operation, error) {
	op := &Operation{
		Type:   opType,
		Target: target,
		Action: action,
		Data:   data,
		Meta:   meta,
	}

	if err := op.Validate(); err != nil {
		return nil, fmt.Errorf("validating operation: %w", err)
	}

	return op, nil
}

// Size returns the size of LogEntry in bytes
func (l *LogEntry) Size() int {
	return 32 + // ID
		4 + // Type
		4 + // Status
		8 + // Timestamp
		4 + // DataLen
		len(l.Data) + // Data
		4 + // Checksum
		8 + // Offset
		4 // Length
}

// CreateID generates a log entry ID from its data
func (l *LogEntry) CreateID() error {
	// Create ID from type, timestamp, and data
	data := make([]byte, 0, 16+len(l.Data))
	data = append(data, byte(l.Type))
	data = append(data, byte(l.Timestamp))
	data = append(data, l.Data...)

	// Calculate hash
	l.ID = sha256.Sum256(data)
	return nil
}
