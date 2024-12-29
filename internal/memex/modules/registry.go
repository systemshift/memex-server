package modules

import (
	"fmt"

	"github.com/systemshift/memex/internal/memex/core"
)

// Registry manages registered modules
type Registry struct {
	modules map[string]core.Module
}

// NewRegistry creates a new registry
func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]core.Module),
	}
}

// RegisterModule registers a module
func (r *Registry) RegisterModule(m core.Module) error {
	if _, exists := r.modules[m.ID()]; exists {
		return fmt.Errorf("module already registered: %s", m.ID())
	}
	r.modules[m.ID()] = m
	return nil
}

// GetModule returns a module by ID
func (r *Registry) GetModule(id string) (core.Module, bool) {
	m, exists := r.modules[id]
	return m, exists
}

// ListModules returns all registered modules
func (r *Registry) ListModules() []core.Module {
	modules := make([]core.Module, 0, len(r.modules))
	for _, m := range r.modules {
		modules = append(modules, m)
	}
	return modules
}

// Global registry instance
var DefaultRegistry = NewRegistry()

// Helper functions that use the default registry

func RegisterModule(m core.Module) error {
	return DefaultRegistry.RegisterModule(m)
}

func GetModule(id string) (core.Module, bool) {
	return DefaultRegistry.GetModule(id)
}

func ListModules() []core.Module {
	return DefaultRegistry.ListModules()
}
