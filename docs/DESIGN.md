# Memex Design Document

This document outlines the key design decisions and architectural choices made in the Memex project.

## Core Design Principles

1. **Directed Acyclic Graph (DAG)**
   - Content organized as nodes in a DAG
   - Nodes connected by typed, directional links
   - Supports versioning and relationships
   - Prevents cycles in relationships

2. **Content-Addressable Chunked Storage**
   - Content split into chunks for efficient storage
   - Each chunk identified by SHA-256 hash
   - Reference counting for chunk management
   - Enables deduplication and sharing between files
   - Similar content detection through shared chunks

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
- Content chunks (content-addressable)
- Node data (DAG nodes)
- Edge data (DAG edges)
- Index for efficient lookup

### DAG Structure
- **Nodes**
  - Unique ID
  - Type (file/note)
  - Metadata
  - Content hash
  - Chunk references
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
- Content split into chunks:
  - Small content (≤1024 bytes): Word-based chunks
  - Large content (>1024 bytes): Fixed-size chunks
- Each chunk identified by SHA-256 hash
- Reference counting for chunk management
- Automatic content deduplication
- Similar content detection through shared chunks
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
  - Chunk storage with reference counting
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
- Content chunks with reference counting
- Node and edge data with metadata
- Index for efficient lookup

### DAG Operations
- Add nodes (files/notes)
- Create edges (links)
- Traverse relationships
- Track versions
- Maintain acyclic property
- Delete nodes and edges
- Manage chunk references

## Future Considerations

1. **Scalability**
   - Split large DAGs
   - Efficient graph traversal
   - Improved indexing
   - Optimized chunk sizes

2. **Extended Features**
   - Advanced graph queries
   - Relationship types
   - Graph visualization
   - Export/import
   - Similarity detection tuning

3. **Performance Optimizations**
   - Caching frequently accessed chunks
   - Batch operations
   - Compression
   - Smarter chunking algorithms

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
- **Decision**: Content-addressable chunks with reference counting
- **Rationale**: Deduplication, sharing, similarity detection
- **Trade-offs**: More complex chunk management
- **Benefits**: 
  * Space efficiency through shared chunks
  * Natural similarity detection
  * Flexible content reconstruction
