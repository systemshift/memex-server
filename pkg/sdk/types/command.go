package types

import (
	"context"
	"io"
)

// Command represents a module command
type Command struct {
	Name    string          // Command name
	Args    []string        // Command arguments
	Input   io.Reader       // Standard input
	Output  io.Writer       // Standard output
	Error   io.Writer       // Standard error
	Context context.Context // Command context
}

// Response represents a command response
type Response struct {
	Status int         // Response status code
	Data   interface{} // Response data
	Error  string      // Error message if any
	Meta   Meta        // Response metadata
}

// Meta represents command metadata
type Meta map[string]interface{}

// Status codes
const (
	StatusSuccess      = 0
	StatusError        = 1
	StatusInvalidArgs  = 2
	StatusUnsupported  = 3
	StatusUnauthorized = 4
)

// Standard command names
const (
	CmdID          = "id"          // Get module ID
	CmdName        = "name"        // Get module name
	CmdDescription = "description" // Get module description
	CmdRun         = "run"         // Run module command
	CmdHelp        = "help"        // Get command help
)

// CommandHandler processes module commands
type CommandHandler interface {
	// Handle processes a command and returns a response
	Handle(cmd Command) Response
}
