# Memex Module System

This document explains how to use and develop modules for Memex.

## Using Modules

### Installing Modules

Modules can be installed from a local directory or a Git repository:

```bash
# Install from a local directory
memex module install /path/to/module

# Install from a Git repository
memex module install https://github.com/username/memex-module.git
```

### Listing Installed Modules

To see all installed modules:

```bash
memex module list
```

### Removing Modules

To remove a module:

```bash
memex module remove <module-id>
```

### Using Module Commands

Once a module is installed, you can use its commands:

```bash
memex <module-id> <command> [arguments]
```

For example, if you have a module with ID "hello" that has a "greet" command:

```bash
memex hello greet John
```

## Developing Modules

### Module Structure

A Memex module is a Go plugin with the following structure:

```
module-directory/
├── go.mod           # Go module definition
├── main.go          # Module implementation
└── README.md        # Documentation
```

### Module Implementation

Here's a simple example of a module implementation:

```go
package main

import (
	"context"
	"fmt"

	"github.com/systemshift/memex/pkg/module"
)

// MyModule is a custom module
type MyModule struct {
	*module.Base
}

// New creates a new module instance
func New() module.Module {
	m := &MyModule{
		Base: module.NewBase("mymodule", "My Module", "A custom module"),
	}

	// Add commands
	m.AddCommand(module.Command{
		Name:        "hello",
		Description: "Say hello",
		Usage:       "mymodule hello [name]",
		Handler:     m.handleHello,
	})

	return m
}

// handleHello handles the hello command
func (m *MyModule) handleHello(ctx context.Context, args []string) (interface{}, error) {
	name := "World"
	if len(args) > 0 {
		name = args[0]
	}
	return fmt.Sprintf("Hello, %s!", name), nil
}

// Required for Go plugins
func main() {}
```

### Module Interface

A module must implement the `module.Module` interface:

```go
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
```

The `module.Base` struct provides a default implementation of this interface, which you can embed in your module struct.

### Adding Commands

Commands are added to a module using the `AddCommand` method:

```go
m.AddCommand(module.Command{
	Name:        "command-name",
	Description: "Command description",
	Usage:       "module-id command-name [args]",
	Args:        []string{"arg1", "arg2"},  // Optional
	Flags:       []module.Flag{...},        // Optional
	Handler:     handlerFunction,
})
```

The handler function has the signature:

```go
func(ctx context.Context, args []string) (interface{}, error)
```

### Building a Module

To build a module:

```bash
go build -buildmode=plugin -o module.so .
```

### Testing a Module

You can test your module by installing it locally:

```bash
# Build the module
go build -buildmode=plugin -o module.so .

# Install the module
memex module install /path/to/module

# Test the module
memex <module-id> <command> [args]
```

## Example Modules

See the `examples/hello-module` directory for a simple example module.

## Advanced Features

### Hooks

Modules can register hooks to extend functionality:

```go
m.AddHook(module.Hook{
	Name:        "hook-name",
	Description: "Hook description",
	Priority:    10,
})
```

And implement the `HandleHook` method:

```go
func (m *MyModule) HandleHook(ctx context.Context, hook string, data interface{}) (interface{}, error) {
	if hook == "hook-name" {
		// Handle the hook
		return result, nil
	}
	return nil, nil
}
```

### Repository Access

Modules can access the Memex repository through the Registry interface:

```go
func (m *MyModule) Init(ctx context.Context, registry module.Registry) error {
	if err := m.Base.Init(ctx, registry); err != nil {
		return err
	}

	// Access repository
	repo := m.GetRegistry()
	
	return nil
}
```

## Best Practices

1. **Unique ID**: Use a unique ID for your module to avoid conflicts
2. **Clear Documentation**: Document your module's commands and functionality
3. **Error Handling**: Provide clear error messages
4. **Versioning**: Use semantic versioning for your module
5. **Testing**: Test your module thoroughly before distributing it
