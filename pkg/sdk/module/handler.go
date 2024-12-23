package module

import (
	"fmt"

	"memex/pkg/sdk/types"
)

// BaseHandler provides common command handling functionality
type BaseHandler struct {
	module types.Module
}

// NewBaseHandler creates a new base command handler
func NewBaseHandler(module types.Module) *BaseHandler {
	return &BaseHandler{
		module: module,
	}
}

// Handle processes a command and returns a response
func (h *BaseHandler) Handle(cmd types.Command) types.Response {
	switch cmd.Name {
	case types.CmdID:
		return h.handleID()
	case types.CmdName:
		return h.handleName()
	case types.CmdDescription:
		return h.handleDescription()
	case types.CmdHelp:
		return h.handleHelp()
	case types.CmdRun:
		return h.handleRun(cmd)
	default:
		return types.Response{
			Status: types.StatusUnsupported,
			Error:  fmt.Sprintf("unsupported command: %s", cmd.Name),
		}
	}
}

func (h *BaseHandler) handleID() types.Response {
	return types.Response{
		Status: types.StatusSuccess,
		Data:   h.module.ID(),
	}
}

func (h *BaseHandler) handleName() types.Response {
	return types.Response{
		Status: types.StatusSuccess,
		Data:   h.module.Name(),
	}
}

func (h *BaseHandler) handleDescription() types.Response {
	return types.Response{
		Status: types.StatusSuccess,
		Data:   h.module.Description(),
	}
}

func (h *BaseHandler) handleHelp() types.Response {
	help := fmt.Sprintf(`Module: %s (%s)
Description: %s

Commands:
  id          Get module ID
  name        Get module name
  description Get module description
  help        Show this help message
  run [args]  Run module command
`, h.module.Name(), h.module.ID(), h.module.Description())

	return types.Response{
		Status: types.StatusSuccess,
		Data:   help,
	}
}

// handleRun is a placeholder implementation that should be overridden by specific modules.
// The cmd parameter is unused in the base implementation but will be used by modules.
func (h *BaseHandler) handleRun(_ types.Command) types.Response {
	return types.Response{
		Status: types.StatusUnsupported,
		Error:  "run command not implemented",
	}
}
