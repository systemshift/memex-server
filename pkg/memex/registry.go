// Package memex provides the public API for memex
package memex

import (
	"github.com/systemshift/memex/internal/memex/core"
	"github.com/systemshift/memex/internal/memex/modules"
)

// Re-export core types
type (
	Command    = core.Command
	Module     = core.Module
	Repository = core.Repository
)

// RegisterModule registers a module with memex
func RegisterModule(m Module) error {
	return modules.RegisterModule(m)
}

// GetModule returns a module by ID
func GetModule(id string) (Module, bool) {
	return modules.GetModule(id)
}

// ListModules returns all registered modules
func ListModules() []Module {
	return modules.ListModules()
}
