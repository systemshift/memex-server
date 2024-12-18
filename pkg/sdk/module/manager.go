package module

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"memex/pkg/sdk/types"
)

// GitSystem defines the interface for Git operations (used for modules installed from Git).
type GitSystem interface {
	Clone(url, targetDir string) error
}

// DefaultGitSystem implements GitSystem using real Git commands
type DefaultGitSystem struct{}

// Clone runs "git clone" in a subprocess
func (g *DefaultGitSystem) Clone(url, targetDir string) error {
	cmd := exec.Command("git", "clone", url, targetDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, output)
	}
	return nil
}

// Config represents module configuration
type Config struct {
	Path     string                 `json:"path"`
	Type     string                 `json:"type"`
	Enabled  bool                   `json:"enabled"` // Whether module is enabled
	Settings map[string]interface{} `json:"settings"`
}

// Manager handles module operations
type Manager interface {
	// Load loads a module from a path
	Load(path string) error

	// Get returns a loaded module by ID
	Get(id string) (types.Module, bool)

	// List returns all loaded modules
	List() []types.Module

	// Remove(id string) removes a module
	Remove(id string) error

	// HandleCommand handles a module command
	HandleCommand(moduleID string, cmd string, args []string) error

	// SetRepository sets the repository for all modules
	SetRepository(repo types.Repository)

	// Additional helper methods used by CLI/tests:
	SetGitSystem(git GitSystem)
	InstallModule(path string) error
	RemoveModule(moduleID string) error
	GetModuleConfig(moduleID string) (Config, bool)
}

// DefaultManager provides a basic module manager implementation
type DefaultManager struct {
	configPath string
	modulesDir string
	registry   Registry
	loader     *DefaultPluginLoader
	config     map[string]Config

	gitSystem GitSystem // optional
}

// NewManager creates a new module manager pointing to the given paths
func NewManager(configPath, modulesDir string) (*DefaultManager, error) {
	// Create directories if needed
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.MkdirAll(modulesDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating modules directory: %w", err)
	}

	m := &DefaultManager{
		configPath: configPath,
		modulesDir: modulesDir,
		registry:   NewRegistry(),
		loader:     NewPluginLoader().(*DefaultPluginLoader),
		config:     make(map[string]Config),
		gitSystem:  &DefaultGitSystem{}, // fallback
	}

	// Load config if it exists
	if err := m.loadConfig(); err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	return m, nil
}

// NewModuleManager is an opinionated helper that creates a manager using ~/.config/memex/modules.json and ~/.config/memex/modules
func NewModuleManager() (*DefaultManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "memex")
	configPath := filepath.Join(configDir, "modules.json")
	modulesDir := filepath.Join(configDir, "modules")
	return NewManager(configPath, modulesDir)
}

// SetGitSystem sets the Git system implementation
func (m *DefaultManager) SetGitSystem(git GitSystem) {
	m.gitSystem = git
}

// IsGitURL checks if a path is a Git URL
func IsGitURL(path string) bool {
	return strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "git@") ||
		strings.HasSuffix(path, ".git")
}

// GetModuleIDFromGit extracts module ID from Git URL
func GetModuleIDFromGit(url string) string {
	// Remove .git suffix if present
	url = strings.TrimSuffix(url, ".git")

	// Extract repo name from URL
	parts := strings.Split(url, "/")
	if len(parts) >= 1 {
		return parts[len(parts)-1]
	}
	return url
}

// InstallModule installs a module from a path or Git URL
func (m *DefaultManager) InstallModule(path string) error {
	moduleID := filepath.Base(path)
	moduleType := "local"
	moduleDir := filepath.Join(m.modulesDir, moduleID)

	if IsGitURL(path) {
		moduleID = GetModuleIDFromGit(path)
		moduleType = "git"
		moduleDir = filepath.Join(m.modulesDir, moduleID)

		// Create module directory
		if err := os.MkdirAll(moduleDir, 0o755); err != nil {
			return fmt.Errorf("creating module directory: %w", err)
		}

		// Clone repository using m.gitSystem
		if m.gitSystem == nil {
			return fmt.Errorf("no GitSystem set to clone from %s", path)
		}
		if err := m.gitSystem.Clone(path, moduleDir); err != nil {
			return err
		}
	} else {
		// Local or some other type
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("checking module path: %w", err)
		}

		if info.IsDir() {
			moduleType = "package"
		} else {
			moduleType = "binary"
		}

		// Use absolute path
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("getting absolute path: %w", err)
		}
		moduleDir = absPath
	}

	// Save config
	m.config[moduleID] = Config{
		Path:     moduleDir,
		Type:     moduleType,
		Enabled:  true,
		Settings: make(map[string]interface{}),
	}
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	return nil
}

// RemoveModule removes a module by ID
func (m *DefaultManager) RemoveModule(moduleID string) error {
	return m.Remove(moduleID)
}

// GetModuleConfig returns module config by ID
func (m *DefaultManager) GetModuleConfig(moduleID string) (Config, bool) {
	cfg, ok := m.config[moduleID]
	return cfg, ok
}

// SetRepository sets the repository for all modules
func (m *DefaultManager) SetRepository(repo types.Repository) {
	m.loader.SetRepository(repo)
	// Load all modules in config
	for moduleID, cfg := range m.config {
		if !cfg.Enabled {
			continue
		}
		mod, err := m.loader.Load(cfg.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load module %s: %v\n", moduleID, err)
			continue
		}

		if err := m.registry.Register(mod); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to register module %s: %v\n", moduleID, err)
			continue
		}

		fmt.Fprintf(os.Stderr, "Debug: Successfully loaded and registered module %s\n", moduleID)
	}
}

// Load loads a module from a path
func (m *DefaultManager) Load(path string) error {
	moduleID := filepath.Base(path)
	moduleDir := filepath.Join(m.modulesDir, moduleID)

	// Attempt to load the plugin
	mod, err := m.loader.Load(moduleDir)
	if err != nil {
		return fmt.Errorf("loading module: %w", err)
	}

	if err := m.registry.Register(mod); err != nil {
		return fmt.Errorf("registering module: %w", err)
	}

	return nil
}

// Get returns a loaded module by ID
func (m *DefaultManager) Get(id string) (types.Module, bool) {
	return m.registry.Get(id)
}

// List returns all loaded modules
func (m *DefaultManager) List() []types.Module {
	return m.registry.List()
}

// Remove removes a module using the manager's built-in function
func (m *DefaultManager) Remove(id string) error {
	// Remove from registry
	if err := m.registry.Remove(id); err != nil {
		return fmt.Errorf("removing from registry: %w", err)
	}

	// Remove module directory
	moduleDir := filepath.Join(m.modulesDir, id)
	if err := os.RemoveAll(moduleDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing module directory: %w", err)
	}

	// Remove from config
	delete(m.config, id)

	// Save config
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	return nil
}

// HandleCommand handles a module command
func (m *DefaultManager) HandleCommand(moduleID string, cmd string, args []string) error {
	// Check if module is enabled
	cfg, exists := m.config[moduleID]
	if !exists || !cfg.Enabled {
		return fmt.Errorf("module is not enabled: %s", moduleID)
	}

	// Get module from registry
	mod, ok := m.registry.Get(moduleID)
	if !ok {
		return fmt.Errorf("module not found: %s", moduleID)
	}

	return mod.HandleCommand(cmd, args)
}

// loadConfig loads the module configuration
func (m *DefaultManager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return m.saveConfig()
		}
		return fmt.Errorf("reading config: %w", err)
	}

	if err := json.Unmarshal(data, &m.config); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	return nil
}

// saveConfig saves the module configuration
func (m *DefaultManager) saveConfig() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}
