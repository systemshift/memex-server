package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MockGit provides a mock implementation for Git operations
type MockGit struct {
	// Map of URL to content for simulating repositories
	repositories map[string]string
}

// NewMockGit creates a new mock Git system
func NewMockGit() *MockGit {
	return &MockGit{
		repositories: make(map[string]string),
	}
}

// AddRepository adds a mock repository
func (g *MockGit) AddRepository(url, content string) {
	g.repositories[url] = content
	// Also add version without .git suffix
	if strings.HasSuffix(url, ".git") {
		urlWithoutGit := strings.TrimSuffix(url, ".git")
		g.repositories[urlWithoutGit] = content
	}
}

// Clone simulates cloning a repository
func (g *MockGit) Clone(url, targetDir string) error {
	content, exists := g.repositories[url]
	if !exists {
		// Try with .git suffix if not found
		if !strings.HasSuffix(url, ".git") {
			content, exists = g.repositories[url+".git"]
		}
		if !exists {
			return fmt.Errorf("repository not found: %s", url)
		}
	}

	// Create .git directory to simulate Git repository
	gitDir := filepath.Join(targetDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		return err
	}

	// Create a mock file to simulate repository content
	contentFile := filepath.Join(targetDir, "content.txt")
	if err := os.WriteFile(contentFile, []byte(content), 0644); err != nil {
		return err
	}

	return nil
}
