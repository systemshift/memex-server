package memex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"memex/internal/memex/core"
)

// Default paths
const (
	DefaultConfigDir  = ".config/memex"
	ModulesConfigFile = "modules.json"
	ModulesDir        = "modules"
)

// ModuleManager handles module installation and configuration
type ModuleManager struct {
	config     *core.ModulesConfig
	configPath string
	modulesDir string
	repo       core.Repository
}

// NewModuleManager creates a new module manager
func NewModuleManager() (*ModuleManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, DefaultConfigDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("creating config directory: %w", err)
	}

	modulesDir := filepath.Join(configDir, ModulesDir)
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		return nil, fmt.Errorf("creating modules directory: %w", err)
	}

	manager := &ModuleManager{
		configPath: filepath.Join(configDir, ModulesConfigFile),
		modulesDir: modulesDir,
	}

	// Load or create config
	if err := manager.loadConfig(); err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	return manager, nil
}

// SetRepository sets the repository for module operations
func (m *ModuleManager) SetRepository(repo core.Repository) {
	m.repo = repo
	// Sync config with repository state
	if repo != nil {
		for _, module := range repo.ListModules() {
			moduleID := module.ID()
			if _, exists := m.config.GetModule(moduleID); !exists {
				m.config.AddModule(moduleID, core.ModuleConfig{
					Path:     moduleID,
					Type:     "package",
					Enabled:  true,
					Settings: make(map[string]interface{}),
				})
			}
		}
		m.saveConfig()
	}
}

// GetModuleCommands returns available commands for a module
func (m *ModuleManager) GetModuleCommands(moduleID string) ([]core.ModuleCommand, error) {
	if m.repo == nil {
		return nil, fmt.Errorf("no repository connected")
	}

	module, exists := m.repo.GetModule(moduleID)
	if !exists {
		return nil, fmt.Errorf("module not found: %s", moduleID)
	}

	if !m.IsModuleEnabled(moduleID) {
		return nil, fmt.Errorf("module not enabled: %s", moduleID)
	}

	return module.Commands(), nil
}

// HandleCommand handles a module command
func (m *ModuleManager) HandleCommand(moduleID string, cmd string, args []string) error {
	if m.repo == nil {
		return fmt.Errorf("no repository connected")
	}

	module, exists := m.repo.GetModule(moduleID)
	if !exists {
		return fmt.Errorf("module not found: %s", moduleID)
	}

	if !m.IsModuleEnabled(moduleID) {
		return fmt.Errorf("module not enabled: %s", moduleID)
	}

	return module.HandleCommand(cmd, args)
}

// loadConfig loads the modules configuration file
func (m *ModuleManager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config
			m.config = core.DefaultModulesConfig()
			return m.saveConfig()
		}
		return fmt.Errorf("reading config: %w", err)
	}

	var config core.ModulesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	m.config = &config
	return nil
}

// saveConfig saves the modules configuration file
func (m *ModuleManager) saveConfig() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// InstallModule installs a module from a path
func (m *ModuleManager) InstallModule(path string) error {
	// Validate module path
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("checking module path: %w", err)
	}

	// Read module metadata
	var moduleType string
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

	// Use directory/file name as the module ID
	moduleID := filepath.Base(path)

	// Create module directory
	moduleDir := filepath.Join(m.modulesDir, moduleID)
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		return fmt.Errorf("creating module directory: %w", err)
	}

	// Add module configuration
	m.config.AddModule(moduleID, core.ModuleConfig{
		Path:     absPath,
		Type:     moduleType,
		Enabled:  true,
		Settings: make(map[string]interface{}),
	})

	// Save updated configuration
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	return nil
}

// RemoveModule removes a module
func (m *ModuleManager) RemoveModule(moduleID string) error {
	// Check if module exists in config
	if _, exists := m.config.GetModule(moduleID); !exists {
		return fmt.Errorf("module not found: %s", moduleID)
	}

	// Remove module directory
	moduleDir := filepath.Join(m.modulesDir, moduleID)
	if err := os.RemoveAll(moduleDir); err != nil {
		return fmt.Errorf("removing module directory: %w", err)
	}

	// Remove from configuration
	m.config.RemoveModule(moduleID)

	// Save updated configuration
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	return nil
}

// ListModules returns list of installed modules
func (m *ModuleManager) ListModules() []string {
	if m.repo == nil {
		return nil
	}

	modules := m.repo.ListModules()
	result := make([]string, len(modules))
	for i, module := range modules {
		result[i] = module.ID()
	}
	return result
}

// EnableModule enables a module
func (m *ModuleManager) EnableModule(moduleID string) error {
	if m.repo == nil {
		return fmt.Errorf("no repository connected")
	}

	if _, exists := m.repo.GetModule(moduleID); !exists {
		return fmt.Errorf("module not found: %s", moduleID)
	}

	if !m.config.EnableModule(moduleID) {
		// Add module to config if it doesn't exist
		m.config.AddModule(moduleID, core.ModuleConfig{
			Path:     moduleID,
			Type:     "package",
			Enabled:  true,
			Settings: make(map[string]interface{}),
		})
	}

	return m.saveConfig()
}

// DisableModule disables a module
func (m *ModuleManager) DisableModule(moduleID string) error {
	if m.repo == nil {
		return fmt.Errorf("no repository connected")
	}

	if _, exists := m.repo.GetModule(moduleID); !exists {
		return fmt.Errorf("module not found: %s", moduleID)
	}

	if !m.config.DisableModule(moduleID) {
		// Add module to config if it doesn't exist
		m.config.AddModule(moduleID, core.ModuleConfig{
			Path:     moduleID,
			Type:     "package",
			Enabled:  false,
			Settings: make(map[string]interface{}),
		})
	}

	return m.saveConfig()
}

// IsModuleEnabled checks if a module is enabled
func (m *ModuleManager) IsModuleEnabled(moduleID string) bool {
	if m.repo == nil {
		return false
	}

	if _, exists := m.repo.GetModule(moduleID); !exists {
		return false
	}

	return m.config.IsModuleEnabled(moduleID)
}

// GetModuleConfig returns configuration for a module
func (m *ModuleManager) GetModuleConfig(moduleID string) (core.ModuleConfig, bool) {
	return m.config.GetModule(moduleID)
}
