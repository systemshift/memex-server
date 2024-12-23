package test

import (
	"os"
	"path/filepath"
	"testing"

	"memex/pkg/sdk/module"
)

func TestModuleManager(t *testing.T) {
	// Create test repository
	repo := NewMockSDKRepository()

	// Create module manager
	manager, err := module.NewModuleManager()
	if err != nil {
		t.Fatalf("creating module manager: %v", err)
	}

	// Set repository
	manager.SetRepository(repo)

	// Create test module
	testMod := NewTestModule(repo)
	testMod.SetID("test")

	// Register module
	if err := repo.RegisterModule(testMod); err != nil {
		t.Fatalf("registering module: %v", err)
	}

	// Test module installation
	moduleDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(moduleDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	if err := manager.InstallModule(moduleDir, false); err != nil {
		t.Fatalf("installing module: %v", err)
	}

	// Test module listing
	modules := manager.List()
	if len(modules) != 1 {
		t.Errorf("expected 1 module, got %d", len(modules))
	}

	// Test module removal
	if err := manager.RemoveModule("test"); err != nil {
		t.Fatalf("removing module: %v", err)
	}

	modules = manager.List()
	if len(modules) != 0 {
		t.Errorf("expected 0 modules, got %d", len(modules))
	}
}

func TestModuleCommands(t *testing.T) {
	// Create test repository
	repo := NewMockSDKRepository()

	// Create module manager
	manager, err := module.NewModuleManager()
	if err != nil {
		t.Fatalf("creating module manager: %v", err)
	}

	// Set repository
	manager.SetRepository(repo)

	// Create test module
	testMod := NewTestModule(repo)
	testMod.SetID("test")

	// Register module
	if err := repo.RegisterModule(testMod); err != nil {
		t.Fatalf("registering module: %v", err)
	}

	// Test getting commands
	commands, err := manager.GetModuleCommands("test")
	if err != nil {
		t.Fatalf("getting commands: %v", err)
	}

	if len(commands) != 3 { // add, remove, list commands
		t.Errorf("expected 3 commands, got %d", len(commands))
	}

	// Test handling command
	if err := manager.HandleCommand("test", "add", []string{"arg1", "arg2"}); err != nil {
		t.Errorf("handling command: %v", err)
	}

	if testMod.GetLastCommand() != "add" {
		t.Errorf("expected command 'add', got '%s'", testMod.GetLastCommand())
	}
}
