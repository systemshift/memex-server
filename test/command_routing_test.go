package test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"memex/internal/memex"
	pkgmemex "memex/pkg/memex"
)

func setupTestModule(t *testing.T) (*TestModule, *pkgmemex.Commands, *memex.ModuleManager) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "memex-module-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	// Set HOME to temp dir for module manager config
	os.Setenv("HOME", tmpDir)

	// Create test module directory
	moduleDir := filepath.Join(tmpDir, "test")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatalf("Failed to create module dir: %v", err)
	}

	// Create module file
	moduleFile := filepath.Join(moduleDir, "module.go")
	if err := os.WriteFile(moduleFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create module file: %v", err)
	}

	// Create repository
	repo := NewMockRepository()

	// Create and register module
	module := NewTestModule(repo)
	module.SetID("test") // Use consistent ID
	if err := repo.RegisterModule(module); err != nil {
		t.Fatalf("Failed to register module: %v", err)
	}

	// Set up global repository for commands
	memex.SetRepository(repo)

	// Create module manager and set repository
	manager, err := memex.NewModuleManager()
	if err != nil {
		t.Fatalf("Failed to create module manager: %v", err)
	}
	manager.SetRepository(repo)

	// Install and enable module
	if err := manager.InstallModule(moduleDir); err != nil {
		t.Fatalf("Failed to install module: %v", err)
	}
	if err := manager.EnableModule(module.ID()); err != nil {
		t.Fatalf("Failed to enable module: %v", err)
	}

	// Create commands
	cmds := pkgmemex.NewCommands()

	return module, cmds, manager
}

func TestCommandRouting(t *testing.T) {
	module, cmds, manager := setupTestModule(t)

	tests := []struct {
		name      string
		args      []string
		wantCmd   string
		wantArgs  []string
		wantError bool
	}{
		{
			name:     "Direct module command",
			args:     []string{"test", "add", "file.txt"},
			wantCmd:  "add",
			wantArgs: []string{"file.txt"},
		},
		{
			name:     "Module run command",
			args:     []string{"run", "test", "add", "file.txt"},
			wantCmd:  "add",
			wantArgs: []string{"file.txt"},
		},
		{
			name:      "Unknown module",
			args:      []string{"unknown", "add", "file.txt"},
			wantError: true,
		},
		{
			name:      "Unknown command",
			args:      []string{"test", "unknown", "file.txt"},
			wantError: true,
		},
		{
			name:      "Missing command",
			args:      []string{"test"},
			wantError: true,
		},
		{
			name:      "Disabled module",
			args:      []string{"test", "add", "file.txt"},
			wantError: true,
		},
		{
			name:      "Command failure",
			args:      []string{"test", "add", "file.txt"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset module state
			module.lastCommand = ""
			module.lastArgs = nil
			module.SetShouldFail(tt.wantError)

			// Enable/disable module based on test case
			if tt.name == "Disabled module" {
				if err := manager.DisableModule(module.ID()); err != nil {
					t.Fatalf("Failed to disable module: %v", err)
				}
			} else {
				if err := manager.EnableModule(module.ID()); err != nil {
					t.Fatalf("Failed to enable module: %v", err)
				}
			}

			// Execute command
			err := cmds.Module(tt.args...)

			// Check error
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

			// Check command routing
			if module.GetLastCommand() != tt.wantCmd {
				t.Errorf("got command %q, want %q", module.GetLastCommand(), tt.wantCmd)
			}

			// Check args
			if !reflect.DeepEqual(module.GetLastArgs(), tt.wantArgs) {
				t.Errorf("got args %v, want %v", module.GetLastArgs(), tt.wantArgs)
			}
		})
	}
}

func TestModuleHelp(t *testing.T) {
	module, cmds, manager := setupTestModule(t)

	tests := []struct {
		name      string
		moduleID  string
		wantError bool
	}{
		{
			name:     "Show help for existing module",
			moduleID: "test",
		},
		{
			name:      "Show help for unknown module",
			moduleID:  "unknown",
			wantError: true,
		},
		{
			name:      "Show help for disabled module",
			moduleID:  "test",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Enable/disable module based on test case
			if tt.name == "Show help for disabled module" {
				if err := manager.DisableModule(module.ID()); err != nil {
					t.Fatalf("Failed to disable module: %v", err)
				}
			} else {
				if err := manager.EnableModule(module.ID()); err != nil {
					t.Fatalf("Failed to enable module: %v", err)
				}
			}

			// Execute help command
			err := cmds.ModuleHelp(tt.moduleID)

			// Check error
			if tt.wantError {
				if err == nil {
					t.Error("expected error got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
