package sdk

import (
	"fmt"
	"os"
	"text/tabwriter"
)

// CLI handles command-line operations for modules
type CLI struct {
	manager *Manager
}

// NewCLI creates a new CLI handler
func NewCLI(manager *Manager) *CLI {
	return &CLI{
		manager: manager,
	}
}

// HandleCommand handles a CLI command
func (c *CLI) HandleCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("module command required")
	}

	moduleID := args[0]
	if moduleID == "list" {
		return c.listModules()
	}

	if len(args) < 2 {
		return c.showModuleHelp(moduleID)
	}

	cmd := args[1]
	cmdArgs := args[2:]

	return c.manager.HandleCommand(moduleID, cmd, cmdArgs)
}

// listModules lists all available modules
func (c *CLI) listModules() error {
	modules := c.manager.ListModules()
	if len(modules) == 0 {
		fmt.Println("No modules installed")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MODULE\tNAME\tDESCRIPTION")
	for _, mod := range modules {
		fmt.Fprintf(w, "%s\t%s\t%s\n", mod.ID(), mod.Name(), mod.Description())
	}
	w.Flush()
	return nil
}

// showModuleHelp shows help for a specific module
func (c *CLI) showModuleHelp(moduleID string) error {
	module, exists := c.manager.GetModule(moduleID)
	if !exists {
		return fmt.Errorf("module not found: %s", moduleID)
	}

	fmt.Printf("Module: %s (%s)\n", module.Name(), module.ID())
	fmt.Printf("Description: %s\n\n", module.Description())

	commands := module.Commands()
	if len(commands) == 0 {
		fmt.Println("No commands available")
		return nil
	}

	fmt.Println("Commands:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, cmd := range commands {
		fmt.Fprintf(w, "  %s\n", cmd.Name)
	}
	w.Flush()
	return nil
}

// formatError formats an error message for display
func formatError(err error) string {
	if err == nil {
		return ""
	}

	// Format error message
	msg := err.Error()
	if len(msg) > 0 {
		msg = msg[:1]
	}
	return msg
}
