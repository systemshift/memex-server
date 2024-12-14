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

// GetModuleConfig returns configuration for a module
func (m *ModuleManager) GetModuleConfig(moduleID string) (core.ModuleConfig, bool) {
	return m.config.GetModule(moduleID)
}

// IsModuleEnabled checks if a module is enabled
func (m *ModuleManager) IsModuleEnabled(moduleID string) bool {
	return m.config.IsModuleEnabled(moduleID)
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

	// TODO: Load and validate module
	// For now, just use the directory/file name as the module ID
	moduleID := filepath.Base(path)

	// Create module directory
	moduleDir := filepath.Join(m.modulesDir, moduleID)
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		return fmt.Errorf("creating module directory: %w", err)
	}

	// Add module configuration
	m.config.AddModule(moduleID, core.ModuleConfig{
		Path:     path,
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
	// Check if module exists
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
	modules := make([]string, 0, len(m.config.Modules))
	for id := range m.config.Modules {
		modules = append(modules, id)
	}
	return modules
}

// EnableModule enables a module
func (m *ModuleManager) EnableModule(moduleID string) error {
	if !m.config.EnableModule(moduleID) {
		return fmt.Errorf("module not found: %s", moduleID)
	}
	return m.saveConfig()
}

// DisableModule disables a module
func (m *ModuleManager) DisableModule(moduleID string) error {
	if !m.config.DisableModule(moduleID) {
		return fmt.Errorf("module not found: %s", moduleID)
	}
	return m.saveConfig()
}
