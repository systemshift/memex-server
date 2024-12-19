package test

import (
	"testing"

	"memex/pkg/sdk/module"
)

func TestCommandRouting(t *testing.T) {
	// Create test repository
	repo := NewMockSDKRepository()

	// Create test module
	testMod := NewTestModule(repo)
	testMod.SetID("test")

	// Register module
	if err := repo.RegisterModule(testMod); err != nil {
		t.Fatalf("registering module: %v", err)
	}

	// Create module manager
	manager, err := module.NewModuleManager()
	if err != nil {
		t.Fatalf("creating module manager: %v", err)
	}

	// Set repository
	manager.SetRepository(repo)

	// Test command routing
	tests := []struct {
		name        string
		command     string
		args        []string
		wantCommand string
		wantError   bool
	}{
		{
			name:        "Add command",
			command:     "add",
			args:        []string{"arg1", "arg2"},
			wantCommand: "add",
			wantError:   false,
		},
		{
			name:        "Remove command",
			command:     "remove",
			args:        []string{"arg1"},
			wantCommand: "remove",
			wantError:   false,
		},
		{
			name:        "List command",
			command:     "list",
			args:        []string{},
			wantCommand: "list",
			wantError:   false,
		},
		{
			name:        "Unknown command",
			command:     "unknown",
			args:        []string{},
			wantCommand: "",
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.HandleCommand("test", tt.command, tt.args)
			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if testMod.GetLastCommand() != tt.wantCommand {
				t.Errorf("got command %q, want %q", testMod.GetLastCommand(), tt.wantCommand)
			}
		})
	}
}
