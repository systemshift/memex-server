package test

import (
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex"
	"memex/internal/memex/core"
	"memex/internal/memex/repository"
)

func NewTestRepository(t *testing.T) core.Repository {
	dir := t.TempDir()
	repo, err := repository.Create(filepath.Join(dir, "test.mx"))
	if err != nil {
		t.Fatalf("creating repository: %v", err)
	}
	return repo
}

func TestModuleManager(t *testing.T) {
	// Create test repository
	repo := NewTestRepository(t)
	defer repo.Close()

	// Create module manager
	manager, err := memex.NewModuleManager()
	if err != nil {
		t.Fatalf("creating module manager: %v", err)
	}

	// Set repository
	manager.SetRepository(repo)

	// Create test module
	module := NewTestModule(repo)
	module.SetID("test")

	// Register module
	if err := repo.RegisterModule(module); err != nil {
		t.Fatalf("registering module: %v", err)
	}

	// Test module installation
	moduleDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(moduleDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	if err := manager.InstallModule(moduleDir); err != nil {
		t.Fatalf("installing module: %v", err)
	}

	// Test module listing
	modules := manager.ListModules()
	if len(modules) != 1 {
		t.Errorf("expected 1 module, got %d", len(modules))
	}

	// Test module removal
	if err := manager.RemoveModule("test.txt"); err != nil {
		t.Fatalf("removing module: %v", err)
	}

	modules = manager.ListModules()
	if len(modules) != 1 {
		t.Errorf("expected 1 module, got %d", len(modules))
	}
}

func TestModuleCommands(t *testing.T) {
	// Create test repository
	repo := NewTestRepository(t)
	defer repo.Close()

	// Create module manager
	manager, err := memex.NewModuleManager()
	if err != nil {
		t.Fatalf("creating module manager: %v", err)
	}

	// Set repository
	manager.SetRepository(repo)

	// Create test module
	module := NewTestModule(repo)
	module.SetID("test")

	// Register module
	if err := repo.RegisterModule(module); err != nil {
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

	if module.GetLastCommand() != "add" {
		t.Errorf("expected command 'add', got '%s'", module.GetLastCommand())
	}
}
