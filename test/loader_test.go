package test

import (
	"testing"

	"memex/pkg/sdk"
	"memex/pkg/types"
)

func TestModuleLoader(t *testing.T) {
	// Create manager and loader
	mgr := sdk.NewManager()
	loader := sdk.NewModuleLoader(mgr)

	// Create test modules
	mod1 := &mockModule{id: "test1", name: "Test 1"}
	mod2 := &mockModule{id: "test2", name: "Test 2"}
	mods := map[string]types.Module{
		"test1": mod1,
		"test2": mod2,
	}

	// Test loading
	t.Run("loading", func(t *testing.T) {
		// Test single module loading
		if err := loader.LoadModule("test1", mod1); err != nil {
			t.Errorf("LoadModule() error = %v", err)
		}

		// Test ID mismatch
		if err := loader.LoadModule("wrong", mod2); err == nil {
			t.Error("LoadModule() should error on ID mismatch")
		}

		// Test nil module
		if err := loader.LoadModule("nil", nil); err == nil {
			t.Error("LoadModule() should error on nil module")
		}

		// Test multiple module loading
		if err := loader.LoadModules(mods); err == nil {
			// Should fail because test1 is already loaded
			t.Error("LoadModules() should error on duplicate module")
		}

		// Verify modules were loaded
		if mod, exists := mgr.GetModule("test1"); !exists {
			t.Error("Module test1 should be loaded")
		} else if mod.ID() != "test1" {
			t.Errorf("Module ID = %v, want test1", mod.ID())
		}
	})

	// Test unloading
	t.Run("unloading", func(t *testing.T) {
		// Test single module unloading
		if err := loader.UnloadModule("test1"); err != nil {
			t.Errorf("UnloadModule() error = %v", err)
		}

		// Verify module was unloaded
		if _, exists := mgr.GetModule("test1"); exists {
			t.Error("Module test1 should be unloaded")
		}

		// Test unloading non-existent module
		if err := loader.UnloadModule("nonexistent"); err == nil {
			t.Error("UnloadModule() should error on non-existent module")
		}
	})

	// Test unloading all
	t.Run("unload all", func(t *testing.T) {
		// Create new manager and loader for this test
		mgr := sdk.NewManager()
		loader := sdk.NewModuleLoader(mgr)

		// Load multiple modules
		if err := loader.LoadModules(mods); err != nil {
			t.Errorf("LoadModules() error = %v", err)
		}

		// Unload all
		if err := loader.UnloadAll(); err != nil {
			t.Errorf("UnloadAll() error = %v", err)
		}

		// Verify all modules were unloaded
		if len(mgr.ListModules()) != 0 {
			t.Error("All modules should be unloaded")
		}
	})

	// Test paths
	t.Run("paths", func(t *testing.T) {
		// Add valid path
		loader.AddPath("test/modules")

		// Add invalid path
		loader.AddPath("")

		// Add duplicate path
		loader.AddPath("test/modules")
	})
}

func TestModuleShutdown(t *testing.T) {
	mgr := sdk.NewManager()
	loader := sdk.NewModuleLoader(mgr)

	// Create module with shutdown
	mod := &mockShutdownModule{
		mockModule: mockModule{id: "test", name: "Test"},
	}

	// Load and unload
	if err := loader.LoadModule("test", mod); err != nil {
		t.Errorf("LoadModule() error = %v", err)
	}

	if err := loader.UnloadModule("test"); err != nil {
		t.Errorf("UnloadModule() error = %v", err)
	}

	// Verify shutdown was called
	if !mod.shutdownCalled {
		t.Error("Shutdown should be called during unload")
	}
}
