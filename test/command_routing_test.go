package test

import (
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex"
)

func TestModuleCommand(t *testing.T) {
	// Create test repository
	repo := NewTestRepository(t)
	defer repo.Close()

	// Set up test module
	module := NewTestModule(repo)
	module.SetID("test")

	// Register module
	if err := repo.RegisterModule(module); err != nil {
		t.Fatalf("registering module: %v", err)
	}

	// Set repository for module manager
	memex.SetRepository(repo)

	// Test module command routing
	testCases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "no args",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "list modules",
			args:    []string{"list"},
			wantErr: false,
		},
		{
			name:    "install without path",
			args:    []string{"install"},
			wantErr: true,
		},
		{
			name:    "remove without name",
			args:    []string{"remove"},
			wantErr: true,
		},
		{
			name:    "unknown command",
			args:    []string{"unknown"},
			wantErr: true,
		},
		{
			name:    "module command without args",
			args:    []string{"test"},
			wantErr: true,
		},
		{
			name:    "module command",
			args:    []string{"test", "add", "arg1", "arg2"},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := memex.ModuleCommand(tc.args...)
			if tc.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestModuleInstallation(t *testing.T) {
	// Create test repository
	repo := NewTestRepository(t)
	defer repo.Close()

	// Set repository for module manager
	memex.SetRepository(repo)

	// Create test module
	moduleDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(moduleDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	// Test installation
	if err := memex.ModuleCommand("install", moduleDir); err != nil {
		t.Fatalf("installing module: %v", err)
	}

	// Test removal
	if err := memex.ModuleCommand("remove", "test.txt"); err != nil {
		t.Fatalf("removing module: %v", err)
	}
}
