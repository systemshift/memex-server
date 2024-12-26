package sdk

import (
	"fmt"
	"memex/pkg/types"
)

// CommandBuilder helps build commands with a fluent interface
type CommandBuilder struct {
	cmd types.Command
}

// NewCommand starts building a new command
func NewCommand(name string) *CommandBuilder {
	return &CommandBuilder{
		cmd: types.Command{
			Name: name,
		},
	}
}

// WithDescription adds a description to the command
func (b *CommandBuilder) WithDescription(desc string) *CommandBuilder {
	b.cmd.Description = desc
	return b
}

// WithUsage adds usage information to the command
func (b *CommandBuilder) WithUsage(usage string) *CommandBuilder {
	b.cmd.Usage = usage
	return b
}

// WithArgs adds required arguments to the command
func (b *CommandBuilder) WithArgs(args ...string) *CommandBuilder {
	b.cmd.Args = args
	return b
}

// Build returns the constructed command
func (b *CommandBuilder) Build() types.Command {
	return b.cmd
}

// CommandHandler represents a function that handles a command
type CommandHandler func(repo types.Repository, args []string) error

// CommandSet represents a set of commands with their handlers
type CommandSet struct {
	commands map[string]types.Command
	handlers map[string]CommandHandler
}

// NewCommandSet creates a new command set
func NewCommandSet() *CommandSet {
	return &CommandSet{
		commands: make(map[string]types.Command),
		handlers: make(map[string]CommandHandler),
	}
}

// Add adds a command and its handler to the set
func (cs *CommandSet) Add(cmd types.Command, handler CommandHandler) {
	cs.commands[cmd.Name] = cmd
	cs.handlers[cmd.Name] = handler
}

// Get returns a command and its handler by name
func (cs *CommandSet) Get(name string) (types.Command, CommandHandler, bool) {
	cmd, exists := cs.commands[name]
	if !exists {
		return types.Command{}, nil, false
	}
	return cmd, cs.handlers[name], true
}

// List returns all commands in the set
func (cs *CommandSet) List() []types.Command {
	cmds := make([]types.Command, 0, len(cs.commands))
	for _, cmd := range cs.commands {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// Handle executes a command with the given arguments
func (cs *CommandSet) Handle(repo types.Repository, cmd string, args []string) error {
	_, handler, exists := cs.Get(cmd)
	if !exists {
		return fmt.Errorf("%w: command %s", ErrNotFound, cmd)
	}
	return handler(repo, args)
}

// Common command patterns

// NewAddCommand creates a standard add command
func NewAddCommand(itemType string, handler CommandHandler) (types.Command, CommandHandler) {
	cmd := NewCommand("add").
		WithDescription("Add " + itemType).
		WithUsage("add <" + itemType + ">").
		WithArgs(itemType).
		Build()
	return cmd, handler
}

// NewListCommand creates a standard list command
func NewListCommand(itemType string, handler CommandHandler) (types.Command, CommandHandler) {
	cmd := NewCommand("list").
		WithDescription("List " + itemType + "s").
		WithUsage("list [filter]").
		Build()
	return cmd, handler
}

// NewRemoveCommand creates a standard remove command
func NewRemoveCommand(itemType string, handler CommandHandler) (types.Command, CommandHandler) {
	cmd := NewCommand("remove").
		WithDescription("Remove " + itemType).
		WithUsage("remove <" + itemType + ">").
		WithArgs(itemType).
		Build()
	return cmd, handler
}

// NewShowCommand creates a standard show command
func NewShowCommand(itemType string, handler CommandHandler) (types.Command, CommandHandler) {
	cmd := NewCommand("show").
		WithDescription("Show " + itemType + " details").
		WithUsage("show <" + itemType + ">").
		WithArgs(itemType).
		Build()
	return cmd, handler
}

// NewUpdateCommand creates a standard update command
func NewUpdateCommand(itemType string, handler CommandHandler) (types.Command, CommandHandler) {
	cmd := NewCommand("update").
		WithDescription("Update " + itemType).
		WithUsage("update <" + itemType + "> [options]").
		WithArgs(itemType).
		Build()
	return cmd, handler
}

// ValidateArgs validates command arguments
func ValidateArgs(cmd types.Command, args []string) error {
	if len(args) < len(cmd.Args) {
		return fmt.Errorf("%w: missing required arguments: %v", ErrInvalidInput, cmd.Args[len(args):])
	}
	return nil
}
