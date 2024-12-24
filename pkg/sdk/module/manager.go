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

// GitSystem defines the interface for Git operations
type GitSystem interface {
	Clone(url, targetDir string) error
}

// DefaultGitSystem implements GitSystem using real Git commands
type DefaultGitSystem struct{}

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
	Path     string                 `json:"path"`     // Path to module
	Type     string                 `json:"type"`     // Type of module (git, local, etc)
	Dev      bool                   `json:"dev"`      // Whether module is in development mode
	Settings map[string]interface{} `json:"settings"` // Module-specific settings
}

// Manager handles module operations
type Manager interface {
	// Core operations
	InstallModule(path string, dev bool) error // Install a module from path or URL
	RemoveModule(moduleID string) error        // Remove an installed module
	GetModule(id string) (types.Module, bool)
	List() []string // List installed modules

	// Command handling
	HandleCommand(moduleID string, cmd string, args []string) error
	GetModuleCommands(moduleID string) ([]types.ModuleCommand, error)

	// Configuration
	GetModuleConfig(moduleID string) (Config, bool)
	SetRepository(repo types.Repository)
	SetGitSystem(git GitSystem)
}

// DefaultManager provides a basic module manager implementation
type DefaultManager struct {
	configPath string
	modulesDir string
	registry   Registry
	loader     *DefaultPluginLoader
	config     map[string]Config
	gitSystem  GitSystem
	repo       types.Repository
}

func NewManager(configPath, modulesDir string) (*DefaultManager, error) {
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
		gitSystem:  &DefaultGitSystem{},
	}

	if err := m.loadConfig(); err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	return m, nil
}

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

func (m *DefaultManager) SetGitSystem(git GitSystem) {
	m.gitSystem = git
}

func (m *DefaultManager) SetRepository(repo types.Repository) {
	m.repo = repo
	m.loader.SetRepository(repo)

	// Load all installed modules
	for moduleID, cfg := range m.config {
		if _, ok := m.registry.Get(moduleID); ok {
			// Module already loaded (e.g. in-memory test module)
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
	}
}

func (m *DefaultManager) InstallModule(path string, dev bool) error {
	moduleID := filepath.Base(path)
	moduleType := "local"
	var moduleDir string

	if IsGitURL(path) {
		moduleID = GetModuleIDFromGit(path)
		moduleType = "git"
		moduleDir = filepath.Join(m.modulesDir, moduleID)

		if err := os.MkdirAll(moduleDir, 0o755); err != nil {
			return fmt.Errorf("creating module directory: %w", err)
		}

		if m.gitSystem == nil {
			return fmt.Errorf("no GitSystem set to clone from %s", path)
		}
		if err := m.gitSystem.Clone(path, moduleDir); err != nil {
			return err
		}
	} else {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("checking module path: %w", err)
		}

		if info.IsDir() {
			moduleType = "package"
		} else {
			moduleType = "binary"
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("getting absolute path: %w", err)
		}
		if dev {
			moduleDir = absPath
		} else {
			// Copy module to modules directory
			moduleDir = filepath.Join(m.modulesDir, moduleID)
			if err := os.MkdirAll(moduleDir, 0o755); err != nil {
				return fmt.Errorf("creating module directory: %w", err)
			}
			if err := os.Link(absPath, filepath.Join(moduleDir, "module.so")); err != nil {
				return fmt.Errorf("copying module: %w", err)
			}
		}
	}

	// For tests, allow registering in-memory modules without loading from disk
	if _, ok := m.registry.Get(moduleID); ok {
		m.config[moduleID] = Config{
			Path:     moduleDir,
			Type:     moduleType,
			Dev:      false,
			Settings: make(map[string]interface{}),
		}
		return m.saveConfig()
	}

	// Load and register the module
	mod, err := m.loader.Load(moduleDir)
	if err != nil {
		return fmt.Errorf("loading module: %w", err)
	}

	if err := m.registry.Register(mod); err != nil {
		return fmt.Errorf("registering module: %w", err)
	}

	// Save config
	m.config[moduleID] = Config{
		Path:     moduleDir,
		Type:     moduleType,
		Dev:      dev,
		Settings: make(map[string]interface{}),
	}
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	return nil
}

func (m *DefaultManager) RemoveModule(moduleID string) error {
	if err := m.registry.Remove(moduleID); err != nil {
		return fmt.Errorf("removing from registry: %w", err)
	}

	cfg, exists := m.config[moduleID]
	if exists {
		if strings.HasPrefix(cfg.Path, m.modulesDir) {
			if err := os.RemoveAll(cfg.Path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing module directory: %w", err)
			}
		}
	}

	delete(m.config, moduleID)
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	return nil
}

func (m *DefaultManager) GetModule(id string) (types.Module, bool) {
	return m.registry.Get(id)
}

func (m *DefaultManager) List() []string {
	var ids []string
	for id := range m.config {
		ids = append(ids, id)
	}
	return ids
}

func (m *DefaultManager) HandleCommand(moduleID string, cmd string, args []string) error {
	mod, ok := m.registry.Get(moduleID)
	if !ok {
		return fmt.Errorf("module not found: %s", moduleID)
	}

	return mod.HandleCommand(cmd, args)
}

func (m *DefaultManager) GetModuleCommands(moduleID string) ([]types.ModuleCommand, error) {
	mod, ok := m.registry.Get(moduleID)
	if !ok {
		return nil, fmt.Errorf("module not found: %s", moduleID)
	}
	return mod.Commands(), nil
}

func (m *DefaultManager) GetModuleConfig(moduleID string) (Config, bool) {
	cfg, ok := m.config[moduleID]
	return cfg, ok
}

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

// Helper functions for Git URLs
func IsGitURL(path string) bool {
	return strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "git@") ||
		strings.HasSuffix(path, ".git")
}

func GetModuleIDFromGit(url string) string {
	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	if len(parts) >= 1 {
		return parts[len(parts)-1]
	}
	return url
}
