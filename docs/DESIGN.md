# Memex Design Document

This document outlines the key design decisions and architectural choices made in the Memex project.

## Core Design Principles

1. **Content-Addressable Storage**
   - Content is stored as blobs identified by SHA-256 hash
   - Ensures data integrity and natural deduplication
   - Similar to Git's object storage model

2. **Flexible Organization**
   - No enforced hierarchy
   - Content can be organized through arbitrary links
   - Multiple independent graphs possible

3. **Simple Storage Model**
   - Single .mx file contains all data
   - Keeps implementation simple and reliable
   - Easy to backup and manage

## Storage Structure

```
example.mx          # Single file containing all data
```

The .mx file format:
- Fixed-size header with metadata
- Content blobs
- Node metadata
- Link information
- Index for efficient lookup

### Content Storage
- Content is stored as blobs
- Each blob is identified by its SHA-256 hash
- Content is deduplicated automatically
- Blobs are immutable

### Node Storage
- Nodes represent files or notes
- Each node has:
  - Unique ID
  - Type (file/note)
  - Metadata (filename, timestamps, etc)
  - Content reference (blob hash)

### Link System
- Links connect nodes
- Each link has:
  - Source node ID
  - Target node ID
  - Link type
  - Optional metadata

## Component Architecture

### CLI Tool (memex)
- Provides command-line interface
- Commands:
  - init: Create new repository
  - add: Add files
  - status: Show repository state
  - link: Create links between nodes
  - links: Show node links
  - delete: Remove nodes

### Web Server (memexd)
- Provides web interface and HTTP API
- Features:
  - View repository contents
  - Upload files
  - Create links
  - Search content
  - Delete nodes
- Embedded static files and templates
- RESTful API endpoints

### Storage Layer
- MXStore: Main storage implementation
  - Header management
  - Node operations
  - Link operations
  - Blob storage
  - Index management

### Core Types
```go
// Node represents a file or note
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

// Link represents a relationship
type Link struct {
    Source string
    Target string
    Type   string
    Meta   map[string]any
}

// IndexEntry for efficient lookup
type IndexEntry struct {
    ID     [32]byte
    Offset uint64
    Length uint32
}
```

## Implementation Details

### File Format
- Magic number for identification
- Version number for format changes
- Fixed-size header with counts and offsets
- Content blobs stored sequentially
- Node and link data with metadata
- Index for efficient lookup

### Operations
- Add nodes (files/notes)
- Get node by ID
- Create links between nodes
- List nodes by type
- Delete nodes and links
- Automatic content deduplication

### Web Interface
- HTML templates for rendering
- Static file serving
- File upload handling
- Search functionality
- Link management
- RESTful API endpoints

## Future Considerations

1. **Scalability**
   - Split large repositories
   - Efficient handling of large files
   - Improved indexing

2. **Extended Features**
   - Content versioning
   - Advanced search capabilities
   - Content type handlers
   - Export/import

3. **Performance Optimizations**
   - Caching frequently accessed data
   - Batch operations
   - Compression

## Design Decisions Log

### File Format
- **Decision**: Single .mx file instead of directory structure
- **Rationale**: Simpler to manage, move, and backup
- **Trade-offs**: Need to manage file size and fragmentation

### Content Storage
- **Decision**: Content-addressable blobs with SHA-256
- **Rationale**: Automatic deduplication, integrity checking
- **Trade-offs**: Larger hash size, but worth it for reliability

### Link System
- **Decision**: Direct node-to-node links with metadata
- **Rationale**: Flexible relationships, rich metadata
- **Trade-offs**: More complex querying, but more powerful

### Web Interface
- **Decision**: Embedded templates and static files
- **Rationale**: Self-contained binary, easy deployment
- **Trade-offs**: Less flexibility in customizing UI
