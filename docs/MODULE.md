# Memex Module System

This document describes the module system for extending Memex functionality.

## Overview

The Memex module system allows extending core functionality through modules that can:
- Add new node and link types
- Provide custom data analysis
- Integrate with external tools and services
- Add new commands to the CLI

## Module Types

### 1. Package Modules
Go packages that implement the Module interface and can be imported directly:

```go
type Module interface {
    ID() string
    Name() string
    Description() string
    Capabilities() []ModuleCapability
    ValidateNodeType(nodeType string) bool
    ValidateLinkType(linkType string) bool
    ValidateMetadata(meta map[string]interface{}) error
}
```

### 2. Binary Modules
Standalone executables that communicate with Memex through standard protocols:
- Input: Command-line arguments and stdin
- Output: Structured data through stdout
- Must implement module interface commands

## Module Development

### Development Workflow

Modules can be installed in two modes:

1. Production Install
```bash
memex module install <module-name>
```
- Installs to global modules directory
- Used for stable, released modules
- Modules are copied to ~/.config/memex/modules/

2. Development Install
```bash
memex module install --dev <path>
```
- Uses module directly from source location
- No files are copied
- Ideal for module development
- Supports unreleased code and dependencies

### Package Module Example

```go
package mymodule

type MyModule struct {
    repo core.Repository
}

func (m *MyModule) ID() string { return "mymodule" }
func (m *MyModule) Name() string { return "My Module" }
func (m *MyModule) Description() string { return "Does something useful" }
func (m *MyModule) Capabilities() []core.ModuleCapability { return nil }

// Implement other interface methods...
```

### Binary Module Example

```bash
#!/usr/bin/env bash

case "$1" in
    "id")
        echo "mymodule"
        ;;
    "name")
        echo "My Module"
        ;;
    "describe")
        echo "Does something useful"
        ;;
    "run")
        # Handle module-specific commands
        ;;
esac
```

## Module Discovery

Memex looks for modules in:
1. Development modules: Modules being developed locally with --dev flag
2. Global modules: ~/.config/memex/modules/

## Module Configuration

Modules are configured through the memex configuration system:

```json
{
    "modules": {
        "module-name": {
            "path": "/path/to/module",
            "type": "package|binary",
            "dev": false,           // true for development modules
            "settings": {
                // Module-specific settings
            }
        }
    }
}
```

## CLI Integration

Modules can be managed through the memex CLI:

```bash
# List installed modules
memex module list

# Install a module (production)
memex module install <module-name>

# Install a module (development)
memex module install --dev <path>

# Remove a module
memex module remove <name>

# Run module-specific command
memex module run <name> [args...]
```

## Module Capabilities

Modules can provide various capabilities:

1. Node Types
- Define custom node types
- Provide node type validation
- Handle node content processing

2. Link Types
- Define custom link types
- Provide link validation
- Handle relationship semantics

3. Commands
- Add new CLI commands
- Extend existing commands
- Provide custom subcommands

4. Data Processing
- Content analysis
- Data transformation
- External integrations

## Security Considerations

1. Module Verification
- Verify module authenticity
- Check module signatures
- Validate module capabilities

2. Permissions
- Modules run with limited permissions
- Access only through repository interface
- No direct file system access

3. Resource Limits
- Memory usage limits
- CPU usage limits
- Storage quotas

## Implementation Roadmap

1. Phase 1: Core Module System (Current)
- Module interface
- Development workflow
- Basic discovery
- Package modules

2. Phase 2: Module Repository
- Registry design
- Version management
- Distribution system

3. Phase 3: Enhanced Features
- Event system
- Inter-module communication
- Web integration

## Example Use Cases

1. AST Module
- Parse source code
- Build code graphs
- Track dependencies

2. Git Module
- Version control integration
- Commit tracking
- Development context

3. Doc Module
- Documentation parsing
- Knowledge extraction
- Cross-referencing
