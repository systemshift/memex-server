package core

// ModuleConfig represents configuration for a single module
type ModuleConfig struct {
	Path     string                 `json:"path"`     // Path to module (local path or git URL)
	Type     string                 `json:"type"`     // Module type (package or binary)
	Enabled  bool                   `json:"enabled"`  // Whether module is enabled
	Settings map[string]interface{} `json:"settings"` // Module-specific settings
}

// ModulesConfig represents configuration for all modules
type ModulesConfig struct {
	Modules map[string]ModuleConfig `json:"modules"` // Map of module ID to config
}

// DefaultModulesConfig returns default module configuration
func DefaultModulesConfig() *ModulesConfig {
	return &ModulesConfig{
		Modules: make(map[string]ModuleConfig),
	}
}

// AddModule adds or updates a module configuration
func (c *ModulesConfig) AddModule(id string, config ModuleConfig) {
	c.Modules[id] = config
}

// RemoveModule removes a module configuration
func (c *ModulesConfig) RemoveModule(id string) {
	delete(c.Modules, id)
}

// GetModule returns configuration for a module
func (c *ModulesConfig) GetModule(id string) (ModuleConfig, bool) {
	config, exists := c.Modules[id]
	return config, exists
}

// IsModuleEnabled checks if a module is enabled
func (c *ModulesConfig) IsModuleEnabled(id string) bool {
	if config, exists := c.Modules[id]; exists {
		return config.Enabled
	}
	return false
}

// EnableModule enables a module
func (c *ModulesConfig) EnableModule(id string) bool {
	if config, exists := c.Modules[id]; exists {
		config.Enabled = true
		c.Modules[id] = config
		return true
	}
	return false
}

// DisableModule disables a module
func (c *ModulesConfig) DisableModule(id string) bool {
	if config, exists := c.Modules[id]; exists {
		config.Enabled = false
		c.Modules[id] = config
		return true
	}
	return false
}
