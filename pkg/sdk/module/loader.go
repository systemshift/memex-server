package module

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"

	"memex/pkg/sdk/types"
)

// PluginLoader handles loading modules from plugin files
type PluginLoader interface {
	// Load loads a module from a plugin file
	Load(path string) (types.Module, error)
}

// DefaultPluginLoader provides a basic plugin loader implementation
type DefaultPluginLoader struct {
	// Repository is needed to initialize modules
	repo types.Repository
}

// NewPluginLoader creates a new plugin loader
func NewPluginLoader() PluginLoader {
	return &DefaultPluginLoader{}
}

// SetRepository sets the repository for module initialization
func (l *DefaultPluginLoader) SetRepository(repo types.Repository) {
	l.repo = repo
}

// Load loads a module from a plugin file
func (l *DefaultPluginLoader) Load(path string) (types.Module, error) {
	// Try direct path first (for dev mode)
	pluginPath := path
	fmt.Fprintf(os.Stderr, "Debug: Looking for plugin at %s\n", pluginPath)
	if _, err := os.Stat(pluginPath); err != nil {
		// Try module.so in directory
		pluginPath = filepath.Join(path, "module.so")
		fmt.Fprintf(os.Stderr, "Debug: Trying alternate path %s\n", pluginPath)
		if _, err := os.Stat(pluginPath); err != nil {
			return nil, fmt.Errorf("plugin not found at %s or %s: %w", path, pluginPath, err)
		}
	}

	// Open plugin
	plug, err := plugin.Open(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("opening plugin: %w", err)
	}

	// Look up NewModule symbol
	newModuleSym, err := plug.Lookup("NewModule")
	if err != nil {
		return nil, fmt.Errorf("looking up NewModule: %w", err)
	}

	// Cast to correct type
	newModule, ok := newModuleSym.(func(types.Repository) types.Module)
	if !ok {
		return nil, fmt.Errorf("invalid module constructor type")
	}

	// Create module instance
	module := newModule(l.repo)
	if module == nil {
		return nil, fmt.Errorf("module creation failed")
	}

	return module, nil
}
