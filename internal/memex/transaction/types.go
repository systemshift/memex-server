package transaction

import (
	"crypto/sha256"
	"encoding/json"
	"time"
)

// ActionType represents different types of graph modifications
type ActionType string

const (
	ActionAddNode    ActionType = "AddNode"
	ActionDeleteNode ActionType = "DeleteNode"
	ActionAddLink    ActionType = "AddLink"
	ActionDeleteLink ActionType = "DeleteLink"
)

// Action represents a single graph modification
type Action struct {
	Type      ActionType     `json:"type"`
	Payload   map[string]any `json:"payload"` // Action-specific data
	Timestamp time.Time      `json:"timestamp"`
	PrevHash  [32]byte       `json:"prev_hash"`  // Hash of previous transaction
	StateHash [32]byte       `json:"state_hash"` // Hash of affected nodes/edges after action
}

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
