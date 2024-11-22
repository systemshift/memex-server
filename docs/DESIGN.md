# Memex Design Document

This document outlines the key design decisions and architectural choices made in the Memex project.

## Core Design Principles

1. **Directed Acyclic Graph (DAG)**
   - Content organized as nodes in a DAG
   - Nodes connected by typed, directional links
   - Supports versioning and relationships
   - Prevents cycles in relationships

2. **Content-Addressable Storage**
   - Content stored as blobs identified by SHA-256 hash
   - Ensures data integrity and natural deduplication
   - Similar to Git's object storage model

3. **Single File Storage**
   - All data contained in one .mx file
   - Keeps implementation simple and reliable
   - Easy to backup and manage

## Storage Structure

```
example.mx          # Single file containing all data
```

The .mx file format:
- Fixed-size header with metadata
- Content blobs (content-addressable)
- Node data (DAG nodes)
- Edge data (DAG edges)
- Index for efficient lookup

### DAG Structure
- **Nodes**
  - Unique ID
  - Type (file/note)
  - Metadata
  - Content reference (blob hash)
  - Version history
  - Links to other nodes

- **Edges (Links)**
  - Source node ID
  - Target node ID
  - Link type
  - Optional metadata
  - Directional relationships

- **Versions**
  - Content hash
  - Chunk references
  - Creation time
  - Version metadata

### Content Storage
- Content stored as immutable blobs
- Each blob identified by SHA-256 hash
- Automatic content deduplication
- Version tracking per node

## Component Architecture

### CLI Tool (memex)
- Provides command-line interface
- Commands:
  - init: Create new repository
  - add: Add files to DAG
  - status: Show DAG state
  - link: Create edges between nodes
  - links: Show node connections
  - delete: Remove nodes

### Web Server (memexd)
- Provides web interface
- Features:
  - View DAG structure
  - Upload files as nodes
  - Create relationships
  - Search graph
  - Visualize connections

### Storage Layer
- MXStore: Main storage implementation
  - Header management
  - Node operations
  - Edge operations
  - Blob storage
  - Index management

### Core Types
```go
// Node in the DAG
type Node struct {
    ID       string
    Type     string
    Meta     map[string]any
    Created  time.Time
    Modified time.Time
    Versions []Version
    Links    []Link
    Current  string
}

// Edge in the DAG
type Link struct {
    Source      string
    Target      string
    Type        string
    Meta        map[string]any
    SourceChunk string
    TargetChunk string
}

// Version history
type Version struct {
    Hash      string
    Chunks    []string
    Created   time.Time
    Meta      map[string]any
    Available bool
}

// DAG root
type Root struct {
    Hash     string
    Modified time.Time
    Nodes    []string
}
```

## Implementation Details

### File Format
- Magic number for identification
- Version number for format changes
- Fixed-size header with counts and offsets
- Content blobs stored sequentially
- Node and edge data with metadata
- Index for efficient lookup

### DAG Operations
- Add nodes (files/notes)
- Create edges (links)
- Traverse relationships
- Track versions
- Maintain acyclic property
- Delete nodes and edges

## Future Considerations

1. **Scalability**
   - Split large DAGs
   - Efficient graph traversal
   - Improved indexing

2. **Extended Features**
   - Advanced graph queries
   - Relationship types
   - Graph visualization
   - Export/import

3. **Performance Optimizations**
   - Caching frequently accessed paths
   - Batch operations
   - Compression

## Design Decisions Log

### Graph Structure
- **Decision**: DAG with typed edges
- **Rationale**: Flexible relationships, version tracking
- **Trade-offs**: More complex than tree structure

### File Format
- **Decision**: Single .mx file with sections
- **Rationale**: Simple to manage and backup
- **Trade-offs**: Need to handle file size

### Content Storage
- **Decision**: Content-addressable blobs
- **Rationale**: Deduplication, integrity
- **Trade-offs**: Larger hash size
