package module

import (
	"fmt"
	"sync"

	"memex/pkg/sdk/types"
)

// Registry manages module registration and lookup
type Registry interface {
	// Register registers a module
	Register(module types.Module) error

	// Get returns a module by ID
	Get(id string) (types.Module, bool)

	// List returns all registered modules
	List() []types.Module

	// Remove removes a module
	Remove(id string) error
}

// DefaultRegistry provides a basic registry implementation
type DefaultRegistry struct {
	modules map[string]types.Module
	mu      sync.RWMutex
}

// NewRegistry creates a new module registry
func NewRegistry() Registry {
	return &DefaultRegistry{
		modules: make(map[string]types.Module),
	}
}

// Register registers a module
func (r *DefaultRegistry) Register(module types.Module) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.modules[module.ID()]; exists {
		return fmt.Errorf("module already registered: %s", module.ID())
	}

	r.modules[module.ID()] = module
	return nil
}

// Get returns a module by ID
func (r *DefaultRegistry) Get(id string) (types.Module, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	module, exists := r.modules[id]
	return module, exists
}

// List returns all registered modules
func (r *DefaultRegistry) List() []types.Module {
	r.mu.RLock()
	defer r.mu.RUnlock()

	modules := make([]types.Module, 0, len(r.modules))
	for _, module := range r.modules {
		modules = append(modules, module)
	}
	return modules
}

// Remove removes a module
func (r *DefaultRegistry) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.modules[id]; !exists {
		return fmt.Errorf("module not found: %s", id)
	}

	delete(r.modules, id)
	return nil
}
