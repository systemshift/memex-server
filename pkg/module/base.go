package module

import (
	"context"
	"fmt"
)

// Base provides a base implementation of the Module interface
type Base struct {
	id          string
	name        string
	description string
	version     string
	commands    map[string]Command
	hooks       map[string]Hook
	registry    Registry
}

// NewBase creates a new Base instance
func NewBase(id, name, description string) *Base {
	return &Base{
		id:          id,
		name:        name,
		description: description,
		version:     "0.1.0",
		commands:    make(map[string]Command),
		hooks:       make(map[string]Hook),
	}
}

// ID returns the module ID
func (b *Base) ID() string {
	return b.id
}

// Name returns the module name
func (b *Base) Name() string {
	return b.name
}

// Description returns the module description
func (b *Base) Description() string {
	return b.description
}

// Version returns the module version
func (b *Base) Version() string {
	return b.version
}

// SetVersion sets the module version
func (b *Base) SetVersion(version string) {
	b.version = version
}

// Init initializes the module
func (b *Base) Init(ctx context.Context, registry Registry) error {
	b.registry = registry
	return nil
}

// Start starts the module
func (b *Base) Start(ctx context.Context) error {
	return nil
}

// Stop stops the module
func (b *Base) Stop(ctx context.Context) error {
	return nil
}

// AddCommand adds a command to the module
func (b *Base) AddCommand(cmd Command) {
	b.commands[cmd.Name] = cmd
}

// Commands returns the module commands
func (b *Base) Commands() []Command {
	cmds := make([]Command, 0, len(b.commands))
	for _, cmd := range b.commands {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// HandleCommand handles a command
func (b *Base) HandleCommand(ctx context.Context, cmd string, args []string) (interface{}, error) {
	command, ok := b.commands[cmd]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrCommandNotFound, cmd)
	}

	if command.Handler == nil {
		return nil, fmt.Errorf("no handler for command: %s", cmd)
	}

	return command.Handler(ctx, args)
}

// AddHook adds a hook to the module
func (b *Base) AddHook(hook Hook) {
	b.hooks[hook.Name] = hook
}

// Hooks returns the module hooks
func (b *Base) Hooks() []Hook {
	hooks := make([]Hook, 0, len(b.hooks))
	for _, hook := range b.hooks {
		hooks = append(hooks, hook)
	}
	return hooks
}

// HandleHook handles a hook
func (b *Base) HandleHook(ctx context.Context, hook string, data interface{}) (interface{}, error) {
	// Default implementation does nothing
	return nil, nil
}

// GetRegistry returns the module registry
func (b *Base) GetRegistry() Registry {
	return b.registry
}
