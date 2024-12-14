package test

import (
	"fmt"

	"memex/internal/memex/core"
)

// TestModule implements core.Module for testing
type TestModule struct {
	*core.BaseModule
	moduleID        string
	lastCommand     string
	lastArgs        []string
	shouldFail      bool
	enabledCommands []string
}

func NewTestModule(repo core.Repository) *TestModule {
	m := &TestModule{
		moduleID:        fmt.Sprintf("test-%d", len(repo.ListModules())),
		enabledCommands: []string{"add", "remove", "list"},
	}
	m.BaseModule = core.NewBaseModule(m.moduleID, "Test Module", "A test module", repo)
	return m
}

func (m *TestModule) ID() string {
	return m.moduleID
}

func (m *TestModule) SetID(id string) {
	m.moduleID = id
}

func (m *TestModule) Commands() []core.ModuleCommand {
	var cmds []core.ModuleCommand
	for _, name := range m.enabledCommands {
		cmds = append(cmds, core.ModuleCommand{
			Name:        name,
			Description: "Test command " + name,
			Usage:       m.moduleID + " " + name + " [args]",
		})
	}
	return cmds
}

func (m *TestModule) HandleCommand(cmd string, args []string) error {
	m.lastCommand = cmd
	m.lastArgs = args
	if m.shouldFail {
		return fmt.Errorf("command failed")
	}
	return nil
}

func (m *TestModule) GetLastCommand() string {
	return m.lastCommand
}

func (m *TestModule) GetLastArgs() []string {
	return m.lastArgs
}

func (m *TestModule) SetShouldFail(fail bool) {
	m.shouldFail = fail
}
