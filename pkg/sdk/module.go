package sdk

import (
	"fmt"

	"memex/pkg/sdk/types"
)

// ModuleOption is a function that configures a BaseModule
type ModuleOption func(*BaseModule)

// Helper functions for repository operations
func (m *BaseModule) AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error) {
	if m.repo == nil {
		return "", ErrNotInitalized
	}
	if content == nil || nodeType == "" {
		return "", fmt.Errorf("%w: content and type required", ErrInvalidInput)
	}
	return m.repo.AddNode(content, nodeType, meta)
}

func (m *BaseModule) GetNode(id string) (*types.Node, error) {
	if m.repo == nil {
		return nil, ErrNotInitalized
	}
	if id == "" {
		return nil, fmt.Errorf("%w: node ID required", ErrInvalidInput)
	}
	node, err := m.repo.GetNode(id)
	if err != nil {
		return nil, fmt.Errorf("%w: node %s", ErrNotFound, id)
	}
	return node, nil
}

func (m *BaseModule) AddLink(source, target, linkType string, meta map[string]interface{}) error {
	if m.repo == nil {
		return ErrNotInitalized
	}
	if source == "" || target == "" || linkType == "" {
		return fmt.Errorf("%w: source, target, and type required", ErrInvalidInput)
	}
	return m.repo.AddLink(source, target, linkType, meta)
}

// Helper functions for command handling
func (m *BaseModule) ValidateArgs(cmd types.Command, args []string) error {
	if len(args) < len(cmd.Args) {
		return fmt.Errorf("missing required arguments: %v", cmd.Args[len(args):])
	}
	return nil
}

func (m *BaseModule) FindCommand(name string) (types.Command, bool) {
	for _, cmd := range m.Commands() {
		if cmd.Name == name {
			return cmd, true
		}
	}
	return types.Command{}, false
}

// BaseModule provides a basic implementation of types.Module
type BaseModule struct {
	id          string
	name        string
	description string
	repo        types.Repository
	handler     types.Handler
	commands    []types.Command

	// Lifecycle hooks
	onInit     func(types.Repository) error
	onCommand  func(string, []string) error
	onShutdown func() error
}

// WithInitHook adds an initialization hook
func WithInitHook(hook func(types.Repository) error) ModuleOption {
	return func(m *BaseModule) {
		m.onInit = hook
	}
}

// WithCommandHook adds a command hook
func WithCommandHook(hook func(string, []string) error) ModuleOption {
	return func(m *BaseModule) {
		m.onCommand = hook
	}
}

// WithShutdownHook adds a shutdown hook
func WithShutdownHook(hook func() error) ModuleOption {
	return func(m *BaseModule) {
		m.onShutdown = hook
	}
}

// NewBaseModule creates a new base module
func NewBaseModule(id, name, description string, opts ...ModuleOption) *BaseModule {
	m := &BaseModule{
		id:          id,
		name:        name,
		description: description,
		commands:    make([]types.Command, 0),
		// Default no-op hooks
		onInit:     func(types.Repository) error { return nil },
		onCommand:  func(string, []string) error { return nil },
		onShutdown: func() error { return nil },
	}

	// Apply options
	for _, opt := range opts {
		opt(m)
	}

	m.handler = NewBaseHandler(m)
	return m
}

// Shutdown cleans up module resources
func (m *BaseModule) Shutdown() error {
	return m.onShutdown()
}

// AddCommand adds a command to the module
func (m *BaseModule) AddCommand(cmd types.Command) {
	m.commands = append(m.commands, cmd)
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
func (m *BaseModule) Init(repo types.Repository) error {
	m.repo = repo
	return m.onInit(repo)
}

// Commands returns the list of available commands
func (m *BaseModule) Commands() []types.Command {
	baseCommands := []types.Command{
		{
			Name:        types.CmdID,
			Description: "Get module ID",
		},
		{
			Name:        types.CmdName,
			Description: "Get module name",
		},
		{
			Name:        types.CmdDescription,
			Description: "Get module description",
		},
		{
			Name:        types.CmdHelp,
			Description: "Get command help",
		},
	}
	return append(baseCommands, m.commands...)
}

// HandleCommand handles a module command
func (m *BaseModule) HandleCommand(cmd string, args []string) error {
	// Find command
	command, exists := m.FindCommand(cmd)
	if !exists {
		return fmt.Errorf("%w: command %s", ErrNotFound, cmd)
	}

	// Validate arguments
	if err := m.ValidateArgs(command, args); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	// Check repository initialization for commands that need it
	if m.repo == nil && cmd != types.CmdID && cmd != types.CmdName &&
		cmd != types.CmdDescription && cmd != types.CmdHelp {
		return ErrNotInitalized
	}

	// Run command hook first
	if err := m.onCommand(cmd, args); err != nil {
		return fmt.Errorf("command hook: %w", err)
	}

	// Handle command through handler
	resp := m.handler.Handle(types.Command{
		Name:        cmd,
		Args:        args,
		Description: command.Description,
		Usage:       command.Usage,
	})

	if resp.Status != types.StatusSuccess {
		if resp.Error != "" {
			return fmt.Errorf(resp.Error)
		}
		return ErrNotSupported
	}

	return nil
}

// BaseHandler provides a basic command handler implementation
type BaseHandler struct {
	module *BaseModule
}

// NewBaseHandler creates a new base handler
func NewBaseHandler(module *BaseModule) *BaseHandler {
	return &BaseHandler{module: module}
}

// Handle handles a command
func (h *BaseHandler) Handle(cmd types.Command) types.Response {
	switch cmd.Name {
	case types.CmdID:
		return SuccessResponse(h.module.ID())
	case types.CmdName:
		return SuccessResponse(h.module.Name())
	case types.CmdDescription:
		return SuccessResponse(h.module.Description())
	case types.CmdHelp:
		return SuccessResponse(h.module.Commands())
	default:
		return NotSupportedResponse(fmt.Sprintf("command: %s", cmd.Name))
	}
}
