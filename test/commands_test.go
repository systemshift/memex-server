package test

import (
	"reflect"
	"testing"

	"memex/pkg/sdk"
	"memex/pkg/types"
)

func TestCommandBuilder(t *testing.T) {
	cmd := sdk.NewCommand("test").
		WithDescription("Test command").
		WithUsage("test <arg>").
		WithArgs("arg").
		Build()

	want := types.Command{
		Name:        "test",
		Description: "Test command",
		Usage:       "test <arg>",
		Args:        []string{"arg"},
	}

	if !reflect.DeepEqual(cmd, want) {
		t.Errorf("got %v, want %v", cmd, want)
	}
}

func TestCommandSet(t *testing.T) {
	cs := sdk.NewCommandSet()
	repo := newMockRepository()

	// Test command handler
	var handlerCalled bool
	handler := func(r types.Repository, args []string) error {
		handlerCalled = true
		return nil
	}

	// Add command
	cmd := sdk.NewCommand("test").
		WithDescription("Test command").
		WithUsage("test <arg>").
		WithArgs("arg").
		Build()
	cs.Add(cmd, handler)

	// Test Get
	gotCmd, gotHandler, exists := cs.Get("test")
	if !exists {
		t.Error("command not found")
	}
	if !reflect.DeepEqual(gotCmd, cmd) {
		t.Errorf("got %v, want %v", gotCmd, cmd)
	}
	if gotHandler == nil {
		t.Error("handler is nil")
	}

	// Test List
	cmds := cs.List()
	if len(cmds) != 1 {
		t.Errorf("got %d commands, want 1", len(cmds))
	}
	if !reflect.DeepEqual(cmds[0], cmd) {
		t.Errorf("got %v, want %v", cmds[0], cmd)
	}

	// Test Handle
	if err := cs.Handle(repo, "test", []string{"arg"}); err != nil {
		t.Errorf("Handle() error = %v", err)
	}
	if !handlerCalled {
		t.Error("handler was not called")
	}
}

func TestCommonCommands(t *testing.T) {
	// Mock handler for testing
	handler := func(r types.Repository, args []string) error { return nil }

	tests := []struct {
		name     string
		builder  func(string, sdk.CommandHandler) (types.Command, sdk.CommandHandler)
		itemType string
		want     types.Command
	}{
		{
			name:     "add command",
			builder:  sdk.NewAddCommand,
			itemType: "item",
			want: types.Command{
				Name:        "add",
				Description: "Add item",
				Usage:       "add <item>",
				Args:        []string{"item"},
			},
		},
		{
			name:     "list command",
			builder:  sdk.NewListCommand,
			itemType: "item",
			want: types.Command{
				Name:        "list",
				Description: "List items",
				Usage:       "list [filter]",
			},
		},
		{
			name:     "remove command",
			builder:  sdk.NewRemoveCommand,
			itemType: "item",
			want: types.Command{
				Name:        "remove",
				Description: "Remove item",
				Usage:       "remove <item>",
				Args:        []string{"item"},
			},
		},
		{
			name:     "show command",
			builder:  sdk.NewShowCommand,
			itemType: "item",
			want: types.Command{
				Name:        "show",
				Description: "Show item details",
				Usage:       "show <item>",
				Args:        []string{"item"},
			},
		},
		{
			name:     "update command",
			builder:  sdk.NewUpdateCommand,
			itemType: "item",
			want: types.Command{
				Name:        "update",
				Description: "Update item",
				Usage:       "update <item> [options]",
				Args:        []string{"item"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := tt.builder(tt.itemType, handler)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateArgs(t *testing.T) {
	cmd := types.Command{
		Name:        "test",
		Description: "Test command",
		Usage:       "test <arg1> <arg2>",
		Args:        []string{"arg1", "arg2"},
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "valid args",
			args:    []string{"value1", "value2"},
			wantErr: false,
		},
		{
			name:    "missing one arg",
			args:    []string{"value1"},
			wantErr: true,
		},
		{
			name:    "missing all args",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "extra args",
			args:    []string{"value1", "value2", "value3"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sdk.ValidateArgs(cmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
