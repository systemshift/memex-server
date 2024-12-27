# Memex Modules

Memex modules are Go packages that extend Memex's functionality. This guide explains how to create and use modules.

## Creating a Module

1. Create a new Go package:
```go
package memexgit

import "memex/pkg/module"

type Git struct {
    *module.Base
}

func New() module.Module {
    return &Git{
        Base: module.NewBase("git", "Git Module", "Git integration for Memex"),
    }
}
```

2. Add commands:
```go
func New() module.Module {
    m := &Git{
        Base: module.NewBase("git", "Git Module", "Git integration for Memex"),
    }

    // Add git-specific commands
    m.AddCommand(module.Command{
        Name:        "status",
        Description: "Show git status",
        Usage:       "git status",
    })

    return m
}
```

3. Handle commands:
```go
func (g *Git) HandleCommand(cmd string, args []string) error {
    switch cmd {
    case "status":
        return g.handleStatus()
    default:
        return g.Base.HandleCommand(cmd, args)
    }
}

func (g *Git) handleStatus() error {
    // Use repository operations
    content := []byte("status output")
    meta := map[string]interface{}{
        "module": g.ID(),
        "type":   "git-status",
    }
    _, err := g.AddNode(content, "git-status", meta)
    return err
}
```

## Module Interface

The `module.Module` interface defines what a module must implement:

```go
type Module interface {
    // Identity
    ID() string          // Unique identifier (e.g., "git", "ast")
    Name() string        // Human-readable name
    Description() string // Module description

    // Core functionality
    Init(repo Repository) error                    // Initialize module
    Commands() []Command                           // Available commands
    HandleCommand(cmd string, args []string) error // Execute a command
}
```

## Base Module

The `module.Base` type provides a default implementation of the Module interface:

```go
base := module.NewBase("mymodule", "My Module", "Description")
```

It provides:
- Basic command handling (help, version)
- Repository operations (AddNode, GetNode, etc.)
- Command management (AddCommand)

## Repository Operations

Modules can use repository operations through the Base module:

```go
// Add a node
nodeID, err := m.AddNode(content, nodeType, meta)

// Get a node
node, err := m.GetNode(id)

// Add a link
err := m.AddLink(source, target, linkType, meta)

// Get links
links, err := m.GetLinks(nodeID)
```

## Installing Modules

Modules are installed using Go's module system:

```bash
# Add module to your project
go get github.com/user/memex-mymodule

# Import in main.go
import _ "github.com/user/memex-mymodule"
```

## Best Practices

1. Use descriptive module IDs:
   ```go
   module.NewBase("git", "Git Module", "Git integration")
   ```

2. Provide helpful command descriptions:
   ```go
   module.Command{
       Name:        "status",
       Description: "Show git status",
       Usage:       "git status [options]",
       Args:        []string{"options"},
   }
   ```

3. Store module-specific data with metadata:
   ```go
   meta := map[string]interface{}{
       "module": m.ID(),
       "type":   "git-commit",
       "hash":   "abc123",
   }
   ```

4. Handle errors appropriately:
   ```go
   if err := m.AddLink(source, target, "reference", meta); err != nil {
       return fmt.Errorf("adding reference link: %w", err)
   }
   ```

5. Clean up resources in shutdown if needed:
   ```go
   func (m *MyModule) HandleCommand(cmd string, args []string) error {
       if cmd == "shutdown" {
           // Clean up resources
           return nil
       }
       return m.Base.HandleCommand(cmd, args)
   }
   ```
