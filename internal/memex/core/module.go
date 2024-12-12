package core

import (
	"fmt"
)

// ModuleCapability represents a specific capability a module provides
type ModuleCapability string

// Module represents a special app module that can be registered with Memex
type Module interface {
	// ID returns the unique identifier for this module
	ID() string

	// Name returns the human-readable name of this module
	Name() string

	// Description returns a description of what this module does
	Description() string

	// Capabilities returns the list of capabilities this module provides
	Capabilities() []ModuleCapability

	// ValidateNodeType checks if a node type is valid for this module
	ValidateNodeType(nodeType string) bool

	// ValidateLinkType checks if a link type is valid for this module
	ValidateLinkType(linkType string) bool

	// ValidateMetadata validates module-specific metadata
	ValidateMetadata(meta map[string]interface{}) error
}

// ModuleRegistry manages registered modules
type ModuleRegistry struct {
	modules map[string]Module
}

// NewModuleRegistry creates a new module registry
func NewModuleRegistry() *ModuleRegistry {
	return &ModuleRegistry{
		modules: make(map[string]Module),
	}
}

// RegisterModule registers a new module
func (r *ModuleRegistry) RegisterModule(module Module) error {
	if _, exists := r.modules[module.ID()]; exists {
		return fmt.Errorf("module %s already registered", module.ID())
	}
	r.modules[module.ID()] = module
	return nil
}

// GetModule returns a registered module by ID
func (r *ModuleRegistry) GetModule(id string) (Module, bool) {
	module, exists := r.modules[id]
	return module, exists
}

// ListModules returns all registered modules
func (r *ModuleRegistry) ListModules() []Module {
	modules := make([]Module, 0, len(r.modules))
	for _, module := range r.modules {
		modules = append(modules, module)
	}
	return modules
}

// ValidateNodeType checks if a node type is valid for any registered module
func (r *ModuleRegistry) ValidateNodeType(nodeType string) bool {
	for _, module := range r.modules {
		if module.ValidateNodeType(nodeType) {
			return true
		}
	}
	return false
}

// ValidateLinkType checks if a link type is valid for any registered module
func (r *ModuleRegistry) ValidateLinkType(linkType string) bool {
	for _, module := range r.modules {
		if module.ValidateLinkType(linkType) {
			return true
		}
	}
	return false
}

// ValidateMetadata validates metadata against all relevant modules
func (r *ModuleRegistry) ValidateMetadata(meta map[string]interface{}) error {
	if moduleID, ok := meta["module"].(string); ok {
		if module, exists := r.modules[moduleID]; exists {
			return module.ValidateMetadata(meta)
		}
		return fmt.Errorf("module %s not found", moduleID)
	}
	return nil
}
