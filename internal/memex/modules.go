package memex

import (
	"github.com/systemshift/memex/internal/memex/core"
)

var (
	// Global module registry
	moduleRegistry = NewRegistry()
)

// RegisterModule registers a module globally
func RegisterModule(module core.Module) error {
	return moduleRegistry.RegisterModule(module)
}

// GetModule returns a module by ID
func GetModule(id string) (core.Module, bool) {
	return moduleRegistry.GetModule(id)
}

// ListModules returns all registered modules
func ListModules() []core.Module {
	return moduleRegistry.ListModules()
}

// HandleModuleCommand executes a module command
func HandleModuleCommand(moduleID string, cmd string, args []string, repo core.Repository) error {
	return moduleRegistry.HandleCommand(moduleID, cmd, args, repo)
}
