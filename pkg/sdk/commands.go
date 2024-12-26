package sdk

import "memex/pkg/types"

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

// Common command patterns

// NewAddCommand creates a standard add command
func NewAddCommand(itemType string) types.Command {
	return NewCommand("add").
		WithDescription("Add " + itemType).
		WithUsage("add <" + itemType + ">").
		WithArgs(itemType).
		Build()
}

// NewListCommand creates a standard list command
func NewListCommand(itemType string) types.Command {
	return NewCommand("list").
		WithDescription("List " + itemType + "s").
		WithUsage("list [filter]").
		Build()
}

// NewRemoveCommand creates a standard remove command
func NewRemoveCommand(itemType string) types.Command {
	return NewCommand("remove").
		WithDescription("Remove " + itemType).
		WithUsage("remove <" + itemType + ">").
		WithArgs(itemType).
		Build()
}

// NewShowCommand creates a standard show command
func NewShowCommand(itemType string) types.Command {
	return NewCommand("show").
		WithDescription("Show " + itemType + " details").
		WithUsage("show <" + itemType + ">").
		WithArgs(itemType).
		Build()
}

// NewUpdateCommand creates a standard update command
func NewUpdateCommand(itemType string) types.Command {
	return NewCommand("update").
		WithDescription("Update " + itemType).
		WithUsage("update <" + itemType + "> [options]").
		WithArgs(itemType).
		Build()
}
