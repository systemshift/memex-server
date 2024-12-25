package sdk

import (
	"fmt"
	"path/filepath"

	"memex/pkg/sdk/types"
)

// ModuleLoader handles module discovery and loading
type ModuleLoader struct {
	manager *Manager
	paths   []string
	events  *EventEmitter
}

// NewModuleLoader creates a new module loader
func NewModuleLoader(manager *Manager) *ModuleLoader {
	return &ModuleLoader{
		manager: manager,
		paths:   make([]string, 0),
		events:  manager.Events(), // Share manager's event emitter
	}
}

// AddPath adds a path to search for modules
func (l *ModuleLoader) AddPath(path string) {
	absPath, err := filepath.Abs(path)
	if err == nil {
		l.paths = append(l.paths, absPath)
	}
}

// LoadModule loads a module by ID
func (l *ModuleLoader) LoadModule(id string, mod types.Module) error {
	if mod == nil {
		return fmt.Errorf("%w: module is nil", ErrInvalidInput)
	}

	if id != mod.ID() {
		return fmt.Errorf("%w: module ID mismatch: %s != %s", ErrInvalidInput, id, mod.ID())
	}

	return l.manager.RegisterModule(mod)
}

// LoadModules loads multiple modules
func (l *ModuleLoader) LoadModules(mods map[string]types.Module) error {
	for id, mod := range mods {
		if err := l.LoadModule(id, mod); err != nil {
			return fmt.Errorf("loading module %s: %w", id, err)
		}
	}
	return nil
}

// UnloadModule unloads a module by ID
func (l *ModuleLoader) UnloadModule(id string) error {
	mod, exists := l.manager.GetModule(id)
	if !exists {
		return fmt.Errorf("%w: module %s", ErrNotFound, id)
	}

	// Call shutdown if supported
	if shutdowner, ok := mod.(interface{ Shutdown() error }); ok {
		if err := shutdowner.Shutdown(); err != nil {
			return fmt.Errorf("shutting down module %s: %w", id, err)
		}
	}

	// Remove from manager
	l.manager.modules[id] = nil
	delete(l.manager.modules, id)

	l.events.EmitModuleUnloaded(mod)
	return nil
}

// UnloadAll unloads all modules
func (l *ModuleLoader) UnloadAll() error {
	var lastErr error
	for id := range l.manager.modules {
		if err := l.UnloadModule(id); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Events returns the event emitter
func (l *ModuleLoader) Events() *EventEmitter {
	return l.events
}
