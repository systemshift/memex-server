package test

import (
	"testing"
)

func TestBaseModule(t *testing.T) {
	// Test module creation
	base := NewMockModule()

	// Test identity methods
	if base.ID() != "mock" {
		t.Errorf("ID() = %v, want %v", base.ID(), "mock")
	}
	if base.Name() != "Mock Module" {
		t.Errorf("Name() = %v, want %v", base.Name(), "Mock Module")
	}
	if base.Description() != "A mock module for testing" {
		t.Errorf("Description() = %v, want %v", base.Description(), "A mock module for testing")
	}

	// Test default commands
	cmds := base.Commands()
	if len(cmds) != 0 {
		t.Errorf("Commands() returned %v commands, want 0", len(cmds))
	}

	// Test command handling
	tests := []struct {
		name      string
		cmd       string
		args      []string
		wantError bool
	}{
		{
			name:      "basic command",
			cmd:       "test",
			args:      []string{"arg1", "arg2"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := base.HandleCommand(tt.cmd, tt.args)
			if (err != nil) != tt.wantError {
				t.Errorf("HandleCommand() error = %v, wantError %v", err, tt.wantError)
			}
			if base.lastCommand != tt.cmd {
				t.Errorf("HandleCommand() lastCommand = %v, want %v", base.lastCommand, tt.cmd)
			}
			if len(base.lastArgs) != len(tt.args) {
				t.Errorf("HandleCommand() lastArgs = %v, want %v", base.lastArgs, tt.args)
			}
		})
	}
}

func TestModuleInitialization(t *testing.T) {
	// Create a mock module
	mod := NewMockModule()

	// Create a mock repository
	repo := NewMockRepository()

	// Test initialization
	if err := mod.Init(repo); err != nil {
		t.Errorf("Init() error = %v", err)
	}
	if !mod.initCalled {
		t.Error("Init() did not set initCalled")
	}

	// Test module registration
	if err := repo.RegisterModule(mod); err != nil {
		t.Errorf("RegisterModule() error = %v", err)
	}

	// Verify module was registered
	if m, exists := repo.GetModule("mock"); !exists {
		t.Error("GetModule() module not found")
	} else if m != mod {
		t.Error("GetModule() returned wrong module")
	}

	// Test listing modules
	modules := repo.ListModules()
	if len(modules) != 1 {
		t.Errorf("ListModules() returned %v modules, want 1", len(modules))
	}
	if modules[0] != mod {
		t.Error("ListModules() returned wrong module")
	}
}
