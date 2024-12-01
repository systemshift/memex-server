# Transaction System

The memex transaction system provides cryptographic verification of all graph modifications, ensuring the integrity and traceability of changes over time. This document explains the design, implementation, and usage of the transaction system.

## Overview

The transaction system maintains a verifiable history of all modifications to the graph database. Each modification (adding/removing nodes or links) is recorded as an action in a cryptographic hash chain, similar to Git's commit history.

### Key Features

- Cryptographic verification of all changes
- State consistency validation
- Temporal ordering guarantees
- Support for future branching/merging

## Design

### Action Chain

Each action in the chain contains:

```go
type Action struct {
    Type      ActionType          // What kind of modification
    Payload   map[string]any      // Action-specific data
    Timestamp time.Time           // When it occurred
    PrevHash  [32]byte           // Hash of previous action
    StateHash [32]byte           // Hash of affected state
}
```

The chain is formed by:
1. Each action containing the hash of the previous action
2. A state hash capturing the affected nodes/edges
3. Timestamps ensuring temporal ordering

### Storage Structure

The transaction system stores its data alongside the main .mx file:

```
my_graph.mx           # Main graph database
.actions/            # Transaction directory
  └── log           # Action log file
```

### Verification System

The system provides two levels of verification:

1. Hash Chain Verification
   - Ensures actions form an unbroken chain
   - Detects any tampering with history
   - Validates temporal ordering

2. State Verification
   - Ensures state hashes match actual graph state
   - Validates consistency of modifications
   - Enables state reconstruction

## Usage

### Basic Usage

```go
// Create stores
store, _ := storage.CreateMX("graph.mx")
actionStore, _ := transaction.NewActionStore(store)

// Record node addition
actionStore.RecordAction(transaction.ActionAddNode, map[string]any{
    "node_id": "123",
    "content": "Test content",
})

// Verify history
valid, _ := actionStore.VerifyHistory()
```

### Action Types

1. Node Actions:
   - `ActionAddNode`: Create a new node
   - `ActionDeleteNode`: Remove a node

2. Link Actions:
   - `ActionAddLink`: Create a link between nodes
   - `ActionDeleteLink`: Remove a link

### Verification

```go
// Get full history
history, _ := actionStore.GetHistory()

// Verify specific action
verifier := transaction.NewStateHasher()
err := verifier.VerifyStateConsistency(action, actionStore)
```

## Implementation Details

### Hash Chain

The hash chain is implemented using SHA-256:
1. Each action is serialized to JSON
2. The JSON is hashed to create the action hash
3. This hash becomes the PrevHash of the next action

### State Hashing

State hashes capture the affected parts of the graph:
1. For node actions: hash of the node and its content
2. For link actions: hash of the link and connected nodes

### Transaction Log

The transaction log is append-only and uses a binary format:
1. Length prefix (uint32)
2. JSON-encoded action data
3. Sequential ordering matches temporal ordering

## Future Extensions

The transaction system is designed to support:

### 1. Branching and Merging
- Create branches for experimental changes
- Merge changes from different branches
- Detect and resolve conflicts

### 2. Distributed Verification
- Verify changes across distributed copies
- Ensure consistency in distributed setups
- Support for partial history verification

### 3. Time Travel
- Roll back to previous states
- View graph at any point in history
- Compare states across time

### 4. Audit Trail
- Track who made what changes
- Understand graph evolution
- Support for compliance requirements

## Best Practices

1. Action Recording
   - Record one logical change per action
   - Include all relevant data in payload
   - Use appropriate action types

2. Verification
   - Verify history after significant operations
   - Implement regular verification in production
   - Handle verification failures gracefully

3. Performance
   - Consider action log size in long-running systems
   - Implement archival strategies for old actions
   - Use appropriate caching for verification

## Integration

The transaction system integrates with the existing storage system:

```go
// Example integration
type System struct {
    store       *storage.MXStore
    actionStore *transaction.ActionStore
}

func (s *System) AddNode(content []byte, meta map[string]any) error {
    // Add node to storage
    id, err := s.store.AddNode(content, "file", meta)
    if err != nil {
        return err
    }

    // Record action
    return s.actionStore.RecordAction(transaction.ActionAddNode, map[string]any{
        "node_id": id,
        "content": content,
        "meta":    meta,
    })
}
```

## Conclusion

The transaction system provides a robust foundation for tracking and verifying all modifications to the graph database. Its design supports future extensions while maintaining backward compatibility and performance.
