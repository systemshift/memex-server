# Memex Module System

This document describes the module system for extending Memex functionality.

## Overview

The Memex module system allows extending core functionality through Go modules that can:
- Add new commands to the CLI
- Define custom node and link types
- Provide data analysis and transformations
- Integrate with external tools and services

## Module Interface

```go
// Module defines the interface that all memex modules must implement
type Module interface {
    // Identity
    ID() string          // Unique identifier (e.g., "git", "ast")
    Name() string        // Human-readable name
    Description() string // Module description

    // Core functionality
    Init(repo Repository) error                    // Initialize module with repository
    Commands() []Command                           // Available commands
    HandleCommand(cmd string, args []string) error // Execute a command

    // Optional capabilities
    ValidateNodeType(nodeType string) bool                  // Validate node types
    ValidateLinkType(linkType string) bool                  // Validate link types
    ValidateMetadata(meta map[string]interface{}) error     // Validate metadata
}

// Command represents a module command
type Command struct {
    Name        string   // Command name (e.g., "add", "status")
    Description string   // Command description
    Usage       string   // Usage example (e.g., "git add <file>")
    Args        []string // Expected arguments
}
```

## Installing Modules

Modules are distributed as Go packages and can be installed using standard Go tools:

```bash
# Install latest version
go install github.com/username/memex-git@latest

# Install specific version
go install github.com/username/memex-git@v1.0.0
```

## Using Modules

Once installed, modules are automatically available in memex:

```bash
# List available modules
memex module list

# Use module commands
memex git init
memex git status
memex ast analyze main.go
```

## Creating Modules

1. Create a new repository following the module interface:

```go
package git

type GitModule struct {
    repo Repository
}

func (m *GitModule) ID() string { return "git" }
func (m *GitModule) Name() string { return "Git Integration" }
func (m *GitModule) Description() string { return "Git version control integration" }

func (m *GitModule) Init(repo Repository) error {
    m.repo = repo
    return nil
}

func (m *GitModule) Commands() []Command {
    return []Command{
        {
            Name:        "init",
            Description: "Initialize git repository",
            Usage:       "git init",
        },
        {
            Name:        "status",
            Description: "Show working tree status",
            Usage:       "git status",
        },
    }
}

func (m *GitModule) HandleCommand(cmd string, args []string) error {
    switch cmd {
    case "init":
        return m.initRepo()
    case "status":
        return m.showStatus()
    default:
        return fmt.Errorf("unknown command: %s", cmd)
    }
}
```

2. Register your module:

```go
package main

import (
    "github.com/username/memex-git/git"
    "github.com/username/memex/pkg/sdk"
)

func init() {
    sdk.RegisterModule(git.NewGitModule())
}
```

## Module Development

1. Create a new module:
```bash
# Create module directory
mkdir memex-git
cd memex-git

# Initialize Go module
go mod init github.com/username/memex-git

# Add memex as dependency
go get github.com/username/memex
```

2. Implement the Module interface:
- Create your module package (e.g., git/module.go)
- Implement required methods
- Add your module's commands
- Register the module

3. Test locally:
```bash
# Build and install
go install .

# Test with memex
memex module list
memex git --help
```

## Module Capabilities

Modules can provide various capabilities:

1. Commands
- Add new CLI commands
- Extend existing functionality
- Process repository data

2. Node Types
- Define custom node types
- Validate node content
- Process node data

3. Link Types
- Define relationship types
- Validate link metadata
- Handle link operations

4. Data Processing
- Analyze repository content
- Transform data
- Integrate external tools

## Example Modules

1. Git Module
- Version control integration
- Track repository changes
- Link commits to nodes

2. AST Module
- Parse source code
- Build code graphs
- Track dependencies

3. Doc Module
- Parse documentation
- Extract knowledge
- Build documentation graphs

## Best Practices

1. Module Design
- Keep modules focused and single-purpose
- Use clear, descriptive command names
- Provide helpful command descriptions
- Include usage examples

2. Error Handling
- Return clear error messages
- Validate input early
- Handle edge cases gracefully

3. Documentation
- Document module capabilities
- Include example usage
- Explain command arguments
- Provide setup instructions
