package module

import (
	"context"
	"fmt"
)

// Module is the interface all modules must implement
type Module interface {
	// Core identity
	ID() string
	Name() string
	Description() string
	Version() string

	// Lifecycle
	Init(ctx context.Context, registry Registry) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	// Command handling
	Commands() []Command
	HandleCommand(ctx context.Context, cmd string, args []string) (interface{}, error)

	// Extension points
	Hooks() []Hook
	HandleHook(ctx context.Context, hook string, data interface{}) (interface{}, error)
}

// Command represents a module command
type Command struct {
	Name        string
	Description string
	Usage       string
	Args        []string
	Flags       []Flag
	Handler     CommandHandler
}

// Flag represents a command flag
type Flag struct {
	Name        string
	Shorthand   string
	Description string
	Default     string
	Required    bool
}

// CommandHandler is a function that handles a command
type CommandHandler func(ctx context.Context, args []string) (interface{}, error)

// Hook represents an extension point
type Hook struct {
	Name        string
	Description string
	Priority    int
}

// Registry manages module registration and discovery
type Registry interface {
	// Registration
	Register(module Module) error
	Unregister(moduleID string) error

	// Discovery
	GetModule(id string) (Module, error)
	ListModules() []ModuleInfo

	// Command routing
	RouteCommand(ctx context.Context, moduleID, cmd string, args []string) (interface{}, error)

	// Hook system
	RegisterHook(hook Hook) error
	TriggerHook(ctx context.Context, name string, data interface{}) ([]interface{}, error)
}

// ModuleInfo contains basic information about a module
type ModuleInfo struct {
	ID          string
	Name        string
	Description string
	Version     string
	Commands    []Command
}

// Repository provides access to the memex repository
type Repository interface {
	// Node operations
	AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error)
	GetNode(id string) (Node, error)
	DeleteNode(id string) error
	ListNodes() ([]string, error)

	// Link operations
	AddLink(source, target, linkType string, meta map[string]interface{}) error
	GetLinks(nodeID string) ([]Link, error)
	DeleteLink(source, target, linkType string) error
}

// Node represents a node in the graph
type Node struct {
	ID       string
	Type     string
	Content  []byte
	Meta     map[string]interface{}
	Created  string
	Modified string
}

// Link represents a relationship between nodes
type Link struct {
	Source   string
	Target   string
	Type     string
	Meta     map[string]interface{}
	Created  string
	Modified string
}

// Error types
var (
	ErrModuleNotFound      = fmt.Errorf("module not found")
	ErrCommandNotFound     = fmt.Errorf("command not found")
	ErrModuleInitFailed    = fmt.Errorf("module initialization failed")
	ErrModuleAlreadyExists = fmt.Errorf("module already exists")
)
