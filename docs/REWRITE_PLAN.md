# Memex Rewrite Plan

This document outlines a comprehensive plan for rewriting the Memex codebase to improve modularity, extensibility, performance, and developer experience.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Module System Design](#module-system-design)
- [Storage System Improvements](#storage-system-improvements)
- [API Layer Design](#api-layer-design)
- [Client Interfaces](#client-interfaces)
- [Implementation Phases](#implementation-phases)
- [Migration Strategy](#migration-strategy)
- [Timeline and Milestones](#timeline-and-milestones)

## Architecture Overview

### Core Design Principles

1. **Strong Core with Extension Points**
   - Robust core that provides essential functionality out of the box
   - Extension through modules for specialized features
   - Clear boundaries between core and extensions

2. **Enhanced Content-Addressable Storage**
   - Improve the existing content-addressable approach for deduplication
   - Enhance indexing and caching for better performance
   - Optimize metadata storage and retrieval

3. **Graph-First Design**
   - Optimize for graph operations from the beginning
   - Use a proper graph data structure internally
   - Support advanced graph queries natively

4. **Simple API Approach**
   - Start with a clean, well-designed REST API
   - Focus on core operations with room to grow
   - CLI and web UI as clients of the API

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Client Interfaces                         │
│                                                                 │
│     ┌─────────────┐      ┌─────────────┐      ┌─────────────┐   │
│     │     CLI     │      │   Web UI    │      │ SDK/Library │   │
│     └─────────────┘      └─────────────┘      └─────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                           API Layer                             │
│                                                                 │
│                    ┌─────────────────────┐                      │
│                    │      REST API       │                      │
│                    └─────────────────────┘                      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                          Core Engine                            │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   Storage   │  │    Graph    │  │      Transaction        │  │
│  │   Manager   │  │    Engine   │  │        System           │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   Content   │  │    Query    │  │        Module           │  │
│  │   Indexer   │  │    Engine   │  │        Manager          │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Module System                           │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   Module    │  │   Command   │  │       Extension         │  │
│  │   Registry  │  │   Router    │  │         API             │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Directory Structure

```
memex/
├── cmd/                    # Command-line tools
│   ├── memex/             # CLI tool
│   └── memexd/            # Web server
├── internal/              # Internal packages
│   ├── core/              # Core engine
│   │   ├── storage/       # Storage system
│   │   ├── graph/         # Graph engine
│   │   ├── transaction/   # Transaction system
│   │   └── query/         # Query engine
│   ├── module/            # Module system
│   │   ├── registry/      # Module registry
│   │   └── installer/     # Module installer
│   └── api/               # API layer
│       └── rest/          # REST API
├── pkg/                   # Public API
│   ├── memex/             # Client library
│   └── module/            # Module SDK
└── web/                   # Web UI
    ├── src/               # React components
    └── public/            # Static assets
```

## Module System Design

The module system enables extensibility through well-defined extension points while keeping core functionality in the main tool.

### Module Interface

```go
// Module is the interface all modules must implement
type Module interface {
    // Core identity
    ID() string
    Name() string
    Description() string
    Version() string
    
    // Lifecycle
    Init(ctx context.Context, registry Registry) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    
    // Command handling
    Commands() []Command
    HandleCommand(ctx context.Context, cmd string, args []string) (interface{}, error)
    
    // Extension points
    Hooks() []Hook
    HandleHook(ctx context.Context, hook string, data interface{}) (interface{}, error)
}

// Command represents a module command
type Command struct {
    Name        string
    Description string
    Usage       string
    Args        []string
    Flags       []Flag
    Handler     CommandHandler
}

// Hook represents an extension point
type Hook struct {
    Name        string
    Description string
    Priority    int
}
```

### Module Registry

```go
// Registry manages module registration and discovery
type Registry interface {
    // Registration
    Register(module Module) error
    Unregister(moduleID string) error
    
    // Discovery
    GetModule(id string) (Module, error)
    ListModules() []ModuleInfo
    
    // Command routing
    RouteCommand(ctx context.Context, moduleID, cmd string, args []string) (interface{}, error)
    
    // Hook system
    RegisterHook(hook Hook) error
    TriggerHook(ctx context.Context, name string, data interface{}) ([]interface{}, error)
}
```

### Module Installation

```go
// Installer handles module installation
type Installer interface {
    // Installation sources
    InstallFromGit(ctx context.Context, url string, options InstallOptions) (string, error)
    InstallFromDirectory(ctx context.Context, path string, options InstallOptions) (string, error)
    InstallFromArchive(ctx context.Context, path string, options InstallOptions) (string, error)
    
    // Management
    Remove(ctx context.Context, moduleID string) error
    Update(ctx context.Context, moduleID string) error
    Enable(ctx context.Context, moduleID string) error
    Disable(ctx context.Context, moduleID string) error
}

// InstallOptions configures the installation process
type InstallOptions struct {
    Version     string            // Specific version to install
    BuildFlags  []string          // Flags to pass to the build process
    Environment map[string]string // Environment variables for build
    Force       bool              // Force reinstall if already exists
}
```

### Module Configuration

Module configuration will be stored in a JSON file:

```json
{
  "modules": {
    "ast": {
      "path": "/path/to/installed/memex-ast",
      "enabled": true,
      "version": "1.0.0",
      "config": {
        "customOption1": "value1",
        "customOption2": "value2"
      }
    }
  }
}
```

### Module Loading Process

1. **Discovery**: Scan modules directory for installed modules
2. **Loading**: Load module metadata and validate
3. **Initialization**: Initialize enabled modules
4. **Registration**: Register commands and hooks
5. **Startup**: Start enabled modules

### Module Development SDK

The SDK will provide:

1. **Base Implementation**: Common functionality for modules
2. **Testing Utilities**: Mock registry, repository, etc.
3. **Development Tools**: Module scaffolding, testing

Example module implementation:

```go
package mymodule

import (
    "context"
    
    "github.com/systemshift/memex/pkg/module"
)

// MyModule implements a custom module
type MyModule struct {
    *module.Base
    // Custom fields
}

// New creates a new instance of MyModule
func New() module.Module {
    m := &MyModule{
        Base: module.NewBase("mymodule", "My Module", "A custom module"),
    }
    
    // Register commands
    m.AddCommand(module.Command{
        Name:        "hello",
        Description: "Say hello",
        Usage:       "mymodule hello [name]",
        Handler:     m.handleHello,
    })
    
    return m
}

// Init initializes the module
func (m *MyModule) Init(ctx context.Context, registry module.Registry) error {
    if err := m.Base.Init(ctx, registry); err != nil {
        return err
    }
    
    // Custom initialization
    
    return nil
}

// handleHello handles the hello command
func (m *MyModule) handleHello(ctx context.Context, args []string) (interface{}, error) {
    name := "World"
    if len(args) > 0 {
        name = args[0]
    }
    
    return fmt.Sprintf("Hello, %s!", name), nil
}
```

## Storage System Improvements

### Content Storage

```go
// ContentStore manages content chunks
type ContentStore interface {
    // Basic operations
    Put(ctx context.Context, data []byte) (string, error)
    Get(ctx context.Context, hash string) ([]byte, error)
    Delete(ctx context.Context, hash string) error
    
    // Batch operations
    PutBatch(ctx context.Context, chunks []Chunk) ([]string, error)
    GetBatch(ctx context.Context, hashes []string) ([]Chunk, error)
    
    // Reference counting
    IncrementRef(ctx context.Context, hash string) error
    DecrementRef(ctx context.Context, hash string) error
    
    // Garbage collection
    CollectGarbage(ctx context.Context) error
}
```

### Graph Storage

```go
// GraphStore manages the node and edge structure
type GraphStore interface {
    // Node operations
    AddNode(ctx context.Context, node Node) error
    GetNode(ctx context.Context, id string) (Node, error)
    UpdateNode(ctx context.Context, node Node) error
    DeleteNode(ctx context.Context, id string) error
    
    // Edge operations
    AddEdge(ctx context.Context, edge Edge) error
    GetEdge(ctx context.Context, id string) (Edge, error)
    UpdateEdge(ctx context.Context, edge Edge) error
    DeleteEdge(ctx context.Context, id string) error
    
    // Graph operations
    GetNeighbors(ctx context.Context, nodeID string, direction Direction) ([]Node, error)
    GetEdgesBetween(ctx context.Context, sourceID, targetID string) ([]Edge, error)
    
    // Traversal
    Traverse(ctx context.Context, start string, options TraversalOptions) ([]Node, error)
    
    // Query
    Query(ctx context.Context, query GraphQuery) (QueryResult, error)
}
```

### Transaction System

```go
// TransactionManager handles ACID transactions
type TransactionManager interface {
    // Transaction control
    Begin(ctx context.Context) (Transaction, error)
    Commit(ctx context.Context, tx Transaction) error
    Rollback(ctx context.Context, tx Transaction) error
    
    // Transaction operations
    WithTransaction(ctx context.Context, fn func(tx Transaction) error) error
    
    // History and verification
    GetHistory(ctx context.Context, options HistoryOptions) ([]TransactionRecord, error)
    Verify(ctx context.Context, options VerifyOptions) (VerificationResult, error)
}
```

### Storage Format

The improved storage format will enhance the existing file-based approach:

1. **Enhanced Indexing**: Better indexing for faster lookups
2. **Optimized Content Chunks**: Improved content chunking and storage
3. **Efficient Graph Structure**: Optimized node and edge storage
4. **Metadata Caching**: In-memory caching for frequently accessed metadata
5. **Robust Transactions**: Enhanced transaction system with better verification

### Performance Improvements

1. **Indexing**: Efficient indexes for common queries
2. **Caching**: In-memory caching for frequently accessed data
3. **Batch Operations**: Support for batch operations
4. **Concurrency**: Better concurrency control
5. **Compression**: Optional compression for content

## API Layer Design

### REST API

```
GET    /api/v1/nodes                # List nodes
POST   /api/v1/nodes                # Create node
GET    /api/v1/nodes/:id            # Get node
PUT    /api/v1/nodes/:id            # Update node
DELETE /api/v1/nodes/:id            # Delete node

GET    /api/v1/nodes/:id/links      # Get node links
POST   /api/v1/links                # Create link
GET    /api/v1/links/:id            # Get link
PUT    /api/v1/links/:id            # Update link
DELETE /api/v1/links/:id            # Delete link

GET    /api/v1/modules              # List modules
POST   /api/v1/modules              # Install module
GET    /api/v1/modules/:id          # Get module info
PUT    /api/v1/modules/:id          # Update module
DELETE /api/v1/modules/:id          # Remove module
POST   /api/v1/modules/:id/enable   # Enable module
POST   /api/v1/modules/:id/disable  # Disable module

POST   /api/v1/modules/:id/:command # Execute module command
```

## Client Interfaces

### CLI Interface

```
memex init <name>                # Create a new repository
memex connect <path>             # Connect to existing repository
memex status                     # Show repository status

memex add <file>                 # Add a file to repository
memex delete <id>                # Delete a node
memex edit                       # Open editor for a new note
memex link <src> <dst> <type>    # Create a link between nodes
memex links <id>                 # Show links for a node

memex module install <source>    # Install a module
memex module remove <id>         # Remove a module
memex module enable <id>         # Enable a module
memex module disable <id>        # Disable a module
memex module list                # List installed modules
memex module update <id>         # Update a module

memex <module-id> <command>      # Execute a module command
```

### Web UI Interface

The web UI will be a React-based single-page application with:

1. **Graph Visualization**: Interactive graph view
2. **Node Management**: Create, edit, delete nodes
3. **Link Management**: Create, edit, delete links
4. **Module Management**: Install, remove, enable, disable modules
5. **Search**: Full-text search and advanced queries
6. **User Management**: Authentication and authorization

### SDK/Library Interface

```go
// Client is the main entry point for the Memex SDK
type Client struct {
    // Configuration
    config ClientConfig
    
    // API clients
    nodes  NodeClient
    links  LinkClient
    modules ModuleClient
}

// NodeClient provides node operations
type NodeClient interface {
    Get(ctx context.Context, id string) (*Node, error)
    List(ctx context.Context, filter NodeFilter) ([]*Node, error)
    Create(ctx context.Context, content []byte, nodeType string, meta map[string]interface{}) (*Node, error)
    Update(ctx context.Context, id string, updates NodeUpdates) (*Node, error)
    Delete(ctx context.Context, id string) error
}

// LinkClient provides link operations
type LinkClient interface {
    Get(ctx context.Context, id string) (*Link, error)
    List(ctx context.Context, filter LinkFilter) ([]*Link, error)
    Create(ctx context.Context, source, target, linkType string, meta map[string]interface{}) (*Link, error)
    Update(ctx context.Context, id string, updates LinkUpdates) (*Link, error)
    Delete(ctx context.Context, id string) error
}

// ModuleClient provides module operations
type ModuleClient interface {
    Get(ctx context.Context, id string) (*Module, error)
    List(ctx context.Context, filter ModuleFilter) ([]*Module, error)
    Install(ctx context.Context, source string, options InstallOptions) (*Module, error)
    Remove(ctx context.Context, id string) error
    Enable(ctx context.Context, id string) error
    Disable(ctx context.Context, id string) error
    ExecuteCommand(ctx context.Context, moduleID, command string, args []string) (interface{}, error)
}
```

## Implementation Phases

### Phase 1: Core Improvements (1-2 months)

1. **Storage Enhancements**
   - Improve indexing in existing storage
   - Enhance content chunking
   - Optimize graph operations

2. **Module System**
   - Design module interface
   - Implement module registry
   - Create module installer

3. **Basic API**
   - Implement core REST API
   - Create API client for CLI and web

### Phase 2: Client Interfaces (1-2 months)

1. **CLI**
   - Enhance command structure
   - Implement module command routing
   - Improve user experience

2. **Web UI**
   - Enhance existing web interface
   - Implement graph visualization
   - Connect to REST API

3. **SDK**
   - Design client library
   - Implement core functionality

### Phase 3: Module Development (1-2 months)

1. **Module SDK**
   - Finalize module interface
   - Create development tools
   - Write documentation

2. **Example Modules**
   - Implement AST module
   - Create visualization module
   - Build search module

### Phase 4: Optimization and Polish (1 month)

1. **Performance Optimization**
   - Optimize storage operations
   - Improve query performance

2. **Documentation**
   - Create comprehensive documentation
   - Write tutorials and examples

3. **Testing**
   - Implement comprehensive tests
   - Create benchmarks

## Migration Strategy

### Data Migration

1. **Export Tool**
   - Create tool to export data from old format
   - Support selective export

2. **Import Tool**
   - Create tool to import data into new format
   - Support mapping between old and new structures

3. **Verification**
   - Verify data integrity after migration
   - Compare old and new repositories

### API Compatibility

1. **Compatibility Layer**
   - Create adapter for old API
   - Support deprecated methods

2. **Version Detection**
   - Detect repository version
   - Provide upgrade path

3. **Gradual Transition**
   - Support both old and new formats
   - Deprecate old format over time

### Module Migration

1. **Module Adapter**
   - Create adapter for old modules
   - Support legacy module interface

2. **Module Conversion**
   - Tool to convert old modules to new format
   - Automated code transformation

3. **Documentation**
   - Guide for module developers
   - Migration examples

## Timeline and Milestones

### Month 1-2: Foundation

- Complete storage engine design
- Implement basic module system
- Create core API endpoints

### Month 3-4: Client Interfaces

- Implement CLI interface
- Create basic web UI
- Design SDK

### Month 5-6: Advanced Features

- Implement query language
- Add GraphQL API
- Create RPC API

### Month 7-8: Optimization and Migration

- Optimize performance
- Create migration tools
- Write documentation

### Month 9: Release

- Release beta version
- Gather feedback
- Fix issues

### Month 10-12: Stabilization

- Release stable version
- Support migration
- Deprecate old format

## Conclusion

This rewrite plan provides a balanced approach to improving Memex while maintaining its core strengths. By enhancing the existing architecture rather than completely replacing it, we can deliver meaningful improvements more quickly and with less risk.

The plan focuses on:
1. Strengthening the core functionality
2. Adding a well-designed module system
3. Improving performance through targeted enhancements
4. Providing a clean API for integration

This approach gives us a solid foundation that can evolve over time, adding more sophisticated features as needed based on real user feedback and requirements.
