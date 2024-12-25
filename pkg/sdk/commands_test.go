package sdk

import (
	"memex/pkg/sdk/types"
	"testing"
)

func TestCommandBuilder(t *testing.T) {
	tests := []struct {
		name      string
		build     func() *CommandBuilder
		wantName  string
		wantDesc  string
		wantUsage string
		wantArgs  []string
	}{
		{
			name: "basic command",
			build: func() *CommandBuilder {
				return NewCommand("test")
			},
			wantName: "test",
		},
		{
			name: "full command",
			build: func() *CommandBuilder {
				return NewCommand("test").
					WithDescription("Test command").
					WithUsage("test <arg>").
					WithArgs("arg1", "arg2")
			},
			wantName:  "test",
			wantDesc:  "Test command",
			wantUsage: "test <arg>",
			wantArgs:  []string{"arg1", "arg2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.build().Build()

			if cmd.Name != tt.wantName {
				t.Errorf("Name = %v, want %v", cmd.Name, tt.wantName)
			}
			if cmd.Description != tt.wantDesc {
				t.Errorf("Description = %v, want %v", cmd.Description, tt.wantDesc)
			}
			if cmd.Usage != tt.wantUsage {
				t.Errorf("Usage = %v, want %v", cmd.Usage, tt.wantUsage)
			}
			if len(cmd.Args) != len(tt.wantArgs) {
				t.Errorf("Args length = %v, want %v", len(cmd.Args), len(tt.wantArgs))
			} else {
				for i, arg := range cmd.Args {
					if arg != tt.wantArgs[i] {
						t.Errorf("Arg[%d] = %v, want %v", i, arg, tt.wantArgs[i])
					}
				}
			}
		})
	}
}

func TestCommonCommands(t *testing.T) {
	tests := []struct {
		name     string
		build    func() types.Command
		wantName string
		wantArgs []string
	}{
		{
			name: "add command",
			build: func() types.Command {
				return NewAddCommand("item")
			},
			wantName: "add",
			wantArgs: []string{"item"},
		},
		{
			name: "list command",
			build: func() types.Command {
				return NewListCommand("item")
			},
			wantName: "list",
			wantArgs: []string{},
		},
		{
			name: "remove command",
			build: func() types.Command {
				return NewRemoveCommand("item")
			},
			wantName: "remove",
			wantArgs: []string{"item"},
		},
		{
			name: "show command",
			build: func() types.Command {
				return NewShowCommand("item")
			},
			wantName: "show",
			wantArgs: []string{"item"},
		},
		{
			name: "update command",
			build: func() types.Command {
				return NewUpdateCommand("item")
			},
			wantName: "update",
			wantArgs: []string{"item"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.build()

			if cmd.Name != tt.wantName {
				t.Errorf("Name = %v, want %v", cmd.Name, tt.wantName)
			}
			if len(cmd.Args) != len(tt.wantArgs) {
				t.Errorf("Args length = %v, want %v", len(cmd.Args), len(tt.wantArgs))
			} else {
				for i, arg := range cmd.Args {
					if arg != tt.wantArgs[i] {
						t.Errorf("Arg[%d] = %v, want %v", i, arg, tt.wantArgs[i])
					}
				}
			}
		})
	}
}
