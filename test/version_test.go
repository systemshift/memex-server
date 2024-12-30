package test

import (
	"os"
	"strings"
	"testing"

	"github.com/systemshift/memex/internal/memex/core"
	"github.com/systemshift/memex/internal/memex/repository"
)

func TestVersionCompatibility(t *testing.T) {
	// Test version parsing
	tests := []struct {
		version    string
		wantMajor  uint8
		wantMinor  uint8
		wantError  bool
		errorMatch string
	}{
		{"1.0", 1, 0, false, ""},
		{"2.1", 2, 1, false, ""},
		{"0.9", 0, 9, false, ""},
		{"1", 0, 0, true, "invalid version format"},
		{"1.a", 0, 0, true, "invalid minor version"},
		{"a.1", 0, 0, true, "invalid major version"},
		{"1.1.1", 0, 0, true, "invalid version format"},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got, err := core.ParseVersion(tt.version)
			if tt.wantError {
				if err == nil {
					t.Errorf("ParseVersion(%q) = %v, want error", tt.version, got)
				} else if tt.errorMatch != "" {
					checkError(t, err, tt.errorMatch)
				}
			} else {
				if err != nil {
					t.Errorf("ParseVersion(%q) error = %v", tt.version, err)
				}
				if got.Major != tt.wantMajor || got.Minor != tt.wantMinor {
					t.Errorf("ParseVersion(%q) = {%d, %d}, want {%d, %d}", tt.version, got.Major, got.Minor, tt.wantMajor, tt.wantMinor)
				}
			}
		})
	}

	// Test version compatibility
	compatTests := []struct {
		current  core.RepositoryVersion
		repo     core.RepositoryVersion
		expected bool
	}{
		// Same versions are compatible
		{core.RepositoryVersion{Major: 1, Minor: 0}, core.RepositoryVersion{Major: 1, Minor: 0}, true},
		// Current version can open older minor versions
		{core.RepositoryVersion{Major: 1, Minor: 1}, core.RepositoryVersion{Major: 1, Minor: 0}, true},
		// Current version cannot open newer minor versions
		{core.RepositoryVersion{Major: 1, Minor: 0}, core.RepositoryVersion{Major: 1, Minor: 1}, false},
		// Different major versions are incompatible
		{core.RepositoryVersion{Major: 1, Minor: 0}, core.RepositoryVersion{Major: 2, Minor: 0}, false},
		{core.RepositoryVersion{Major: 2, Minor: 0}, core.RepositoryVersion{Major: 1, Minor: 0}, false},
	}

	for _, tt := range compatTests {
		t.Run(tt.current.String()+"-"+tt.repo.String(), func(t *testing.T) {
			got := tt.current.IsCompatible(tt.repo)
			if got != tt.expected {
				t.Errorf("IsCompatible(%v, %v) = %v, want %v", tt.current, tt.repo, got, tt.expected)
			}
		})
	}
}

func TestRepositoryVersioning(t *testing.T) {
	testFile := "test_version.mx"
	defer cleanup(t, testFile)

	// Create a repository
	repo, err := repository.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Check version info
	info := repo.GetVersionInfo()
	if info.FormatVersion != core.CurrentVersion.Major {
		t.Errorf("Repository format version = %d, want %d", info.FormatVersion, core.CurrentVersion.Major)
	}
	if info.FormatMinor != core.CurrentVersion.Minor {
		t.Errorf("Repository format minor = %d, want %d", info.FormatMinor, core.CurrentVersion.Minor)
	}

	// Close and reopen repository
	repo.Close()
	repo, err = repository.Open(testFile)
	if err != nil {
		t.Fatalf("Failed to open repository: %v", err)
	}

	// Check version info again
	info = repo.GetVersionInfo()
	if info.FormatVersion != core.CurrentVersion.Major {
		t.Errorf("Reopened repository format version = %d, want %d", info.FormatVersion, core.CurrentVersion.Major)
	}
	if info.FormatMinor != core.CurrentVersion.Minor {
		t.Errorf("Reopened repository format minor = %d, want %d", info.FormatMinor, core.CurrentVersion.Minor)
	}

	// Verify memex version is stored
	if info.MemexVersion == "" {
		t.Error("Repository memex version is empty")
	}
}

func TestIncompatibleVersion(t *testing.T) {
	testFile := "test_incompatible.mx"
	defer cleanup(t, testFile)

	// Create a repository with future version
	repo, err := repository.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Modify version to be incompatible
	repo.GetVersionInfo()
	repo.Close()

	// Try to open with incompatible version
	savedCurrent := core.CurrentVersion
	defer func() { core.CurrentVersion = savedCurrent }()

	core.CurrentVersion = core.RepositoryVersion{Major: 2, Minor: 0}
	_, err = repository.Open(testFile)
	if err == nil {
		t.Error("Expected error opening repository with incompatible version")
	} else {
		checkError(t, err, "incompatible repository version")
	}
}

// cleanup removes test files
func cleanup(t *testing.T, files ...string) {
	for _, f := range files {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			t.Errorf("Failed to clean up %s: %v", f, err)
		}
	}
}

// Helper function to check if error contains expected message
func checkError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Errorf("Expected error containing %q, got nil", want)
		return
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("Expected error containing %q, got %q", want, err.Error())
	}
}
