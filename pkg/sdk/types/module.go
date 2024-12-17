package types

// ModuleCapability represents a specific capability a module provides
type ModuleCapability string

// ModuleCommand represents a command provided by a module
type ModuleCommand struct {
	Name        string   // Command name (e.g., "add", "remove")
	Description string   // Command description
	Usage       string   // Command usage (e.g., "ast add <file>")
	Args        []string // Expected arguments
}

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
