package test

import (
	"memex/pkg/types"
)

// mockModule implements types.Module for testing
type mockModule struct {
	id          string
	name        string
	description string
	commands    []types.Command
	initCalled  bool
	lastCommand string
	lastArgs    []string
}

func (m *mockModule) ID() string {
	return m.id
}

func (m *mockModule) Name() string {
	return m.name
}

func (m *mockModule) Description() string {
	return m.description
}

func (m *mockModule) Init(repo types.Repository) error {
	m.initCalled = true
	return nil
}

func (m *mockModule) Commands() []types.Command {
	return m.commands
}

func (m *mockModule) HandleCommand(cmd string, args []string) error {
	m.lastCommand = cmd
	m.lastArgs = args
	return nil
}

// mockShutdownModule extends mockModule with shutdown capability
type mockShutdownModule struct {
	mockModule
	shutdownCalled bool
}

func (m *mockShutdownModule) Shutdown() error {
	m.shutdownCalled = true
	return nil
}

// mockRepository implements types.ModuleRepository for testing
type mockRepository struct {
	modules   map[string]types.Module
	loader    types.ModuleLoader
	discovery types.ModuleDiscovery
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		modules:   make(map[string]types.Module),
		loader:    &mockModuleLoader{},
		discovery: &mockModuleDiscovery{},
	}
}

func (r *mockRepository) GetLoader() types.ModuleLoader {
	return r.loader
}

func (r *mockRepository) GetDiscovery() types.ModuleDiscovery {
	return r.discovery
}

// mockModuleLoader implements types.ModuleLoader for testing
type mockModuleLoader struct{}

func (l *mockModuleLoader) AddPath(path string)                          {}
func (l *mockModuleLoader) AddDevPath(moduleID, path string)             {}
func (l *mockModuleLoader) LoadModule(id string, mod types.Module) error { return nil }
func (l *mockModuleLoader) UnloadModule(id string) error                 { return nil }
func (l *mockModuleLoader) UnloadAll() error                             { return nil }
func (l *mockModuleLoader) IsDevModule(moduleID string) bool             { return false }
func (l *mockModuleLoader) GetDevPath(moduleID string) (string, bool)    { return "", false }

// mockModuleDiscovery implements types.ModuleDiscovery for testing
type mockModuleDiscovery struct{}

func (d *mockModuleDiscovery) DiscoverModules() error                { return nil }
func (d *mockModuleDiscovery) ValidateModule(mod types.Module) error { return nil }

func (r *mockRepository) AddNode(content []byte, nodeType string, meta map[string]interface{}) (string, error) {
	return "test-id", nil
}

func (r *mockRepository) AddNodeWithID(id string, content []byte, nodeType string, meta map[string]interface{}) error {
	return nil
}

func (r *mockRepository) GetNode(id string) (*types.Node, error) {
	return nil, nil
}

func (r *mockRepository) DeleteNode(id string) error {
	return nil
}

func (r *mockRepository) ListNodes() ([]string, error) {
	return nil, nil
}

func (r *mockRepository) GetContent(id string) ([]byte, error) {
	return nil, nil
}

func (r *mockRepository) AddLink(source, target, linkType string, meta map[string]interface{}) error {
	return nil
}

func (r *mockRepository) GetLinks(nodeID string) ([]*types.Link, error) {
	return nil, nil
}

func (r *mockRepository) DeleteLink(source, target, linkType string) error {
	return nil
}

func (r *mockRepository) ListModules() []types.Module {
	modules := make([]types.Module, 0, len(r.modules))
	for _, mod := range r.modules {
		modules = append(modules, mod)
	}
	return modules
}

func (r *mockRepository) GetModule(id string) (types.Module, bool) {
	mod, exists := r.modules[id]
	return mod, exists
}

func (r *mockRepository) RegisterModule(module types.Module) error {
	r.modules[module.ID()] = module
	return nil
}

func (r *mockRepository) QueryNodesByModule(moduleID string) ([]*types.Node, error) {
	return nil, nil
}

func (r *mockRepository) QueryLinksByModule(moduleID string) ([]*types.Link, error) {
	return nil, nil
}

func (r *mockRepository) Close() error {
	return nil
}
