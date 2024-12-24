package sdk

import (
	"fmt"
	"sync"

	"memex/pkg/sdk/types"
)

var (
	registry = &moduleRegistry{
		modules: make(map[string]types.Module),
	}
)

type moduleRegistry struct {
	mu      sync.RWMutex
	modules map[string]types.Module
}

// RegisterModule registers a module with memex
func RegisterModule(module types.Module) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	if _, exists := registry.modules[module.ID()]; exists {
		return fmt.Errorf("module already registered: %s", module.ID())
	}

	registry.modules[module.ID()] = module
	return nil
}

// GetModule returns a registered module by ID
func GetModule(id string) (types.Module, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	module, exists := registry.modules[id]
	return module, exists
}

// ListModules returns all registered modules
func ListModules() []types.Module {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	modules := make([]types.Module, 0, len(registry.modules))
	for _, module := range registry.modules {
		modules = append(modules, module)
	}
	return modules
}
