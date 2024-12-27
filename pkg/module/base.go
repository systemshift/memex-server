package module

import "fmt"

// Base provides common module functionality
type Base struct {
	id          string
	name        string
	description string
	repo        Repository
	commands    []Command
}

// NewBase creates a new base module
func NewBase(id, name, description string) *Base {
	return &Base{
		id:          id,
		name:        name,
		description: description,
		commands:    make([]Command, 0),
	}
}

// ID returns the module identifier
func (b *Base) ID() string {
	return b.id
}

// Name returns the module name
func (b *Base) Name() string {
	return b.name
}

// Description returns the module description
func (b *Base) Description() string {
	return b.description
}

// Init initializes the module with a repository
func (b *Base) Init(repo Repository) error {
	b.repo = repo
	return nil
}

// Commands returns the list of available commands
func (b *Base) Commands() []Command {
	baseCommands := []Command{
		{
			Name:        "help",
			Description: "Show module help",
		},
		{
			Name:        "version",
			Description: "Show module version",
		},
	}
	return append(baseCommands, b.commands...)
}

// HandleCommand handles a module command
func (b *Base) HandleCommand(cmd string, args []string) error {
	switch cmd {
	case "help":
		fmt.Printf("Module: %s\n", b.name)
		fmt.Printf("Description: %s\n\n", b.description)
		fmt.Println("Commands:")
		for _, cmd := range b.Commands() {
			if cmd.Usage != "" {
				fmt.Printf("  %s - %s\n    Usage: %s\n", cmd.Name, cmd.Description, cmd.Usage)
			} else {
				fmt.Printf("  %s - %s\n", cmd.Name, cmd.Description)
			}
		}
		return nil
	case "version":
		fmt.Printf("%s version 1.0.0\n", b.name)
		return nil
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

// AddCommand adds a command to the module
func (b *Base) AddCommand(cmd Command) {
	b.commands = append(b.commands, cmd)
}

// Helper methods for repository operations

// AddNode adds a node to the repository
func (b *Base) AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error) {
	if b.repo == nil {
		return "", fmt.Errorf("module not initialized")
	}
	return b.repo.AddNode(content, nodeType, meta)
}

// GetNode gets a node from the repository
func (b *Base) GetNode(id string) (*Node, error) {
	if b.repo == nil {
		return nil, fmt.Errorf("module not initialized")
	}
	return b.repo.GetNode(id)
}

// AddLink adds a link between nodes
func (b *Base) AddLink(source, target, linkType string, meta map[string]interface{}) error {
	if b.repo == nil {
		return fmt.Errorf("module not initialized")
	}
	return b.repo.AddLink(source, target, linkType, meta)
}

// GetLinks gets links for a node
func (b *Base) GetLinks(nodeID string) ([]*Link, error) {
	if b.repo == nil {
		return nil, fmt.Errorf("module not initialized")
	}
	return b.repo.GetLinks(nodeID)
}
