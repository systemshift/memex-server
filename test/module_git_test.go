package test

import (
	"os"
	"path/filepath"
	"testing"

	"memex/internal/memex"
)

func TestGitModuleInstallation(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "memex-git-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME to temp dir for module manager config
	os.Setenv("HOME", tmpDir)

	// Create mock Git system
	mockGit := NewMockGit()
	mockGit.AddRepository("https://github.com/user/repo.git", "mock repository content")
	mockGit.AddRepository("git@github.com:user/repo.git", "mock repository content")
	mockGit.AddRepository("https://gitlab.com/user/repo.git", "mock repository content")

	tests := []struct {
		name        string
		url         string
		wantID      string
		wantType    string
		shouldError bool
	}{
		{
			name:     "HTTPS GitHub URL",
			url:      "https://github.com/user/repo.git",
			wantID:   "repo",
			wantType: "git",
		},
		{
			name:     "SSH GitHub URL",
			url:      "git@github.com:user/repo.git",
			wantID:   "repo",
			wantType: "git",
		},
		{
			name:     "GitLab URL",
			url:      "https://gitlab.com/user/repo.git",
			wantID:   "repo",
			wantType: "git",
		},
		{
			name:     "URL without .git suffix",
			url:      "https://github.com/user/repo",
			wantID:   "repo",
			wantType: "git",
		},
		{
			name:        "Invalid URL",
			url:         "not-a-git-url",
			shouldError: true,
		},
		{
			name:        "Non-existent repository",
			url:         "https://github.com/user/nonexistent-repo.git",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create module manager
			manager, err := memex.NewModuleManager()
			if err != nil {
				t.Fatalf("Failed to create module manager: %v", err)
			}

			// Set mock Git system
			manager.SetGitSystem(mockGit)

			// Create mock repository
			repo := NewMockRepository()
			manager.SetRepository(repo)

			// Install module
			err = manager.InstallModule(tt.url)

			// Check error
			if tt.shouldError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Get module config
			config, exists := manager.GetModuleConfig(tt.wantID)
			if !exists {
				t.Error("module config not found")
				return
			}

			// Check module type
			if config.Type != tt.wantType {
				t.Errorf("got type %q, want %q", config.Type, tt.wantType)
			}

			// Check module directory exists
			moduleDir := filepath.Join(tmpDir, ".config", "memex", "modules", tt.wantID)
			if _, err := os.Stat(moduleDir); os.IsNotExist(err) {
				t.Error("module directory not created")
			}

			// Check Git repository was cloned
			if _, err := os.Stat(filepath.Join(moduleDir, ".git")); os.IsNotExist(err) {
				t.Error(".git directory not found")
			}

			// Check repository content
			contentFile := filepath.Join(moduleDir, "content.txt")
			content, err := os.ReadFile(contentFile)
			if err != nil {
				t.Errorf("failed to read content file: %v", err)
			}
			if string(content) != "mock repository content" {
				t.Errorf("got content %q, want %q", string(content), "mock repository content")
			}
		})
	}
}

func TestGitURLParsing(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantID   string
		isGitURL bool
	}{
		{
			name:     "HTTPS GitHub URL",
			url:      "https://github.com/user/repo.git",
			wantID:   "repo",
			isGitURL: true,
		},
		{
			name:     "SSH GitHub URL",
			url:      "git@github.com:user/repo.git",
			wantID:   "repo",
			isGitURL: true,
		},
		{
			name:     "GitLab URL",
			url:      "https://gitlab.com/user/repo.git",
			wantID:   "repo",
			isGitURL: true,
		},
		{
			name:     "URL without .git suffix",
			url:      "https://github.com/user/repo",
			wantID:   "repo",
			isGitURL: true,
		},
		{
			name:     "Deep repository path",
			url:      "https://github.com/org/group/repo.git",
			wantID:   "repo",
			isGitURL: true,
		},
		{
			name:     "Local path",
			url:      "/path/to/module",
			wantID:   "module",
			isGitURL: false,
		},
		{
			name:     "Relative path",
			url:      "./local/module",
			wantID:   "module",
			isGitURL: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test isGitURL
			if got := memex.IsGitURL(tt.url); got != tt.isGitURL {
				t.Errorf("isGitURL(%q) = %v, want %v", tt.url, got, tt.isGitURL)
			}

			// Test getModuleIDFromGit
			if tt.isGitURL {
				if got := memex.GetModuleIDFromGit(tt.url); got != tt.wantID {
					t.Errorf("getModuleIDFromGit(%q) = %q, want %q", tt.url, got, tt.wantID)
				}
			}
		})
	}
}

func TestGitModuleRemoval(t *testing.T) {
	// Create temporary test directory
	tmpDir, err := os.MkdirTemp("", "memex-git-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME to temp dir for module manager config
	os.Setenv("HOME", tmpDir)

	// Create mock Git system
	mockGit := NewMockGit()
	mockGit.AddRepository("https://github.com/user/test-repo.git", "mock repository content")

	// Create module manager
	manager, err := memex.NewModuleManager()
	if err != nil {
		t.Fatalf("Failed to create module manager: %v", err)
	}

	// Set mock Git system
	manager.SetGitSystem(mockGit)

	// Create mock repository
	repo := NewMockRepository()
	manager.SetRepository(repo)

	// Install test module
	moduleURL := "https://github.com/user/test-repo.git"
	if err := manager.InstallModule(moduleURL); err != nil {
		t.Fatalf("Failed to install module: %v", err)
	}

	// Verify module was installed
	moduleDir := filepath.Join(tmpDir, ".config", "memex", "modules", "test-repo")
	if _, err := os.Stat(moduleDir); os.IsNotExist(err) {
		t.Fatal("module directory not created")
	}

	// Remove module
	if err := manager.RemoveModule("test-repo"); err != nil {
		t.Fatalf("Failed to remove module: %v", err)
	}

	// Verify module directory was removed
	if _, err := os.Stat(moduleDir); !os.IsNotExist(err) {
		t.Error("module directory still exists")
	}

	// Verify module config was removed
	if _, exists := manager.GetModuleConfig("test-repo"); exists {
		t.Error("module config still exists")
	}
}
