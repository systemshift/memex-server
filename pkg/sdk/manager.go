package sdk

import (
	"fmt"
	"sync"

	"memex/pkg/sdk/types"
)

// Manager handles module operations
type Manager struct {
	mu      sync.RWMutex
	modules map[string]types.Module
	repo    types.Repository
}

// NewManager creates a new module manager
func NewManager() *Manager {
	return &Manager{
		modules: make(map[string]types.Module),
	}
}

// RegisterModule registers a module with the manager
func (m *Manager) RegisterModule(module types.Module) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.modules[module.ID()]; exists {
		return fmt.Errorf("module already registered: %s", module.ID())
	}

	// Initialize module if repository is set
	if m.repo != nil {
		if err := module.Init(m.repo); err != nil {
			return fmt.Errorf("initializing module: %w", err)
		}
	}

	m.modules[module.ID()] = module
	return nil
}

// UnregisterModule removes a module from the manager
func (m *Manager) UnregisterModule(moduleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.modules[moduleID]; !exists {
		return fmt.Errorf("module not found: %s", moduleID)
	}

	delete(m.modules, moduleID)
	return nil
}

// GetModule returns a module by ID
func (m *Manager) GetModule(id string) (types.Module, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	module, exists := m.modules[id]
	return module, exists
}

// ListModules returns all registered modules
func (m *Manager) ListModules() []types.Module {
	m.mu.RLock()
	defer m.mu.RUnlock()

	modules := make([]types.Module, 0, len(m.modules))
	for _, module := range m.modules {
		modules = append(modules, module)
	}
	return modules
}

// SetRepository sets the repository for all modules
func (m *Manager) SetRepository(repo types.Repository) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.repo = repo

	// Initialize all modules with repository
	for _, module := range m.modules {
		if err := module.Init(repo); err != nil {
			return fmt.Errorf("initializing module %s: %w", module.ID(), err)
		}
	}

	return nil
}

// HandleCommand handles a module command
func (m *Manager) HandleCommand(moduleID string, cmd string, args []string) error {
	module, exists := m.GetModule(moduleID)
	if !exists {
		return fmt.Errorf("module not found: %s", moduleID)
	}

	return module.HandleCommand(cmd, args)
}

// GetModuleCommands returns available commands for a module
func (m *Manager) GetModuleCommands(moduleID string) ([]types.Command, error) {
	module, exists := m.GetModule(moduleID)
	if !exists {
		return nil, fmt.Errorf("module not found: %s", moduleID)
	}

	return module.Commands(), nil
}
