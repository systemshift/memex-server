package transaction

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// StateHasher calculates state hashes for verification
type StateHasher struct {
	// Add fields as needed for caching/optimization
}

// NewStateHasher creates a new state hasher
func NewStateHasher() *StateHasher {
	return &StateHasher{}
}

// CalculateStateHash computes a hash of the affected state after an action
func (sh *StateHasher) CalculateStateHash(action *Action, store *ActionStore) ([32]byte, error) {
	// Get affected nodes/edges based on action type
	affected, err := sh.getAffectedState(action, store)
	if err != nil {
		return [32]byte{}, fmt.Errorf("getting affected state: %w", err)
	}

	// Calculate combined hash of all affected elements
	return sh.hashState(affected)
}

// VerifyActionChain verifies a sequence of actions forms a valid chain
func (sh *StateHasher) VerifyActionChain(actions []*Action) error {
	if len(actions) == 0 {
		return nil
	}

	// Verify first action
	if !bytes.Equal(actions[0].PrevHash[:], make([]byte, 32)) {
		return fmt.Errorf("first action must have zero previous hash")
	}

	// Verify chain
	for i := 1; i < len(actions); i++ {
		prevHash, err := actions[i-1].Hash()
		if err != nil {
			return fmt.Errorf("calculating hash for action %d: %w", i-1, err)
		}

		if prevHash != actions[i].PrevHash {
			return fmt.Errorf("hash chain broken at action %d", i)
		}
	}

	return nil
}

// VerifyStateConsistency verifies that state hashes are consistent
func (sh *StateHasher) VerifyStateConsistency(action *Action, store *ActionStore) error {
	// Calculate expected state hash
	expectedHash, err := sh.CalculateStateHash(action, store)
	if err != nil {
		return fmt.Errorf("calculating state hash: %w", err)
	}

	// Compare with recorded hash
	if expectedHash != action.StateHash {
		return fmt.Errorf("state hash mismatch")
	}

	return nil
}

// Internal methods

// getAffectedState returns the state elements affected by an action
func (sh *StateHasher) getAffectedState(action *Action, store *ActionStore) (map[string]interface{}, error) {
	affected := make(map[string]interface{})

	switch action.Type {
	case ActionAddNode:
		// Get node ID from payload
		nodeID, ok := action.Payload["node_id"].(string)
		if !ok {
			return nil, fmt.Errorf("missing node_id in payload")
		}

		// Get node state
		node, err := store.store.GetNode(nodeID)
		if err != nil {
			return nil, fmt.Errorf("getting node state: %w", err)
		}

		affected["node:"+nodeID] = node

	case ActionAddLink:
		// Get source and target from payload
		source, ok := action.Payload["source"].(string)
		if !ok {
			return nil, fmt.Errorf("missing source in payload")
		}
		target, ok := action.Payload["target"].(string)
		if !ok {
			return nil, fmt.Errorf("missing target in payload")
		}

		// Get link state
		links, err := store.store.GetLinks(source)
		if err != nil {
			return nil, fmt.Errorf("getting link state: %w", err)
		}

		affected["link:"+source+":"+target] = links

	case ActionDeleteNode:
		// For deletions, we store the fact that the node doesn't exist
		nodeID, ok := action.Payload["node_id"].(string)
		if !ok {
			return nil, fmt.Errorf("missing node_id in payload")
		}
		affected["node:"+nodeID] = nil

	case ActionDeleteLink:
		// For deletions, we store the fact that the link doesn't exist
		source, ok := action.Payload["source"].(string)
		if !ok {
			return nil, fmt.Errorf("missing source in payload")
		}
		target, ok := action.Payload["target"].(string)
		if !ok {
			return nil, fmt.Errorf("missing target in payload")
		}
		affected["link:"+source+":"+target] = nil
	}

	return affected, nil
}

// hashState calculates a combined hash of multiple state elements
func (sh *StateHasher) hashState(state map[string]interface{}) ([32]byte, error) {
	// Sort keys for consistent ordering
	keys := make([]string, 0, len(state))
	for k := range state {
		keys = append(keys, k)
	}

	// Create ordered map for consistent hashing
	ordered := make(map[string]interface{})
	for _, k := range keys {
		ordered[k] = state[k]
	}

	// Marshal to JSON for consistent representation
	data, err := json.Marshal(ordered)
	if err != nil {
		return [32]byte{}, fmt.Errorf("marshaling state: %w", err)
	}

	return sha256.Sum256(data), nil
}
