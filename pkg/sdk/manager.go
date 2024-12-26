package sdk

import (
	"fmt"
	"sync"

	"memex/pkg/types"
)

// Manager handles module registration and lifecycle
type Manager struct {
	modules map[string]types.Module
	repo    types.Repository
	events  *EventEmitter
	mu      sync.RWMutex
}

// NewManager creates a new module manager
func NewManager() *Manager {
	return &Manager{
		modules: make(map[string]types.Module),
		events:  NewEventEmitter(),
	}
}

// Events returns the event emitter
func (m *Manager) Events() *EventEmitter {
	return m.events
}

// RegisterModule registers a module with the manager
func (m *Manager) RegisterModule(mod types.Module) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if mod == nil {
		return fmt.Errorf("%w: module is nil", ErrInvalidInput)
	}

	id := mod.ID()
	if id == "" {
		return fmt.Errorf("%w: module ID is required", ErrInvalidInput)
	}

	if _, exists := m.modules[id]; exists {
		return fmt.Errorf("%w: module %s already registered", ErrInvalidInput, id)
	}

	// Initialize module if repository is set
	if m.repo != nil {
		if err := mod.Init(m.repo); err != nil {
			return fmt.Errorf("initializing module %s: %w", id, err)
		}
	}

	m.modules[id] = mod
	m.events.EmitModuleLoaded(mod)
	return nil
}

// GetModule returns a module by ID
func (m *Manager) GetModule(id string) (types.Module, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mod, exists := m.modules[id]
	return mod, exists
}

// ListModules returns all registered modules
func (m *Manager) ListModules() []types.Module {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mods := make([]types.Module, 0, len(m.modules))
	for _, mod := range m.modules {
		mods = append(mods, mod)
	}
	return mods
}

// SetRepository sets the repository for all modules
func (m *Manager) SetRepository(repo types.Repository) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if repo == nil {
		return fmt.Errorf("%w: repository is nil", ErrInvalidInput)
	}

	// Initialize all modules with repository
	for id, mod := range m.modules {
		if err := mod.Init(repo); err != nil {
			return fmt.Errorf("initializing module %s: %w", id, err)
		}
	}

	m.repo = repo
	return nil
}

// HandleCommand routes a command to the appropriate module
func (m *Manager) HandleCommand(moduleID string, cmd string, args []string) error {
	m.mu.RLock()
	mod, exists := m.modules[moduleID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("%w: module %s", ErrNotFound, moduleID)
	}

	m.events.EmitCommandStarted(mod, cmd, args)
	err := mod.HandleCommand(cmd, args)
	if err != nil {
		m.events.EmitCommandError(mod, cmd, args, err)
		return err
	}
	m.events.EmitCommandCompleted(mod, cmd, args)
	return nil
}

// Shutdown shuts down all modules
func (m *Manager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for id, mod := range m.modules {
		if shutdowner, ok := mod.(interface{ Shutdown() error }); ok {
			if err := shutdowner.Shutdown(); err != nil {
				lastErr = fmt.Errorf("shutting down module %s: %w", id, err)
			}
		}
	}

	return lastErr
}
