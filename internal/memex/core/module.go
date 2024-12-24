package core

import "fmt"

// Command represents a module command
type Command struct {
	Name        string   // Command name (e.g., "add", "status")
	Description string   // Command description
	Usage       string   // Usage example (e.g., "git add <file>")
	Args        []string // Expected arguments
}

// Module defines the interface that all memex modules must implement
type Module interface {
	// Identity
	ID() string          // Unique identifier (e.g., "git", "ast")
	Name() string        // Human-readable name
	Description() string // Module description

	// Core functionality
	Init(repo Repository) error                    // Initialize module with repository
	Commands() []Command                           // Available commands
	HandleCommand(cmd string, args []string) error // Execute a command
}

// BaseModule provides a basic implementation of the Module interface
type BaseModule struct {
	id          string
	name        string
	description string
	repo        Repository
	commands    []Command
}

// NewBaseModule creates a new base module
func NewBaseModule(id, name, description string) *BaseModule {
	return &BaseModule{
		id:          id,
		name:        name,
		description: description,
		commands:    make([]Command, 0),
	}
}

// ID returns the module identifier
func (m *BaseModule) ID() string {
	return m.id
}

// Name returns the module name
func (m *BaseModule) Name() string {
	return m.name
}

// Description returns the module description
func (m *BaseModule) Description() string {
	return m.description
}

// Init initializes the module with a repository
func (m *BaseModule) Init(repo Repository) error {
	m.repo = repo
	return nil
}

// Commands returns the list of available commands
func (m *BaseModule) Commands() []Command {
	return m.commands
}

// HandleCommand handles a module command
func (m *BaseModule) HandleCommand(cmd string, args []string) error {
	return fmt.Errorf("command not implemented: %s", cmd)
}

// AddCommand adds a command to the module
func (m *BaseModule) AddCommand(cmd Command) {
	m.commands = append(m.commands, cmd)
}

// ModuleRegistry manages module registration and lookup
type ModuleRegistry struct {
	modules map[string]Module
}

// NewModuleRegistry creates a new module registry
func NewModuleRegistry() *ModuleRegistry {
	return &ModuleRegistry{
		modules: make(map[string]Module),
	}
}

// RegisterModule registers a module
func (r *ModuleRegistry) RegisterModule(module Module) error {
	if _, exists := r.modules[module.ID()]; exists {
		return fmt.Errorf("module already registered: %s", module.ID())
	}
	r.modules[module.ID()] = module
	return nil
}

// GetModule returns a module by ID
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
