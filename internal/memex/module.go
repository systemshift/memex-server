package memex

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"strings"

	"github.com/systemshift/memex/pkg/module"
)

// ModuleConfig represents the configuration for a module
type ModuleConfig struct {
	Path    string            `json:"path"`
	Version string            `json:"version"`
	Config  map[string]string `json:"config,omitempty"`
}

// ModuleManager handles module operations
type ModuleManager struct {
	configDir  string
	configPath string
	modules    map[string]ModuleConfig
	registry   *ModuleRegistry
}

// NewModuleManager creates a new module manager
func NewModuleManager(configDir string) (*ModuleManager, error) {
	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("creating config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "modules.json")
	manager := &ModuleManager{
		configDir:  configDir,
		configPath: configPath,
		modules:    make(map[string]ModuleConfig),
		registry:   NewModuleRegistry(),
	}

	// Load existing configuration if it exists
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("reading config file: %w", err)
		}

		if err := json.Unmarshal(data, &manager.modules); err != nil {
			return nil, fmt.Errorf("parsing config file: %w", err)
		}
	}

	return manager, nil
}

// saveConfig saves the module configuration
func (m *ModuleManager) saveConfig() error {
	data, err := json.MarshalIndent(m.modules, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// Install installs a module from a source
func (m *ModuleManager) Install(source string) error {
	// Determine if source is a git repository or local directory
	isGit := strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "git://")

	// Create modules directory
	modulesDir := filepath.Join(m.configDir, "modules")
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		return fmt.Errorf("creating modules directory: %w", err)
	}

	// Create temporary directory for building
	tempDir, err := os.MkdirTemp("", "memex-module-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Clone or copy source
	if isGit {
		cmd := exec.Command("git", "clone", source, tempDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cloning repository: %w", err)
		}
	} else {
		// Check if source exists
		if _, err := os.Stat(source); err != nil {
			return fmt.Errorf("source directory not found: %w", err)
		}

		// Copy source to temp directory
		cmd := exec.Command("cp", "-r", source+"/.", tempDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("copying source: %w", err)
		}
	}

	// Check for go.mod file
	goModPath := filepath.Join(tempDir, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return fmt.Errorf("go.mod file not found in module source")
	}

	// Build module
	cmd := exec.Command("go", "build", "-o", filepath.Join(tempDir, "module.so"), "-buildmode=plugin", ".")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building module: %w", err)
	}

	// Load module to get ID
	p, err := plugin.Open(filepath.Join(tempDir, "module.so"))
	if err != nil {
		return fmt.Errorf("loading module: %w", err)
	}

	newSymbol, err := p.Lookup("New")
	if err != nil {
		return fmt.Errorf("module does not export New function: %w", err)
	}

	newFunc, ok := newSymbol.(func() module.Module)
	if !ok {
		return fmt.Errorf("module New function has wrong signature")
	}

	mod := newFunc()
	moduleID := mod.ID()

	// Check if module already exists
	if _, exists := m.modules[moduleID]; exists {
		return fmt.Errorf("%w: %s", module.ErrModuleAlreadyExists, moduleID)
	}

	// Create module directory
	moduleDir := filepath.Join(modulesDir, moduleID)
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		return fmt.Errorf("creating module directory: %w", err)
	}

	// Copy module files
	cmd = exec.Command("cp", "-r", tempDir+"/.", moduleDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("copying module files: %w", err)
	}

	// Update configuration
	m.modules[moduleID] = ModuleConfig{
		Path:    moduleDir,
		Version: mod.Version(),
	}

	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("saving configuration: %w", err)
	}

	fmt.Printf("Module %s installed successfully\n", moduleID)
	return nil
}

// Remove removes a module
func (m *ModuleManager) Remove(moduleID string) error {
	config, exists := m.modules[moduleID]
	if !exists {
		return fmt.Errorf("%w: %s", module.ErrModuleNotFound, moduleID)
	}

	// Unregister module if it's loaded
	if m.registry.HasModule(moduleID) {
		if err := m.registry.Unregister(moduleID); err != nil {
			return fmt.Errorf("unregistering module: %w", err)
		}
	}

	// Remove module directory
	if err := os.RemoveAll(config.Path); err != nil {
		return fmt.Errorf("removing module directory: %w", err)
	}

	// Update configuration
	delete(m.modules, moduleID)
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("saving configuration: %w", err)
	}

	fmt.Printf("Module %s removed successfully\n", moduleID)
	return nil
}

// List lists all installed modules
func (m *ModuleManager) List() error {
	if len(m.modules) == 0 {
		fmt.Println("No modules installed")
		return nil
	}

	fmt.Println("Installed modules:")
	for id, config := range m.modules {
		fmt.Printf("  %s (v%s)\n", id, config.Version)
	}

	return nil
}

// loadModule loads a module from the given path
func (m *ModuleManager) loadModule(moduleID, modulePath string) error {
	// Check if module is already loaded
	if m.registry.HasModule(moduleID) {
		return nil
	}

	// Load module
	p, err := plugin.Open(filepath.Join(modulePath, "module.so"))
	if err != nil {
		return fmt.Errorf("loading module: %w", err)
	}

	newSymbol, err := p.Lookup("New")
	if err != nil {
		return fmt.Errorf("module does not export New function: %w", err)
	}

	newFunc, ok := newSymbol.(func() module.Module)
	if !ok {
		return fmt.Errorf("module New function has wrong signature")
	}

	mod := newFunc()
	if mod.ID() != moduleID {
		return fmt.Errorf("module ID mismatch: expected %s, got %s", moduleID, mod.ID())
	}

	// Initialize module
	ctx := context.Background()
	if err := mod.Init(ctx, m.registry); err != nil {
		return fmt.Errorf("%w: %s", module.ErrModuleInitFailed, err)
	}

	// Register module
	if err := m.registry.Register(mod); err != nil {
		return fmt.Errorf("registering module: %w", err)
	}

	// Start module
	if err := mod.Start(ctx); err != nil {
		return fmt.Errorf("starting module: %w", err)
	}

	return nil
}

// LoadAllModules loads all modules
func (m *ModuleManager) LoadAllModules() error {
	for id, config := range m.modules {
		if err := m.loadModule(id, config.Path); err != nil {
			return fmt.Errorf("loading module %s: %w", id, err)
		}
	}
	return nil
}

// ExecuteCommand executes a module command
func (m *ModuleManager) ExecuteCommand(moduleID, cmd string, args []string) error {
	// Check if module exists
	config, exists := m.modules[moduleID]
	if !exists {
		return fmt.Errorf("%w: %s", module.ErrModuleNotFound, moduleID)
	}

	// Check if module is loaded
	if !m.registry.HasModule(moduleID) {
		if err := m.loadModule(moduleID, config.Path); err != nil {
			return fmt.Errorf("loading module: %w", err)
		}
	}

	// Execute command
	ctx := context.Background()
	result, err := m.registry.RouteCommand(ctx, moduleID, cmd, args)
	if err != nil {
		return fmt.Errorf("executing command: %w", err)
	}

	// Print result if it's a string
	if str, ok := result.(string); ok {
		fmt.Println(str)
	}

	return nil
}

// ModuleRegistry implements the module.Registry interface
type ModuleRegistry struct {
	modules map[string]module.Module
	hooks   map[string][]module.Hook
}

// NewModuleRegistry creates a new module registry
func NewModuleRegistry() *ModuleRegistry {
	return &ModuleRegistry{
		modules: make(map[string]module.Module),
		hooks:   make(map[string][]module.Hook),
	}
}

// Register registers a module
func (r *ModuleRegistry) Register(mod module.Module) error {
	if _, exists := r.modules[mod.ID()]; exists {
		return fmt.Errorf("%w: %s", module.ErrModuleAlreadyExists, mod.ID())
	}

	r.modules[mod.ID()] = mod

	// Register hooks
	for _, hook := range mod.Hooks() {
		r.hooks[hook.Name] = append(r.hooks[hook.Name], hook)
	}

	return nil
}

// Unregister unregisters a module
func (r *ModuleRegistry) Unregister(moduleID string) error {
	mod, exists := r.modules[moduleID]
	if !exists {
		return fmt.Errorf("%w: %s", module.ErrModuleNotFound, moduleID)
	}

	// Stop module
	ctx := context.Background()
	if err := mod.Stop(ctx); err != nil {
		return fmt.Errorf("stopping module: %w", err)
	}

	// Unregister hooks
	for _, moduleHook := range mod.Hooks() {
		hooks := r.hooks[moduleHook.Name]
		for i, h := range hooks {
			if h.Name == moduleHook.Name {
				r.hooks[moduleHook.Name] = append(hooks[:i], hooks[i+1:]...)
				break
			}
		}
	}

	delete(r.modules, moduleID)
	return nil
}

// GetModule returns a module by ID
func (r *ModuleRegistry) GetModule(id string) (module.Module, error) {
	mod, exists := r.modules[id]
	if !exists {
		return nil, fmt.Errorf("%w: %s", module.ErrModuleNotFound, id)
	}
	return mod, nil
}

// HasModule checks if a module exists
func (r *ModuleRegistry) HasModule(id string) bool {
	_, exists := r.modules[id]
	return exists
}

// ListModules returns a list of all modules
func (r *ModuleRegistry) ListModules() []module.ModuleInfo {
	modules := make([]module.ModuleInfo, 0, len(r.modules))
	for _, mod := range r.modules {
		modules = append(modules, module.ModuleInfo{
			ID:          mod.ID(),
			Name:        mod.Name(),
			Description: mod.Description(),
			Version:     mod.Version(),
			Commands:    mod.Commands(),
		})
	}
	return modules
}

// RouteCommand routes a command to the appropriate module
func (r *ModuleRegistry) RouteCommand(ctx context.Context, moduleID, cmd string, args []string) (interface{}, error) {
	mod, exists := r.modules[moduleID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", module.ErrModuleNotFound, moduleID)
	}
	return mod.HandleCommand(ctx, cmd, args)
}

// RegisterHook registers a hook
func (r *ModuleRegistry) RegisterHook(hook module.Hook) error {
	r.hooks[hook.Name] = append(r.hooks[hook.Name], hook)
	return nil
}

// TriggerHook triggers a hook
func (r *ModuleRegistry) TriggerHook(ctx context.Context, name string, data interface{}) ([]interface{}, error) {
	hooks, exists := r.hooks[name]
	if !exists {
		return nil, nil
	}

	results := make([]interface{}, 0, len(hooks))
	for _, mod := range r.modules {
		result, err := mod.HandleHook(ctx, name, data)
		if err != nil {
			return nil, fmt.Errorf("handling hook: %w", err)
		}
		if result != nil {
			results = append(results, result)
		}
	}

	return results, nil
}

// ModuleCommand handles module-related commands
func ModuleCommand(args ...string) error {
	if len(args) < 1 {
		return fmt.Errorf("module command required")
	}

	// Create module manager
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "memex")
	manager, err := NewModuleManager(configDir)
	if err != nil {
		return fmt.Errorf("creating module manager: %w", err)
	}

	cmd := args[0]
	switch cmd {
	case "install":
		if len(args) < 2 {
			return fmt.Errorf("source path or URL required")
		}
		return manager.Install(args[1])

	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("module ID required")
		}
		return manager.Remove(args[1])

	case "list":
		return manager.List()

	default:
		return fmt.Errorf("unknown module command: %s", cmd)
	}
}

// HandleModuleCommand handles a module-specific command
func HandleModuleCommand(moduleID string, args ...string) error {
	// Create module manager
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "memex")
	manager, err := NewModuleManager(configDir)
	if err != nil {
		return fmt.Errorf("creating module manager: %w", err)
	}

	// Load all modules
	if err := manager.LoadAllModules(); err != nil {
		return fmt.Errorf("loading modules: %w", err)
	}

	// Execute command
	cmd := ""
	cmdArgs := args
	if len(args) > 0 {
		cmd = args[0]
		cmdArgs = args[1:]
	}

	return manager.ExecuteCommand(moduleID, cmd, cmdArgs)
}
