package memex

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// Install installs a module using 'go install'
func (m *ModuleManager) Install(source string) error {
	// Check if source is a Go module URL
	if !strings.Contains(source, "/") {
		return fmt.Errorf("invalid module source: must be a Go module URL like 'github.com/user/memex-module@latest'")
	}

	// Install using go install
	cmd := exec.Command("go", "install", source)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("installing module: %w\nOutput: %s", err, string(output))
	}

	// Extract module name from source (e.g., github.com/user/memex-ast -> memex-ast)
	parts := strings.Split(source, "/")
	moduleName := parts[len(parts)-1]
	// Remove version suffix if present (e.g., memex-ast@latest -> memex-ast)
	if idx := strings.Index(moduleName, "@"); idx != -1 {
		moduleName = moduleName[:idx]
	}

	// Check if binary exists in PATH
	if _, err := exec.LookPath(moduleName); err != nil {
		return fmt.Errorf("module binary '%s' not found in PATH after installation", moduleName)
	}

	// Get module info by running the binary with --info flag
	cmd = exec.Command(moduleName, "--info")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("getting module info: %w", err)
	}

	// Parse module info (expecting JSON output)
	var moduleInfo struct {
		ID      string `json:"id"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(output, &moduleInfo); err != nil {
		// Fallback: use module name as ID
		moduleInfo.ID = moduleName
		moduleInfo.Version = "unknown"
	}

	// Check if module already exists
	if _, exists := m.modules[moduleInfo.ID]; exists {
		return fmt.Errorf("%w: %s", module.ErrModuleAlreadyExists, moduleInfo.ID)
	}

	// Update configuration
	m.modules[moduleInfo.ID] = ModuleConfig{
		Path:    moduleName, // Store binary name instead of path
		Version: moduleInfo.Version,
	}

	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("saving configuration: %w", err)
	}

	fmt.Printf("Module %s installed successfully\n", moduleInfo.ID)
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

	// Note: We don't actually remove the binary from the system
	// Users can manually remove it using 'go clean -i <module>' if needed
	fmt.Printf("Module %s removed from memex registry\n", moduleID)
	fmt.Printf("Note: Binary '%s' is still installed. Use 'go clean -i <module>' to remove it completely\n", config.Path)

	// Update configuration
	delete(m.modules, moduleID)
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("saving configuration: %w", err)
	}

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

// isModuleAvailable checks if a module binary is available in PATH
func (m *ModuleManager) isModuleAvailable(moduleID string) bool {
	config, exists := m.modules[moduleID]
	if !exists {
		return false
	}

	// Check if binary exists in PATH
	_, err := exec.LookPath(config.Path)
	return err == nil
}

// LoadAllModules validates all registered modules are available
func (m *ModuleManager) LoadAllModules() error {
	for id := range m.modules {
		if !m.isModuleAvailable(id) {
			fmt.Printf("Warning: module %s binary not found in PATH\n", id)
		}
	}
	return nil
}

// ExecuteCommand executes a module command using subprocess
func (m *ModuleManager) ExecuteCommand(moduleID, cmd string, args []string) error {
	// Check if module exists
	config, exists := m.modules[moduleID]
	if !exists {
		return fmt.Errorf("%w: %s", module.ErrModuleNotFound, moduleID)
	}

	// Check if module binary is available
	if !m.isModuleAvailable(moduleID) {
		return fmt.Errorf("module binary '%s' not found in PATH", config.Path)
	}

	// Prepare command arguments
	cmdArgs := []string{cmd}
	cmdArgs = append(cmdArgs, args...)

	// Execute subprocess
	execCmd := exec.Command(config.Path, cmdArgs...)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("executing module command: %w", err)
	}

	return nil
}

// ModuleRegistry keeps track of installed modules for subprocess execution
type ModuleRegistry struct {
	modules map[string]ModuleConfig
}

// NewModuleRegistry creates a new module registry
func NewModuleRegistry() *ModuleRegistry {
	return &ModuleRegistry{
		modules: make(map[string]ModuleConfig),
	}
}

// Register registers a module configuration
func (r *ModuleRegistry) Register(moduleID string, config ModuleConfig) error {
	if _, exists := r.modules[moduleID]; exists {
		return fmt.Errorf("%w: %s", module.ErrModuleAlreadyExists, moduleID)
	}

	r.modules[moduleID] = config
	return nil
}

// Unregister unregisters a module
func (r *ModuleRegistry) Unregister(moduleID string) error {
	_, exists := r.modules[moduleID]
	if !exists {
		return fmt.Errorf("%w: %s", module.ErrModuleNotFound, moduleID)
	}

	delete(r.modules, moduleID)
	return nil
}

// GetModule returns module config by ID
func (r *ModuleRegistry) GetModule(id string) (ModuleConfig, error) {
	mod, exists := r.modules[id]
	if !exists {
		return ModuleConfig{}, fmt.Errorf("%w: %s", module.ErrModuleNotFound, id)
	}
	return mod, nil
}

// HasModule checks if a module exists
func (r *ModuleRegistry) HasModule(id string) bool {
	_, exists := r.modules[id]
	return exists
}

// ListModules returns a list of all registered modules
func (r *ModuleRegistry) ListModules() []ModuleConfig {
	modules := make([]ModuleConfig, 0, len(r.modules))
	for _, config := range r.modules {
		modules = append(modules, config)
	}
	return modules
}

// RouteCommand is deprecated in subprocess model - use ExecuteCommand directly
func (r *ModuleRegistry) RouteCommand(ctx context.Context, moduleID, cmd string, args []string) (interface{}, error) {
	return nil, fmt.Errorf("RouteCommand is deprecated - use subprocess execution")
}

// Hook system is simplified for subprocess model
// RegisterHook is deprecated
func (r *ModuleRegistry) RegisterHook(hook module.Hook) error {
	return fmt.Errorf("hook system not implemented for subprocess modules")
}

// TriggerHook is deprecated
func (r *ModuleRegistry) TriggerHook(ctx context.Context, name string, data interface{}) ([]interface{}, error) {
	return nil, fmt.Errorf("hook system not implemented for subprocess modules")
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
