package core

import "fmt"

// ModuleCapability represents a specific capability a module provides
type ModuleCapability string

// ModuleCommand represents a command provided by a module
type ModuleCommand struct {
	Name        string   // Command name (e.g., "add", "remove")
	Description string   // Command description
	Usage       string   // Command usage (e.g., "ast add <file>")
	Args        []string // Expected arguments
}

// ModuleCommandHandler handles execution of a module command
type ModuleCommandHandler func(repo Repository, args []string) error

// Module defines the interface that all memex modules must implement
type Module interface {
	// Identity
	ID() string
	Name() string
	Description() string

	// Commands
	Commands() []ModuleCommand                     // List of commands provided by this module
	HandleCommand(cmd string, args []string) error // Handle a command

	// Validation
	ValidateNodeType(nodeType string) bool
	ValidateLinkType(linkType string) bool
	ValidateMetadata(meta map[string]interface{}) error
}

// BaseModule provides a basic implementation of the Module interface
type BaseModule struct {
	id          string
	name        string
	description string
	repo        Repository
	commands    map[string]ModuleCommandHandler
}

// NewBaseModule creates a new base module
func NewBaseModule(id, name, description string, repo Repository) *BaseModule {
	return &BaseModule{
		id:          id,
		name:        name,
		description: description,
		repo:        repo,
		commands:    make(map[string]ModuleCommandHandler),
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

// Commands returns the list of available commands
func (m *BaseModule) Commands() []ModuleCommand {
	cmds := make([]ModuleCommand, 0, len(m.commands))
	for name := range m.commands {
		cmds = append(cmds, ModuleCommand{
			Name: name,
			// Description and usage would be set by implementing module
		})
	}
	return cmds
}

// HandleCommand handles a module command
func (m *BaseModule) HandleCommand(cmd string, args []string) error {
	handler, ok := m.commands[cmd]
	if !ok {
		return fmt.Errorf("unknown command: %s", cmd)
	}
	return handler(m.repo, args)
}

// RegisterCommand registers a command handler
func (m *BaseModule) RegisterCommand(name string, handler ModuleCommandHandler) {
	m.commands[name] = handler
}

// ValidateNodeType returns true by default
func (m *BaseModule) ValidateNodeType(nodeType string) bool {
	return true
}

// ValidateLinkType returns true by default
func (m *BaseModule) ValidateLinkType(linkType string) bool {
	return true
}

// ValidateMetadata returns nil by default
func (m *BaseModule) ValidateMetadata(meta map[string]interface{}) error {
	return nil
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

// ValidateNodeType checks if any module accepts this node type
func (r *ModuleRegistry) ValidateNodeType(nodeType string) bool {
	for _, module := range r.modules {
		if module.ValidateNodeType(nodeType) {
			return true
		}
	}
	return false
}

// ValidateLinkType checks if any module accepts this link type
func (r *ModuleRegistry) ValidateLinkType(linkType string) bool {
	for _, module := range r.modules {
		if module.ValidateLinkType(linkType) {
			return true
		}
	}
	return false
}
