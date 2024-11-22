# Storage Implementation

This document details how Memex implements its storage system using a single .mx file format.

## File Format Overview

The .mx file is structured as:

```
[Header]     Fixed 128 bytes
[Blobs]      Content storage
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
    BlobCount uint32    // Number of content blobs
    NodeIndex uint64    // Offset to node index
    EdgeIndex uint64    // Offset to edge index
    BlobIndex uint64    // Offset to blob index
    Reserved  [64]byte  // Future use
}
```

## Content Storage (Blobs)

Content is stored in content-addressable blobs:

1. Content is hashed using SHA-256
2. Hash becomes the blob's identifier
3. Blob is stored with length prefix
4. Automatic deduplication through hash identity

```go
// Writing a blob:
1. Calculate SHA-256 hash
2. Check if hash exists in blob index
3. If not exists:
   - Write uint32 length
   - Write content bytes
   - Add entry to blob index
4. Return hash as identifier
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
    Meta     []byte   // JSON metadata
}
```

Node writing process:
1. Store content as blob
2. Generate node ID
3. Create metadata with content hash
4. Write node data
5. Add to node index

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

Three index types for efficient lookup:

```go
type IndexEntry struct {
    ID     [32]byte // Hash/ID
    Offset uint64   // File offset
    Length uint32   // Entry length
}
```

1. **Node Index**: Maps node IDs to node data
2. **Edge Index**: Maps source IDs to edge data
3. **Blob Index**: Maps content hashes to blobs

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
    blobs  []IndexEntry // Blob index
}
```

Key methods:
- `CreateMX`: Creates new repository
- `OpenMX`: Opens existing repository
- `Close`: Writes indexes and closes file

### blob.go
- Content storage implementation
- Blob reading/writing
- Content deduplication

Key methods:
- `storeBlob`: Stores content, returns hash
- `LoadBlob`: Retrieves content by hash

### node.go
- Node operations
- Metadata handling
- Node data serialization

Key methods:
- `AddNode`: Creates new node
- `GetNode`: Retrieves node by ID
- `DeleteNode`: Removes node and its edges

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
- Content deduplication
- No file fragmentation handling (yet)
- No compaction (yet)

## Future Improvements

1. **Compaction**
   - Remove deleted content
   - Rewrite file without gaps
   - Update all offsets

2. **Caching**
   - Cache frequently accessed blobs
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
