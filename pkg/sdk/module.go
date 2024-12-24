package sdk

import (
	"fmt"

	"memex/pkg/sdk/types"
)

// BaseModule provides a basic implementation of types.Module
type BaseModule struct {
	id          string
	name        string
	description string
	repo        types.Repository
	handler     types.Handler
	commands    []types.Command
}

// NewBaseModule creates a new base module
func NewBaseModule(id, name, description string) *BaseModule {
	m := &BaseModule{
		id:          id,
		name:        name,
		description: description,
		commands:    make([]types.Command, 0),
	}
	m.handler = NewBaseHandler(m)
	return m
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
	return nil
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
	resp := m.handler.Handle(types.Command{
		Name: cmd,
		Args: args,
	})

	if resp.Status != types.StatusSuccess {
		if resp.Error != "" {
			return fmt.Errorf(resp.Error)
		}
		return fmt.Errorf("command failed: %s", cmd)
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
		return types.Response{
			Status: types.StatusSuccess,
			Data:   h.module.ID(),
		}
	case types.CmdName:
		return types.Response{
			Status: types.StatusSuccess,
			Data:   h.module.Name(),
		}
	case types.CmdDescription:
		return types.Response{
			Status: types.StatusSuccess,
			Data:   h.module.Description(),
		}
	case types.CmdHelp:
		return types.Response{
			Status: types.StatusSuccess,
			Data:   h.module.Commands(),
		}
	default:
		return types.Response{
			Status: types.StatusUnsupported,
			Error:  fmt.Sprintf("unsupported command: %s", cmd.Name),
		}
	}
}
