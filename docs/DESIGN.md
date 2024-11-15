# Memex Design Document

This document outlines the key design decisions and architectural choices made in the Memex project.

## Core Design Principles

1. **Content-Addressable Storage**
   - All content is stored using its hash as the identifier
   - Ensures data integrity and natural deduplication
   - Similar to Git's object storage model

2. **Flexible Organization**
   - No enforced hierarchy unlike Git's tree structure
   - Content can be organized through arbitrary relationships
   - Multiple independent graphs possible

3. **Simple Storage Model**
   - Uses filesystem directly instead of a database
   - Keeps implementation simple and reliable
   - Easy to backup and manage

## Storage Structure

```
.vault/
├── objects/          # Content storage
│   └── ab/          # First two chars of hash
│       └── cd1234   # Remaining hash chars
├── refs/            # Named references
└── trees/           # Structure definitions
```

### Content Storage
- Content is stored in content-addressable format
- Files are stored in a two-level directory structure
- First two characters of hash form the directory name
- Remaining characters form the file name

### Metadata Storage
- Metadata is stored separately from content
- Allows for flexible metadata updates without content changes
- Enables efficient metadata searches

### Link System
- Links are stored as separate entities
- Supports bidirectional relationships
- Can include metadata about the relationship
- Multiple link types supported

## Component Architecture

### CLI Tool (memex)
- Provides command-line interface
- Direct interaction with storage layer
- Focus on scripting and automation

### Web Server (memexd)
- Provides HTTP API and web interface
- Uses same storage layer as CLI
- Focuses on visualization and browsing

### Storage Layer
- Handles content storage and retrieval
- Manages metadata and links
- Provides atomic operations

### Core Types
- Defines fundamental data structures
- Ensures consistent data model
- Provides type safety

## Implementation Details

### Content Storage
```go
type Object struct {
    ID       string
    Type     string
    Content  []byte
    Metadata map[string]interface{}
}
```

### Link Storage
```go
type Link struct {
    Source   string
    Target   string
    Type     string
    Metadata map[string]interface{}
}
```

### Operations
- Add content
- Get content by ID
- Create links
- Query relationships
- Search content and metadata

## Future Considerations

1. **Scalability**
   - Potential sharding of content storage
   - Caching layer for frequently accessed content
   - Optimized metadata indexing

2. **Extended Features**
   - Content versioning
   - More sophisticated relationship types
   - Advanced querying capabilities
   - Collaborative features

3. **Performance Optimizations**
   - Bulk operations
   - Parallel processing
   - Improved caching

## Design Decisions Log

### Storage Backend
- **Decision**: Use filesystem instead of database
- **Rationale**: Simplicity, reliability, easy backup
- **Trade-offs**: Potentially slower for large datasets

### Content Addressing
- **Decision**: SHA-256 for content hashing
- **Rationale**: Strong collision resistance, widely used
- **Trade-offs**: Longer identifiers than necessary for small files

### Link System
- **Decision**: Store links as separate entities
- **Rationale**: Flexibility in relationship management
- **Trade-offs**: More complex querying
