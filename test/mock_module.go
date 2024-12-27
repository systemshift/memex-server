package test

import (
	"github.com/systemshift/memex/internal/memex/core"
)

// MockModule implements core.Module for testing
type MockModule struct {
	id          string
	name        string
	description string
	commands    []core.Command
	initCalled  bool
	lastCommand string
	lastArgs    []string
}

func NewMockModule() *MockModule {
	return &MockModule{
		id:          "mock",
		name:        "Mock Module",
		description: "A mock module for testing",
		commands:    make([]core.Command, 0),
	}
}

func (m *MockModule) ID() string {
	return m.id
}

func (m *MockModule) Name() string {
	return m.name
}

func (m *MockModule) Description() string {
	return m.description
}

func (m *MockModule) Init(repo core.Repository) error {
	m.initCalled = true
	return nil
}

func (m *MockModule) Commands() []core.Command {
	return m.commands
}

func (m *MockModule) HandleCommand(cmd string, args []string) error {
	m.lastCommand = cmd
	m.lastArgs = args
	return nil
}

// MockRepository implements core.Repository for testing
type MockRepository struct {
	modules map[string]core.Module
	nodes   map[string]*core.Node
	links   map[string][]*core.Link
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		modules: make(map[string]core.Module),
		nodes:   make(map[string]*core.Node),
		links:   make(map[string][]*core.Link),
	}
}

func (r *MockRepository) AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error) {
	return "test-id", nil
}

func (r *MockRepository) AddNodeWithID(id string, content []byte, nodeType string, meta map[string]interface{}) error {
	return nil
}

func (r *MockRepository) GetNode(id string) (*core.Node, error) {
	return r.nodes[id], nil
}

func (r *MockRepository) DeleteNode(id string) error {
	return nil
}

func (r *MockRepository) ListNodes() ([]string, error) {
	ids := make([]string, 0, len(r.nodes))
	for id := range r.nodes {
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *MockRepository) GetContent(id string) ([]byte, error) {
	return nil, nil
}

func (r *MockRepository) AddLink(source, target, linkType string, meta map[string]interface{}) error {
	return nil
}

func (r *MockRepository) GetLinks(nodeID string) ([]*core.Link, error) {
	return r.links[nodeID], nil
}

func (r *MockRepository) DeleteLink(source, target, linkType string) error {
	return nil
}

func (r *MockRepository) ListModules() []core.Module {
	modules := make([]core.Module, 0, len(r.modules))
	for _, mod := range r.modules {
		modules = append(modules, mod)
	}
	return modules
}

func (r *MockRepository) GetModule(id string) (core.Module, bool) {
	mod, exists := r.modules[id]
	return mod, exists
}

func (r *MockRepository) RegisterModule(m core.Module) error {
	r.modules[m.ID()] = m
	return nil
}

func (r *MockRepository) QueryNodesByModule(moduleID string) ([]*core.Node, error) {
	return nil, nil
}

func (r *MockRepository) QueryLinksByModule(moduleID string) ([]*core.Link, error) {
	return nil, nil
}

func (r *MockRepository) Close() error {
	return nil
}
