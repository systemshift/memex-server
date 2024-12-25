package sdk

import (
	"testing"

	"memex/pkg/sdk/types"
)

func TestNewBaseModule(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		moduleName  string
		description string
	}{
		{
			name:        "basic module",
			id:          "test",
			moduleName:  "Test Module",
			description: "A test module",
		},
		{
			name:        "empty description",
			id:          "empty",
			moduleName:  "Empty Module",
			description: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewBaseModule(tt.id, tt.moduleName, tt.description)

			if m.ID() != tt.id {
				t.Errorf("ID() = %v, want %v", m.ID(), tt.id)
			}
			if m.Name() != tt.moduleName {
				t.Errorf("Name() = %v, want %v", m.Name(), tt.moduleName)
			}
			if m.Description() != tt.description {
				t.Errorf("Description() = %v, want %v", m.Description(), tt.description)
			}

			// Check default commands
			cmds := m.Commands()
			if len(cmds) != 4 {
				t.Errorf("Commands() returned %v commands, want 4", len(cmds))
			}
		})
	}
}

func TestModuleHooks(t *testing.T) {
	var (
		initCalled     bool
		commandCalled  bool
		shutdownCalled bool
	)

	m := NewBaseModule("test", "Test", "Test Module",
		WithInitHook(func(r types.Repository) error {
			initCalled = true
			return nil
		}),
		WithCommandHook(func(cmd string, args []string) error {
			commandCalled = true
			return nil
		}),
		WithShutdownHook(func() error {
			shutdownCalled = true
			return nil
		}),
	)

	// Test Init hook
	if err := m.Init(nil); err != nil {
		t.Errorf("Init() error = %v", err)
	}
	if !initCalled {
		t.Error("Init hook was not called")
	}

	// Test Command hook
	if err := m.HandleCommand("id", nil); err != nil {
		t.Errorf("HandleCommand() error = %v", err)
	}
	if !commandCalled {
		t.Error("Command hook was not called")
	}

	// Test Shutdown hook
	if err := m.Shutdown(); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
	if !shutdownCalled {
		t.Error("Shutdown hook was not called")
	}
}

func TestHandleCommand(t *testing.T) {
	m := NewBaseModule("test", "Test", "Test Module")

	tests := []struct {
		name      string
		cmd       string
		args      []string
		wantError bool
	}{
		{
			name:      "built-in id command",
			cmd:       "id",
			args:      nil,
			wantError: false,
		},
		{
			name:      "built-in help command",
			cmd:       "help",
			args:      nil,
			wantError: false,
		},
		{
			name:      "unknown command",
			cmd:       "unknown",
			args:      nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.HandleCommand(tt.cmd, tt.args)
			if (err != nil) != tt.wantError {
				t.Errorf("HandleCommand() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
