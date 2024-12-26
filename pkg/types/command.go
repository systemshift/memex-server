package types

// Command constants
const (
	CmdID          = "id"
	CmdName        = "name"
	CmdDescription = "description"
	CmdHelp        = "help"
)

// Handler defines the interface for command handlers
type Handler interface {
	Handle(Command) Response
}
