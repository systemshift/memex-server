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

	// Initialize module manager
	manager, err := memex.NewModuleManager()
	if err != nil {
		t.Fatalf("Failed to create module manager: %v", err)
	}

	// Test module installation
	t.Run("Install Module", func(t *testing.T) {
		if err := manager.InstallModule(moduleDir); err != nil {
			t.Errorf("Failed to install module: %v", err)
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
	})

	// Test module enabling/disabling
	t.Run("Enable/Disable Module", func(t *testing.T) {
		// Disable module
		if err := manager.DisableModule("test-module"); err != nil {
			t.Errorf("Failed to disable module: %v", err)
		}

		if manager.IsModuleEnabled("test-module") {
			t.Error("Module still enabled after disable")
		}

		// Enable module
		if err := manager.EnableModule("test-module"); err != nil {
			t.Errorf("Failed to enable module: %v", err)
		}

		if !manager.IsModuleEnabled("test-module") {
			t.Error("Module not enabled after enable")
		}
	})

	// Test module removal
	t.Run("Remove Module", func(t *testing.T) {
		if err := manager.RemoveModule("test-module"); err != nil {
			t.Errorf("Failed to remove module: %v", err)
		}

		// Verify module was removed
		modules := manager.ListModules()
		for _, mod := range modules {
			if mod == "test-module" {
				t.Error("Module still exists after removal")
			}
		}

		// Verify module configuration was removed
		if _, exists := manager.GetModuleConfig("test-module"); exists {
			t.Error("Module config still exists after removal")
		}
	})

	// Test error cases
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

	// Test configuration persistence
	t.Run("Config Persistence", func(t *testing.T) {
		// Install module
		if err := manager.InstallModule(moduleDir); err != nil {
			t.Fatalf("Failed to install module: %v", err)
		}

		// Create new manager instance
		newManager, err := memex.NewModuleManager()
		if err != nil {
			t.Fatalf("Failed to create new module manager: %v", err)
		}

		// Verify module config was loaded
		config, exists := newManager.GetModuleConfig("test-module")
		if !exists {
			t.Error("Module config not persisted")
		}
		if config.Path != moduleDir {
			t.Errorf("Wrong module path, got %s, want %s", config.Path, moduleDir)
		}
	})
}
