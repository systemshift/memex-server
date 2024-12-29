package memex

import (
	"fmt"
	"sync"

	"github.com/systemshift/memex/internal/memex/core"
)

// Registry manages registered modules
type Registry struct {
	modules map[string]core.Module
	mu      sync.RWMutex
}

// NewRegistry creates a new module registry
func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]core.Module),
	}
}

// RegisterModule registers a module
func (r *Registry) RegisterModule(module core.Module) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.modules[module.ID()]; exists {
		return fmt.Errorf("module already registered: %s", module.ID())
	}

	r.modules[module.ID()] = module
	return nil
}

// GetModule returns a module by ID
func (r *Registry) GetModule(id string) (core.Module, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	module, exists := r.modules[id]
	return module, exists
}

// ListModules returns all registered modules
func (r *Registry) ListModules() []core.Module {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]core.Module, 0, len(r.modules))
	for _, mod := range r.modules {
		result = append(result, mod)
	}
	return result
}

// HandleCommand executes a module command
func (r *Registry) HandleCommand(moduleID string, cmd string, args []string, repo core.Repository) error {
	module, exists := r.GetModule(moduleID)
	if !exists {
		return fmt.Errorf("module not found: %s", moduleID)
	}

	// Initialize module with repository if needed
	if err := module.Init(repo); err != nil {
		return fmt.Errorf("initializing module: %w", err)
	}

	return module.HandleCommand(cmd, args)
}
