package transaction

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// ActionType represents different types of graph modifications
type ActionType string

const (
	// Original actions
	ActionAddNode    ActionType = "AddNode"
	ActionDeleteNode ActionType = "DeleteNode"
	ActionAddLink    ActionType = "AddLink"
	ActionDeleteLink ActionType = "DeleteLink"
	ActionModifyNode ActionType = "ModifyNode"
	ActionModifyLink ActionType = "ModifyLink"

	// Storage actions
	ActionPutContent    ActionType = "PutContent"    // Store new content
	ActionDeleteContent ActionType = "DeleteContent" // Delete content
)

// Action represents a single graph modification
type Action struct {
	Type      ActionType     `json:"type"`       // Type of action
	Payload   map[string]any `json:"payload"`    // Action-specific data
	Timestamp time.Time      `json:"timestamp"`  // When action occurred
	PrevHash  [32]byte       `json:"prev_hash"`  // Hash of previous action
	StateHash [32]byte       `json:"state_hash"` // Hash of affected nodes/edges after action
}

// Operation represents a specific operation within an action
type Operation struct {
	Type     uint32         `json:"type"`     // Operation type
	Target   string         `json:"target"`   // Target ID (node/link/chunk)
	Action   string         `json:"action"`   // Action to perform
	Data     []byte         `json:"data"`     // Operation data
	Meta     map[string]any `json:"meta"`     // Operation metadata
	Checksum uint32         `json:"checksum"` // Data checksum
}

// Constants for operation types
const (
	OpTypeNone   uint32 = 0
	OpTypeCreate uint32 = 1
	OpTypeUpdate uint32 = 2
	OpTypeDelete uint32 = 3
	OpTypeWrite  uint32 = 4
	OpTypeModify uint32 = 5
)

// Constants for size limits
const (
	MaxMetaSize = 4096    // Maximum metadata size in bytes (4KB)
	MaxDataSize = 1 << 20 // Maximum data size in bytes (1MB)
)

// Hash computes the cryptographic hash of the action
func (a *Action) Hash() ([32]byte, error) {
	// Marshal action to JSON for consistent hashing
	data, err := json.Marshal(struct {
		Type      ActionType     `json:"type"`
		Payload   map[string]any `json:"payload"`
		Timestamp time.Time      `json:"timestamp"`
		PrevHash  [32]byte       `json:"prev_hash"`
		StateHash [32]byte       `json:"state_hash"`
	}{
		Type:      a.Type,
		Payload:   a.Payload,
		Timestamp: a.Timestamp,
		PrevHash:  a.PrevHash,
		StateHash: a.StateHash,
	})
	if err != nil {
		return [32]byte{}, err
	}

	return sha256.Sum256(data), nil
}

// Verify checks if this action follows correctly from the previous one
func (a *Action) Verify(prevAction *Action) (bool, error) {
	if prevAction == nil {
		// This is the first action, only verify its own hash
		hash, err := a.Hash()
		if err != nil {
			return false, err
		}
		return hash == a.StateHash, nil
	}

	// Verify hash chain
	prevHash, err := prevAction.Hash()
	if err != nil {
		return false, err
	}

	// Previous action's hash should match this action's PrevHash
	return prevHash == a.PrevHash, nil
}

// ValidateOperation validates an operation
func (o *Operation) ValidateOperation() error {
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

	if err := op.ValidateOperation(); err != nil {
		return nil, fmt.Errorf("validating operation: %w", err)
	}

	return op, nil
}
