# Graph Migration Design

This document outlines the design for importing and exporting graphs or subgraphs in Memex.

## Export Format

### Archive Structure
```
graph-export/
├── manifest.json     # Export metadata
├── nodes/           # Node data
│   ├── node1.json
│   └── node2.json
├── edges/           # Edge data
│   ├── edge1.json
│   └── edge2.json
├── blobs/           # Content blobs
│   ├── hash1
│   └── hash2
└── index.json       # Node/edge mapping
```

### Manifest Format
```json
{
    "version": "1",
    "created": "2024-01-21T15:04:05Z",
    "nodes": 42,
    "edges": 23,
    "blobs": 15,
    "source": "repo-name.mx"
}
```

### Node Export Format
```json
{
    "id": "node-id",
    "type": "file",
    "meta": {
        "filename": "example.txt",
        "added": "2024-01-21T15:04:05Z"
    },
    "content": "hash1",
    "created": "2024-01-21T15:04:05Z",
    "modified": "2024-01-21T15:04:05Z"
}
```

### Edge Export Format
```json
{
    "source": "node-id-1",
    "target": "node-id-2",
    "type": "references",
    "meta": {
        "note": "Important connection"
    },
    "created": "2024-01-21T15:04:05Z",
    "modified": "2024-01-21T15:04:05Z"
}
```

## Implementation Plan

### 1. Export Implementation

#### Subgraph Selection
```go
type ExportOptions struct {
    Seeds []string // Starting node IDs
    Depth int     // How many edges to follow
    Query string  // Alternative selection method
}

func SelectSubgraph(store *MXStore, opts ExportOptions) ([]Node, []Edge, error) {
    // Start with seed nodes
    // BFS to specified depth
    // Include all relevant edges
    // Return selected nodes and edges
}
```

#### Content Export
```go
type Exporter struct {
    store  *MXStore
    writer *tar.Writer
}

func (e *Exporter) Export(nodes []Node, edges []Edge) error {
    // Write manifest
    // Export nodes
    // Export edges
    // Export referenced blobs
    // Create index
}
```

### 2. Import Implementation

#### Archive Reading
```go
type Importer struct {
    store   *MXStore
    reader  *tar.Reader
    mapping map[string]string // Old ID -> New ID
}

func (i *Importer) Import(opts ImportOptions) error {
    // Read manifest
    // Import blobs
    // Import nodes (generate new IDs)
    // Update edge references
    // Import edges
}
```

#### Conflict Resolution
```go
type ImportOptions struct {
    OnConflict ConflictStrategy
    Merge      bool
    Prefix     string // Namespace for imported nodes
}

type ConflictStrategy int

const (
    Skip ConflictStrategy = iota
    Replace
    Rename
)
```

### 3. CLI Commands

```bash
# Export Commands
memex export --nodes <id1,id2> --depth 2 --output graph.tar
memex export --query "type:note" --output notes.tar
memex export --all --output full.tar

# Import Commands
memex import graph.tar
memex import --merge --on-conflict=skip notes.tar
memex import --prefix="imported/" graph.tar
```

### 4. Implementation Phases

1. **Phase 1: Basic Export**
   - Export entire graph
   - Simple tar archive format
   - Basic manifest

2. **Phase 2: Subgraph Selection**
   - Implement node selection
   - BFS for related nodes
   - Edge selection

3. **Phase 3: Import**
   - Basic import functionality
   - New ID generation
   - Reference updating

4. **Phase 4: Advanced Features**
   - Conflict resolution
   - Merge strategies
   - Query-based export
   - Namespacing

## Considerations

### ID Handling
- Generate new IDs on import
- Maintain ID mapping for edge updates
- Handle circular references

### Content Deduplication
- Check for existing blobs
- Update references to existing content
- Handle modified content with same name

### Validation
- Verify graph integrity
- Check for missing references
- Validate content hashes

### Performance
- Stream large exports
- Batch imports
- Progress reporting

## Future Extensions

1. **Remote Import/Export**
   - HTTP(S) transport
   - Streaming support
   - Resume capability

2. **Differential Export**
   - Export only changes
   - Incremental updates
   - Change tracking

3. **Format Conversion**
   - Export to other graph formats
   - Import from other systems
   - Standard format support

4. **Collaboration**
   - Merge graphs from different sources
   - Conflict resolution strategies
   - Change history
