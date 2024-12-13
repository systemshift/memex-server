# Memex Module SDK

This document describes the SDK for developing Memex modules.

## Overview

The Memex Module SDK provides tools and utilities for developing both package modules and binary modules. It aims to make module development simple and standardized.

## Package Modules

### Base Module
```go
// BaseModule provides common module functionality
type BaseModule struct {
    id          string
    name        string
    description string
    repo        Repository
}

// NewBaseModule creates a new base module
func NewBaseModule(id, name, description string) *BaseModule

// Common interface implementations
func (m *BaseModule) ID() string
func (m *BaseModule) Name() string
func (m *BaseModule) Description() string
```

### Repository Interface
```go
// Repository provides access to memex operations
type Repository interface {
    // Node operations
    AddNode(content []byte, nodeType string, meta Meta) (string, error)
    GetNode(id string) (*Node, error)
    DeleteNode(id string) error
    
    // Link operations
    AddLink(source, target, linkType string, meta Meta) error
    GetLinks(nodeID string) ([]*Link, error)
    DeleteLink(source, target, linkType string) error
    
    // Query operations
    QueryNodes(query Query) ([]*Node, error)
    QueryLinks(query Query) ([]*Link, error)
}
```

### Helper Functions
```go
// Node type helpers
func ValidateNodeType(nodeType string, validTypes []string) bool
func IsNodeTypeValid(nodeType string, pattern string) bool

// Link type helpers
func ValidateLinkType(linkType string, validTypes []string) bool
func IsLinkTypeValid(linkType string, pattern string) bool

// Metadata helpers
func ValidateMetadata(meta Meta, required []string) error
func MergeMetadata(base, override Meta) Meta
```

## Binary Modules

### Protocol
```go
// Command protocol
type Command struct {
    Name    string          // Command name
    Args    []string        // Command arguments
    Input   io.Reader       // Standard input
    Output  io.Writer       // Standard output
    Error   io.Writer       // Standard error
    Context context.Context // Command context
}

// Response protocol
type Response struct {
    Status  int             // Response status code
    Data    interface{}     // Response data
    Error   string          // Error message if any
    Meta    Meta            // Response metadata
}
```

### Command Handler
```go
// CommandHandler processes module commands
type CommandHandler interface {
    Handle(cmd Command) Response
}

// BaseHandler provides common command handling
type BaseHandler struct {
    module Module
}

func (h *BaseHandler) Handle(cmd Command) Response {
    switch cmd.Name {
    case "id":
        return h.handleID()
    case "name":
        return h.handleName()
    case "describe":
        return h.handleDescribe()
    case "run":
        return h.handleRun(cmd)
    default:
        return h.handleUnknown(cmd)
    }
}
```

### I/O Utilities
```go
// ReadInput reads and parses standard input
func ReadInput(r io.Reader) ([]byte, error)

// WriteOutput writes formatted output
func WriteOutput(w io.Writer, data interface{}) error

// WriteError writes error response
func WriteError(w io.Writer, err error) error
```

## Module Development

### 1. Creating a Package Module
```go
package mymodule

import "github.com/systemshift/memex/sdk"

type MyModule struct {
    *sdk.BaseModule
    // Additional fields
}

func New() *MyModule {
    base := sdk.NewBaseModule(
        "mymodule",
        "My Module",
        "Does something useful",
    )
    return &MyModule{BaseModule: base}
}

// Implement additional functionality
func (m *MyModule) ProcessNode(node *sdk.Node) error {
    // Custom node processing
}
```

### 2. Creating a Binary Module
```go
package main

import "github.com/systemshift/memex/sdk"

func main() {
    module := NewMyModule()
    handler := sdk.NewBaseHandler(module)
    sdk.RunModule(handler)
}

type MyModule struct {
    *sdk.BaseModule
}

func (m *MyModule) HandleCommand(cmd sdk.Command) sdk.Response {
    // Custom command handling
}
```

## Best Practices

1. Module Structure
- Use the BaseModule for common functionality
- Implement custom capabilities as needed
- Follow standard naming conventions
- Include documentation

2. Error Handling
- Use standard error types
- Provide descriptive error messages
- Handle cleanup in error cases
- Log errors appropriately

3. Testing
- Test module interface implementation
- Test custom functionality
- Use mock repository for testing
- Test error conditions

4. Documentation
- Document module purpose
- Document custom node/link types
- Document configuration options
- Include usage examples

## Example Modules

1. AST Module Example
```go
package astmodule

import "github.com/systemshift/memex/sdk"

type ASTModule struct {
    *sdk.BaseModule
}

func New() *ASTModule {
    base := sdk.NewBaseModule(
        "ast",
        "AST Analysis",
        "Analyzes Go source code structure",
    )
    return &ASTModule{BaseModule: base}
}

func (m *ASTModule) ValidateNodeType(nodeType string) bool {
    return sdk.IsNodeTypeValid(nodeType, "ast.*")
}

func (m *ASTModule) ProcessFile(path string) error {
    // Parse and store AST
}
```

2. Git Module Example
```go
package gitmodule

import "github.com/systemshift/memex/sdk"

type GitModule struct {
    *sdk.BaseModule
}

func New() *GitModule {
    base := sdk.NewBaseModule(
        "git",
        "Git Integration",
        "Integrates with Git repositories",
    )
    return &GitModule{BaseModule: base}
}

func (m *GitModule) ValidateNodeType(nodeType string) bool {
    return sdk.IsNodeTypeValid(nodeType, "git.*")
}

func (m *GitModule) ProcessRepo(path string) error {
    // Process Git repository
}
```

## Future Enhancements

1. Enhanced Development Tools
- Module scaffolding tool
- Code generation utilities
- Development server
- Hot reload support

2. Additional Utilities
- Common data processors
- Standard query builders
- Caching utilities
- Batch operations

3. Integration Tools
- Web interface helpers
- API client generation
- Event handling utilities
- Inter-module messaging
