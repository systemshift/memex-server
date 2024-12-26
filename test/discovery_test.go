package test

import (
	"os"
	"path/filepath"
	"testing"

	"memex/pkg/sdk"
	"memex/pkg/types"
)

func TestModuleDiscovery(t *testing.T) {
	// Create test directory structure
	testDir := filepath.Join(t.TempDir(), "modules")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("creating test directory: %v", err)
	}

	// Create manager and loader
	mgr := sdk.NewManager()
	loader := sdk.NewModuleLoader(mgr)
	discovery := sdk.NewModuleDiscovery(loader)

	// Test module validation
	t.Run("validation", func(t *testing.T) {
		// Valid module
		validMod := &mockModule{
			id:          "test",
			name:        "Test Module",
			description: "A test module",
			commands: []types.Command{
				{
					Name:        "test",
					Usage:       "test <arg>",
					Args:        []string{"arg"},
					Description: "Test command",
				},
			},
		}

		// Invalid modules
		noIDMod := &mockModule{}
		noNameMod := &mockModule{id: "test"}
		invalidCmdMod := &mockModule{
			id:   "test",
			name: "Test",
			commands: []types.Command{
				{Name: ""}, // Invalid command
			},
		}

		tests := []struct {
			name    string
			mod     types.Module
			wantErr bool
		}{
			{
				name:    "valid module",
				mod:     validMod,
				wantErr: false,
			},
			{
				name:    "no id",
				mod:     noIDMod,
				wantErr: true,
			},
			{
				name:    "no name",
				mod:     noNameMod,
				wantErr: true,
			},
			{
				name:    "invalid command",
				mod:     invalidCmdMod,
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Test validation directly
				err := discovery.ValidateModule(tt.mod)
				if (err != nil) != tt.wantErr {
					t.Errorf("ValidateModule() error = %v, wantErr %v", err, tt.wantErr)
				}

				// Test validation through loading
				if !tt.wantErr {
					if err := loader.LoadModule(tt.mod.ID(), tt.mod); err != nil {
						t.Errorf("LoadModule() error = %v", err)
					}
				}
			})
		}
	})

	// Test path handling
	t.Run("paths", func(t *testing.T) {
		tests := []struct {
			name     string
			setup    func() (*sdk.ModuleDiscovery, error)
			wantErr  bool
			errCheck func(error) bool
		}{
			{
				name: "empty directory",
				setup: func() (*sdk.ModuleDiscovery, error) {
					mgr := sdk.NewManager()
					loader := sdk.NewModuleLoader(mgr)
					discovery := sdk.NewModuleDiscovery(loader)
					loader.AddPath(testDir)
					return discovery, nil
				},
				wantErr: false,
			},
			{
				name: "non-existent path",
				setup: func() (*sdk.ModuleDiscovery, error) {
					mgr := sdk.NewManager()
					loader := sdk.NewModuleLoader(mgr)
					discovery := sdk.NewModuleDiscovery(loader)
					loader.AddPath("/nonexistent/path")
					return discovery, nil
				},
				wantErr:  true,
				errCheck: os.IsNotExist,
			},
			{
				name: "directory with non-plugin files",
				setup: func() (*sdk.ModuleDiscovery, error) {
					mgr := sdk.NewManager()
					loader := sdk.NewModuleLoader(mgr)
					discovery := sdk.NewModuleDiscovery(loader)

					nonPluginPath := filepath.Join(testDir, "not-a-plugin.txt")
					if err := os.WriteFile(nonPluginPath, []byte("not a plugin"), 0644); err != nil {
						return nil, err
					}

					loader.AddPath(testDir)
					return discovery, nil
				},
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				d, err := tt.setup()
				if err != nil {
					t.Fatalf("setup failed: %v", err)
				}

				err = d.DiscoverModules()
				if tt.wantErr {
					if err == nil {
						t.Error("DiscoverModules() returned nil, want error")
					} else if tt.errCheck != nil && !tt.errCheck(err) {
						t.Errorf("DiscoverModules() error = %v, want error matching os.IsNotExist", err)
					}
				} else if err != nil {
					t.Errorf("DiscoverModules() error = %v", err)
				}
			})
		}
	})

	// Test dev mode functionality
	t.Run("dev mode", func(t *testing.T) {
		// Create manager and loader
		mgr := sdk.NewManager()
		loader := sdk.NewModuleLoader(mgr)

		// Add dev path
		devPath := filepath.Join(testDir, "test-dev")
		loader.AddDevPath("test-dev", devPath)

		// Test dev mode checks
		if !loader.IsDevModule("test-dev") {
			t.Error("IsDevModule() = false, want true")
		}

		if !loader.IsDevModule("test-dev") {
			t.Error("IsDevModule() = false, want true")
		}

		if path, exists := loader.GetDevPath("test-dev"); !exists {
			t.Error("GetDevPath() exists = false, want true")
		} else if path != devPath {
			t.Errorf("GetDevPath() path = %v, want %v", path, devPath)
		}

		// Test non-dev module
		if loader.IsDevModule("other") {
			t.Error("IsDevModule() = true for non-dev module")
		}

		if _, exists := loader.GetDevPath("other"); exists {
			t.Error("GetDevPath() exists = true for non-dev module")
		}
	})
}

// Test plugin loading errors
func TestPluginErrors(t *testing.T) {
	mgr := sdk.NewManager()
	loader := sdk.NewModuleLoader(mgr)
	discovery := sdk.NewModuleDiscovery(loader)

	// Create invalid plugin file
	testDir := t.TempDir()
	invalidPlugin := filepath.Join(testDir, "invalid.so")
	if err := os.WriteFile(invalidPlugin, []byte("invalid plugin"), 0644); err != nil {
		t.Fatalf("creating invalid plugin: %v", err)
	}

	// Test loading invalid plugin
	loader.AddPath(invalidPlugin)
	if err := discovery.DiscoverModules(); err == nil {
		t.Error("DiscoverModules() should error on invalid plugin")
	}
}

// Test command validation
func TestCommandValidation(t *testing.T) {
	tests := []struct {
		name    string
		cmd     types.Command
		wantErr bool
	}{
		{
			name: "valid command",
			cmd: types.Command{
				Name:        "test",
				Usage:       "test <arg>",
				Args:        []string{"arg"},
				Description: "Test command",
			},
			wantErr: false,
		},
		{
			name: "no name",
			cmd: types.Command{
				Usage: "test",
			},
			wantErr: true,
		},
		{
			name: "args without usage",
			cmd: types.Command{
				Name: "test",
				Args: []string{"arg"},
			},
			wantErr: true,
		},
	}

	mgr := sdk.NewManager()
	loader := sdk.NewModuleLoader(mgr)
	discovery := sdk.NewModuleDiscovery(loader)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod := &mockModule{
				id:       "test",
				name:     "Test",
				commands: []types.Command{tt.cmd},
			}

			err := discovery.ValidateModule(mod)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateModule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
