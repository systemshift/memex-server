package test

import (
	"os"
	"path/filepath"
	"testing"

	"memex/pkg/sdk/module"
)

func TestGitModuleInstallation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "memex-git-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Point HOME to temp dir so the module manager config goes to the test directory
	os.Setenv("HOME", tmpDir)

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
			manager, err := module.NewModuleManager()
			if err != nil {
				t.Fatalf("Failed to create module manager: %v", err)
			}

			manager.SetGitSystem(mockGit)

			repo := NewMockSDKRepository()
			manager.SetRepository(repo)

			err = manager.InstallModule(tt.url, false)
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

			moduleDir := filepath.Join(tmpDir, ".config", "memex", "modules", tt.wantID)
			if _, err := os.Stat(moduleDir); os.IsNotExist(err) {
				t.Error("module directory not created")
			}

			if _, err := os.Stat(filepath.Join(moduleDir, ".git")); os.IsNotExist(err) && !tt.shouldError {
				t.Error(".git directory not found (mock or real clone expected)")
			}

			// Check repository content if your mocks do that
			contentFile := filepath.Join(moduleDir, "content.txt")
			content, readErr := os.ReadFile(contentFile)
			if readErr == nil {
				// This is from your mock system
				if string(content) != "mock repository content" {
					t.Errorf("got content %q, want %q", string(content), "mock repository content")
				}
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
			gotIsGitURL := module.IsGitURL(tt.url)
			if gotIsGitURL != tt.isGitURL {
				t.Errorf("IsGitURL(%q) = %v, want %v", tt.url, gotIsGitURL, tt.isGitURL)
			}

			if tt.isGitURL {
				gotID := module.GetModuleIDFromGit(tt.url)
				if gotID != tt.wantID {
					t.Errorf("GetModuleIDFromGit(%q) = %q, want %q", tt.url, gotID, tt.wantID)
				}
			}
		})
	}
}

func TestGitModuleRemoval(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "memex-git-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.Setenv("HOME", tmpDir)

	mockGit := NewMockGit()
	mockGit.AddRepository("https://github.com/user/test-repo.git", "mock repository content")

	manager, err := module.NewModuleManager()
	if err != nil {
		t.Fatalf("Failed to create module manager: %v", err)
	}

	manager.SetGitSystem(mockGit)

	repo := NewMockSDKRepository()
	manager.SetRepository(repo)

	moduleURL := "https://github.com/user/test-repo.git"
	if err := manager.InstallModule(moduleURL, false); err != nil {
		t.Fatalf("Failed to install module: %v", err)
	}

	moduleDir := filepath.Join(tmpDir, ".config", "memex", "modules", "test-repo")
	if _, err := os.Stat(moduleDir); os.IsNotExist(err) {
		t.Fatal("module directory not created")
	}

	if err := manager.RemoveModule("test-repo"); err != nil {
		t.Fatalf("Failed to remove module: %v", err)
	}

	if _, err := os.Stat(moduleDir); !os.IsNotExist(err) {
		t.Error("module directory still exists")
	}
}
