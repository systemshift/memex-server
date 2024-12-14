package test

import (
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex"
)

func TestModuleManager(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "memex-module-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up test environment
	os.Setenv("HOME", tmpDir)

	// Create test module
	moduleDir := filepath.Join(tmpDir, "test-module")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatalf("Failed to create module dir: %v", err)
	}

	// Create module file
	moduleFile := filepath.Join(moduleDir, "module.go")
	if err := os.WriteFile(moduleFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create module file: %v", err)
	}

	// Create repository and module manager
	repo := NewMockRepository()
	manager, err := memex.NewModuleManager()
	if err != nil {
		t.Fatalf("Failed to create module manager: %v", err)
	}
	manager.SetRepository(repo)

	t.Run("Install Module", func(t *testing.T) {
		// Install module
		if err := manager.InstallModule(moduleDir); err != nil {
			t.Errorf("Failed to install module: %v", err)
		}

		// Create and register test module
		module := NewTestModule(repo)
		module.SetID("test-module") // Match the installed module ID
		if err := repo.RegisterModule(module); err != nil {
			t.Fatalf("Failed to register module: %v", err)
		}

		// Verify module was installed
		modules := manager.ListModules()
		found := false
		for _, mod := range modules {
			if mod == "test-module" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Module not found after installation")
		}

		// Verify module configuration
		config, exists := manager.GetModuleConfig("test-module")
		if !exists {
			t.Error("Module config not found")
		}
		if !config.Enabled {
			t.Error("Module should be enabled by default")
		}
		if config.Type != "package" {
			t.Errorf("Wrong module type, got %s, want package", config.Type)
		}

		// Clean up
		repo.UnregisterModule("test-module")
	})

	t.Run("Enable/Disable Module", func(t *testing.T) {
		// Create test module with unique ID
		module := NewTestModule(repo)
		module.SetID("enable-test")
		if err := repo.RegisterModule(module); err != nil {
			t.Fatalf("Failed to register module: %v", err)
		}

		// Disable module
		if err := manager.DisableModule(module.ID()); err != nil {
			t.Errorf("Failed to disable module: %v", err)
		}

		if manager.IsModuleEnabled(module.ID()) {
			t.Error("Module still enabled after disable")
		}

		// Enable module
		if err := manager.EnableModule(module.ID()); err != nil {
			t.Errorf("Failed to enable module: %v", err)
		}

		if !manager.IsModuleEnabled(module.ID()) {
			t.Error("Module not enabled after enable")
		}

		// Clean up
		repo.UnregisterModule(module.ID())
	})

	t.Run("Remove Module", func(t *testing.T) {
		// Install module to remove
		moduleToRemove := filepath.Join(tmpDir, "remove-test")
		if err := os.MkdirAll(moduleToRemove, 0755); err != nil {
			t.Fatalf("Failed to create module dir: %v", err)
		}
		if err := manager.InstallModule(moduleToRemove); err != nil {
			t.Fatalf("Failed to install module: %v", err)
		}

		// Create and register test module
		module := NewTestModule(repo)
		module.SetID("remove-test")
		if err := repo.RegisterModule(module); err != nil {
			t.Fatalf("Failed to register module: %v", err)
		}

		// Remove module
		if err := manager.RemoveModule("remove-test"); err != nil {
			t.Errorf("Failed to remove module: %v", err)
		}

		// Unregister from repository
		if err := repo.UnregisterModule("remove-test"); err != nil {
			t.Fatalf("Failed to unregister module: %v", err)
		}

		// Verify module was removed
		modules := manager.ListModules()
		for _, mod := range modules {
			if mod == "remove-test" {
				t.Error("Module still exists after removal")
			}
		}

		// Verify module configuration was removed
		if _, exists := manager.GetModuleConfig("remove-test"); exists {
			t.Error("Module config still exists after removal")
		}
	})

	t.Run("Error Cases", func(t *testing.T) {
		// Try to install non-existent module
		if err := manager.InstallModule("/nonexistent"); err == nil {
			t.Error("Expected error when installing non-existent module")
		}

		// Try to remove non-existent module
		if err := manager.RemoveModule("nonexistent"); err == nil {
			t.Error("Expected error when removing non-existent module")
		}

		// Try to enable non-existent module
		if err := manager.EnableModule("nonexistent"); err == nil {
			t.Error("Expected error when enabling non-existent module")
		}

		// Try to disable non-existent module
		if err := manager.DisableModule("nonexistent"); err == nil {
			t.Error("Expected error when disabling non-existent module")
		}
	})

	t.Run("Config Persistence", func(t *testing.T) {
		// Clean up any existing modules
		for _, module := range repo.ListModules() {
			repo.UnregisterModule(module.ID())
		}

		// Install module
		if err := manager.InstallModule(moduleDir); err != nil {
			t.Fatalf("Failed to install module: %v", err)
		}

		// Create and register test module
		module := NewTestModule(repo)
		module.SetID("test-module")
		if err := repo.RegisterModule(module); err != nil {
			t.Fatalf("Failed to register module: %v", err)
		}

		// Create new manager instance
		newManager, err := memex.NewModuleManager()
		if err != nil {
			t.Fatalf("Failed to create new module manager: %v", err)
		}
		newManager.SetRepository(repo)

		// Verify module config was loaded
		config, exists := newManager.GetModuleConfig("test-module")
		if !exists {
			t.Error("Module config not persisted")
		}

		absPath, _ := filepath.Abs(moduleDir)
		if config.Path != absPath {
			t.Errorf("Wrong module path, got %s, want %s", config.Path, absPath)
		}

		// Clean up
		repo.UnregisterModule("test-module")
	})
}
