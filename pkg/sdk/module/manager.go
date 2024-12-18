package module

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"memex/pkg/sdk/types"
)

// Manager handles module operations
type Manager interface {
	// Load loads a module from a path
	Load(path string) error

	// Get returns a loaded module by ID
	Get(id string) (types.Module, bool)

	// List returns all loaded modules
	List() []types.Module

	// Remove removes a module
	Remove(id string) error

	// SetRepository sets the repository for all modules
	SetRepository(repo types.Repository)

	// HandleCommand handles a module command
	HandleCommand(moduleID string, cmd string, args []string) error
}

// Config represents module configuration
type Config struct {
	Path     string                 `json:"path"`
	Type     string                 `json:"type"`
	Enabled  bool                   `json:"enabled"` // Whether module is enabled
	Settings map[string]interface{} `json:"settings"`
}

// DefaultManager provides a basic module manager implementation
type DefaultManager struct {
	configPath string
	modulesDir string
	registry   Registry
	loader     *DefaultPluginLoader
	config     map[string]Config
}

// NewManager creates a new module manager
func NewManager(configPath, modulesDir string) (*DefaultManager, error) {
	// Create directories if needed
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return nil, fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		return nil, fmt.Errorf("creating modules directory: %w", err)
	}

	m := &DefaultManager{
		configPath: configPath,
		modulesDir: modulesDir,
		registry:   NewRegistry(),
		loader:     NewPluginLoader().(*DefaultPluginLoader),
		config:     make(map[string]Config),
	}

	// Load config if it exists
	if err := m.loadConfig(); err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	return m, nil
}

// SetRepository sets the repository for all modules
func (m *DefaultManager) SetRepository(repo types.Repository) {
	// Set repository for loader
	m.loader.SetRepository(repo)

	// Load all modules in config
	for moduleID, config := range m.config {
		// Try to load module
		module, err := m.loader.Load(config.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load module %s: %v\n", moduleID, err)
			continue
		}

		// Register module
		if err := m.registry.Register(module); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to register module %s: %v\n", moduleID, err)
			continue
		}

		fmt.Fprintf(os.Stderr, "Debug: Successfully loaded and registered module %s\n", moduleID)
	}
}

// Load loads a module from a path
func (m *DefaultManager) Load(path string) error {
	// Get module ID from path
	moduleID := filepath.Base(path)

	// Create module directory
	moduleDir := filepath.Join(m.modulesDir, moduleID)
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		return fmt.Errorf("creating module directory: %w", err)
	}

	// Copy module files
	// TODO: Implement file copying or git cloning

	// Load module
	module, err := m.loader.Load(moduleDir)
	if err != nil {
		return fmt.Errorf("loading module: %w", err)
	}

	// Register module
	if err := m.registry.Register(module); err != nil {
		return fmt.Errorf("registering module: %w", err)
	}

	// Update config
	m.config[moduleID] = Config{
		Path:     moduleDir,
		Type:     "plugin",
		Enabled:  true,
		Settings: make(map[string]interface{}),
	}

	// Save config
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("saving config: %w", err)
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

// Remove removes a module
func (m *DefaultManager) Remove(id string) error {
	// Remove from registry
	if err := m.registry.Remove(id); err != nil {
		return fmt.Errorf("removing from registry: %w", err)
	}

	// Remove module directory
	moduleDir := filepath.Join(m.modulesDir, id)
	if err := os.RemoveAll(moduleDir); err != nil {
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

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// HandleCommand handles a module command
func (m *DefaultManager) HandleCommand(moduleID string, cmd string, args []string) error {
	// Check if module is enabled
	config, exists := m.config[moduleID]
	if !exists || !config.Enabled {
		return fmt.Errorf("module is not enabled: %s", moduleID)
	}

	// Get module from registry
	module, exists := m.registry.Get(moduleID)
	if !exists {
		return fmt.Errorf("module not found: %s", moduleID)
	}

	return module.HandleCommand(cmd, args)
}
