package sdk

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"

	"memex/pkg/sdk/types"
)

// ModuleDiscovery handles module discovery and validation
type ModuleDiscovery struct {
	loader *ModuleLoader
	events *EventEmitter
}

// NewModuleDiscovery creates a new module discovery instance
func NewModuleDiscovery(loader *ModuleLoader) *ModuleDiscovery {
	return &ModuleDiscovery{
		loader: loader,
		events: loader.Events(), // Share loader's event emitter
	}
}

// Events returns the event emitter
func (d *ModuleDiscovery) Events() *EventEmitter {
	return d.events
}

// DiscoverModules finds and loads modules from the given paths
func (d *ModuleDiscovery) DiscoverModules() error {
	var lastErr error
	for _, path := range d.loader.paths {
		if err := d.discoverInPath(path); err != nil {
			if lastErr == nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

// discoverInPath finds modules in a specific path
func (d *ModuleDiscovery) discoverInPath(path string) error {
	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return err // Return original error for IsNotExist check
		}
		return fmt.Errorf("checking path %s: %w", path, err)
	}

	// Handle single plugin file
	if !info.IsDir() {
		return d.loadPluginFile(path)
	}

	// Handle directory of plugins
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("reading directory %s: %w", path, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip subdirectories
		}

		if filepath.Ext(entry.Name()) == ".so" {
			pluginPath := filepath.Join(path, entry.Name())
			if err := d.loadPluginFile(pluginPath); err != nil {
				return fmt.Errorf("loading plugin %s: %w", pluginPath, err)
			}
		}
	}

	return nil
}

// loadPluginFile loads a single plugin file
func (d *ModuleDiscovery) loadPluginFile(path string) error {
	// Open plugin
	plug, err := plugin.Open(path)
	if err != nil {
		return fmt.Errorf("opening plugin %s: %w", path, err)
	}

	// Look up module symbol
	sym, err := plug.Lookup("Module")
	if err != nil {
		return fmt.Errorf("looking up Module symbol in %s: %w", path, err)
	}

	// Assert module type
	mod, ok := sym.(types.Module)
	if !ok {
		return fmt.Errorf("Module symbol in %s does not implement types.Module", path)
	}

	// Validate module
	if err := d.ValidateModule(mod); err != nil {
		return fmt.Errorf("validating module from %s: %w", path, err)
	}

	// Load module
	if err := d.loader.LoadModule(mod.ID(), mod); err != nil {
		return fmt.Errorf("loading module from %s: %w", path, err)
	}

	return nil
}

// ValidateModule checks if a module meets all requirements
func (d *ModuleDiscovery) ValidateModule(mod types.Module) error {
	// Check required fields
	if mod.ID() == "" {
		return fmt.Errorf("%w: module ID is required", ErrInvalidInput)
	}
	if mod.Name() == "" {
		return fmt.Errorf("%w: module name is required", ErrInvalidInput)
	}

	// Check commands
	cmds := mod.Commands()
	for _, cmd := range cmds {
		if err := d.ValidateCommand(cmd); err != nil {
			return fmt.Errorf("validating command %s: %w", cmd.Name, err)
		}
	}

	return nil
}

// ValidateCommand checks if a command is valid
func (d *ModuleDiscovery) ValidateCommand(cmd types.Command) error {
	// Check required fields
	if cmd.Name == "" {
		return fmt.Errorf("%w: command name is required", ErrInvalidInput)
	}

	// Check args match usage
	// If args are specified, usage must be provided
	if len(cmd.Args) > 0 && cmd.Usage == "" {
		return fmt.Errorf("%w: command usage required when args specified", ErrInvalidInput)
	}

	return nil
}
