# Storage Implementation

This document details how Memex implements its storage system using a single .mx file format.

## File Format Overview

The .mx file is structured as:

```
[Header]     Fixed 128 bytes
[Chunks]     Content storage
[Nodes]      Node metadata
[Edges]      Link data
[Indexes]    Lookup tables
```

### Header (128 bytes)

```go
type Header struct {
    Magic     [7]byte   // "MEMEX01"
    Version   uint8     // Format version
    Created   time.Time // Creation timestamp
    Modified  time.Time // Last modified timestamp
    NodeCount uint32    // Number of nodes
    EdgeCount uint32    // Number of edges
    NodeIndex uint64    // Offset to node index
    EdgeIndex uint64    // Offset to edge index
    Reserved  [64]byte  // Future use
}
```

## Content Storage (Chunks)

Content is stored in content-addressable chunks:

1. Content is split into chunks:
   - Small content (≤1024 bytes): Split on word boundaries
   - Large content (>1024 bytes): Split into fixed 512-byte chunks
2. Each chunk is hashed using SHA-256
3. Hash becomes the chunk's identifier
4. Chunks are stored with reference counting
5. Automatic deduplication through hash identity

```go
// Writing content:
1. Split content into chunks
2. For each chunk:
   - Calculate SHA-256 hash
   - Check if hash exists (reference counting)
   - If not exists:
     * Write chunk content
     * Initialize reference count
   - If exists:
     * Increment reference count
3. Store chunk list in node metadata
4. Calculate content hash from chunk hashes
```

## Node Storage

Nodes represent files and notes:

```go
type NodeData struct {
    ID       [32]byte // Node identifier
    Type     [32]byte // Node type (file/note)
    Created  int64    // Unix timestamp
    Modified int64    // Unix timestamp
    MetaLen  uint32   // Metadata length
    Meta     []byte   // JSON metadata with content hash and chunk list
}
```

Node writing process:
1. Split content into chunks
2. Store each chunk with reference counting
3. Generate node ID
4. Create metadata with content hash and chunk list
5. Write node data
6. Add to node index

## Edge Storage (Links)

Edges represent relationships:

```go
type EdgeData struct {
    Source   [32]byte // Source node ID
    Target   [32]byte // Target node ID
    Type     [32]byte // Link type
    Created  int64    // Unix timestamp
    Modified int64    // Unix timestamp
    MetaLen  uint32   // Metadata length
    Meta     []byte   // JSON metadata
}
```

Edge writing process:
1. Verify source and target exist
2. Write edge data
3. Add to edge index

## Index System

Two index types for efficient lookup:

```go
type IndexEntry struct {
    ID     [32]byte // Hash/ID
    Offset uint64   // File offset
    Length uint32   // Entry length
}
```

1. **Node Index**: Maps node IDs to node data
2. **Edge Index**: Maps source IDs to edge data

## Implementation Files

### store.go
- Main storage implementation
- Header management
- Index handling
- File operations

```go
type MXStore struct {
    path   string       // Path to .mx file
    file   *os.File     // File handle
    header Header       // File header
    nodes  []IndexEntry // Node index
    edges  []IndexEntry // Edge index
    chunks *ChunkStore  // Chunk storage
}
```

Key methods:
- `CreateMX`: Creates new repository
- `OpenMX`: Opens existing repository
- `Close`: Writes indexes and closes file

### chunk.go
- Content chunking implementation
- Chunk storage with reference counting
- Content deduplication

Key methods:
- `ChunkContent`: Splits content into chunks
- `Store`: Stores chunk with reference counting
- `Get`: Retrieves chunk content
- `Delete`: Decrements reference count and removes if zero

### node.go
- Node operations
- Metadata handling
- Node data serialization

Key methods:
- `AddNode`: Creates new node
- `GetNode`: Retrieves node by ID
- `DeleteNode`: Removes node and its edges
- `ReconstructContent`: Rebuilds content from chunks

### link.go
- Edge operations
- Relationship management
- Link metadata handling

Key methods:
- `AddLink`: Creates new edge
- `GetLinks`: Gets edges for node
- `DeleteLink`: Removes edge

### index.go
- Index management
- Lookup operations
- Index serialization

## Reading Flow

1. Open file
2. Read header
3. Load indexes
4. Use indexes for lookups
5. Seek to data using offsets

## Writing Flow

1. Write data at end of file
2. Update relevant index
3. Update header counts
4. On close:
   - Write updated indexes
   - Update header offsets
   - Sync to disk

## Error Handling

- File corruption detection via magic number
- Index validation on load
- Transaction-like updates:
  1. Write new data
  2. Update indexes
  3. Update header
  4. Sync to disk

## Performance Considerations

- Indexes kept in memory
- Sequential writes for data
- Content deduplication through chunks
- Reference counting for chunk cleanup
- No file fragmentation handling (yet)
- No compaction (yet)

## Future Improvements

1. **Compaction**
   - Remove unreferenced chunks
   - Rewrite file without gaps
   - Update all offsets

2. **Caching**
   - Cache frequently accessed chunks
   - Cache node metadata
   - LRU cache implementation

3. **Transactions**
   - Proper atomic updates
   - Rollback capability
   - Write-ahead logging

4. **Optimization**
   - Index compression
   - Content compression
   - Batch operations
   - Smarter chunking algorithms
