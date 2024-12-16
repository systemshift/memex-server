package memex

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"strings"

	"memex/internal/memex/core"
)

// Default paths
const (
	DefaultConfigDir  = ".config/memex"
	ModulesConfigFile = "modules.json"
	ModulesDir        = "modules"
)

// GitSystem defines the interface for Git operations
type GitSystem interface {
	Clone(url, targetDir string) error
}

// DefaultGitSystem implements GitSystem using real Git commands
type DefaultGitSystem struct{}

func (g *DefaultGitSystem) Clone(url, targetDir string) error {
	cmd := exec.Command("git", "clone", url, targetDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cloning repository: %w\nOutput: %s", err, output)
	}
	return nil
}

// ModuleManager handles module installation and configuration
type ModuleManager struct {
	config     *core.ModulesConfig
	configPath string
	modulesDir string
	repo       core.Repository
	git        GitSystem
}

// NewModuleManager creates a new module manager
func NewModuleManager() (*ModuleManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, DefaultConfigDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("creating config directory: %w", err)
	}

	modulesDir := filepath.Join(configDir, ModulesDir)
	if err := os.MkdirAll(modulesDir, 0755); err != nil {
		return nil, fmt.Errorf("creating modules directory: %w", err)
	}

	manager := &ModuleManager{
		configPath: filepath.Join(configDir, ModulesConfigFile),
		modulesDir: modulesDir,
		git:        &DefaultGitSystem{},
	}

	// Load or create config
	if err := manager.loadConfig(); err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	return manager, nil
}

// SetGitSystem sets the Git system implementation
func (m *ModuleManager) SetGitSystem(git GitSystem) {
	m.git = git
}

// SetRepository sets the repository for module operations
func (m *ModuleManager) SetRepository(repo core.Repository) {
	m.repo = repo
	// Load and register all installed modules
	if repo != nil {
		for moduleID, config := range m.config.Modules {
			if config.Type == "git" {
				pluginPath := filepath.Join(m.modulesDir, moduleID, "module.so")
				if _, err := os.Stat(pluginPath); err == nil {
					// Load the plugin
					plug, err := plugin.Open(pluginPath)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to load plugin %s: %v\n", moduleID, err)
						continue
					}

					// Look up the NewModule symbol
					newModuleSym, err := plug.Lookup("NewModule")
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to lookup NewModule in %s: %v\n", moduleID, err)
						continue
					}

					// Cast to the correct type
					newModule, ok := newModuleSym.(func(core.Repository) core.Module)
					if !ok {
						fmt.Fprintf(os.Stderr, "Warning: invalid module constructor type in %s\n", moduleID)
						continue
					}

					// Create and register the module
					module := newModule(repo)
					if err := repo.RegisterModule(module); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to register module %s: %v\n", moduleID, err)
						continue
					}
				}
			}
		}
	}
}

// GetModuleCommands returns available commands for a module
func (m *ModuleManager) GetModuleCommands(moduleID string) ([]core.ModuleCommand, error) {
	if m.repo == nil {
		return nil, fmt.Errorf("no repository connected")
	}

	module, exists := m.repo.GetModule(moduleID)
	if !exists {
		return nil, fmt.Errorf("module not found: %s", moduleID)
	}

	return module.Commands(), nil
}

// HandleCommand handles a module command
func (m *ModuleManager) HandleCommand(moduleID string, cmd string, args []string) error {
	if m.repo == nil {
		return fmt.Errorf("no repository connected")
	}

	module, exists := m.repo.GetModule(moduleID)
	if !exists {
		return fmt.Errorf("module not found: %s", moduleID)
	}

	return module.HandleCommand(cmd, args)
}

// loadConfig loads the modules configuration file
func (m *ModuleManager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config
			m.config = core.DefaultModulesConfig()
			return m.saveConfig()
		}
		return fmt.Errorf("reading config: %w", err)
	}

	var config core.ModulesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	m.config = &config
	return nil
}

// saveConfig saves the modules configuration file
func (m *ModuleManager) saveConfig() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// IsGitURL checks if a path is a Git URL
func IsGitURL(path string) bool {
	return strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "git@") ||
		strings.HasSuffix(path, ".git")
}

// GetModuleIDFromGit extracts module ID from Git URL
func GetModuleIDFromGit(url string) string {
	// Remove .git suffix if present
	url = strings.TrimSuffix(url, ".git")

	// Extract repo name from URL
	parts := strings.Split(url, "/")
	if len(parts) >= 1 {
		return parts[len(parts)-1]
	}
	return url
}

// cloneGitRepo clones a Git repository
func (m *ModuleManager) cloneGitRepo(url, moduleDir string) error {
	return m.git.Clone(url, moduleDir)
}

// InstallModule installs a module from a path or Git URL
func (m *ModuleManager) InstallModule(path string) error {
	var moduleID string
	var moduleType string
	var modulePath string

	if IsGitURL(path) {
		// Handle Git installation
		moduleID = GetModuleIDFromGit(path)
		moduleType = "git"

		// Create module directory
		moduleDir := filepath.Join(m.modulesDir, moduleID)
		if err := os.MkdirAll(moduleDir, 0755); err != nil {
			return fmt.Errorf("creating module directory: %w", err)
		}

		// Clone repository
		if err := m.cloneGitRepo(path, moduleDir); err != nil {
			return err
		}

		modulePath = moduleDir

		// Build module if it's a Go module
		if _, err := os.Stat(filepath.Join(moduleDir, "go.mod")); err == nil {
			// Build as a plugin
			pluginPath := filepath.Join(moduleDir, "module.so")
			cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", pluginPath, "./ast")
			cmd.Dir = moduleDir
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("building module: %w\nOutput: %s", err, output)
			}

			// Load and register module if repository is connected
			if m.repo != nil {
				// Load the plugin
				plug, err := plugin.Open(pluginPath)
				if err != nil {
					return fmt.Errorf("loading module plugin: %w", err)
				}

				// Look up the NewModule symbol
				newModuleSym, err := plug.Lookup("NewModule")
				if err != nil {
					return fmt.Errorf("looking up NewModule: %w", err)
				}

				// Cast to the correct type
				newModule, ok := newModuleSym.(func(core.Repository) core.Module)
				if !ok {
					return fmt.Errorf("invalid module constructor type")
				}

				// Create and register the module
				module := newModule(m.repo)
				if err := m.repo.RegisterModule(module); err != nil {
					return fmt.Errorf("registering module: %w", err)
				}
			}
		}
	} else {
		// Handle local installation
		// Validate module path
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("checking module path: %w", err)
		}

		if info.IsDir() {
			moduleType = "package"
		} else {
			moduleType = "binary"
		}

		// Use absolute path
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("getting absolute path: %w", err)
		}

		moduleID = filepath.Base(path)
		modulePath = absPath

		// Create module directory
		moduleDir := filepath.Join(m.modulesDir, moduleID)
		if err := os.MkdirAll(moduleDir, 0755); err != nil {
			return fmt.Errorf("creating module directory: %w", err)
		}
	}

	// Add module configuration
	m.config.AddModule(moduleID, core.ModuleConfig{
		Path:     modulePath,
		Type:     moduleType,
		Settings: make(map[string]interface{}),
	})

	// Save updated configuration
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	return nil
}

// RemoveModule removes a module
func (m *ModuleManager) RemoveModule(moduleID string) error {
	// Check if module exists in config
	if _, exists := m.config.GetModule(moduleID); !exists {
		return fmt.Errorf("module not found: %s", moduleID)
	}

	// Remove module directory
	moduleDir := filepath.Join(m.modulesDir, moduleID)
	if err := os.RemoveAll(moduleDir); err != nil {
		return fmt.Errorf("removing module directory: %w", err)
	}

	// Remove from configuration
	m.config.RemoveModule(moduleID)

	// Save updated configuration
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	return nil
}

// ListModules returns list of installed modules
func (m *ModuleManager) ListModules() []string {
	if m.repo == nil {
		return nil
	}

	modules := m.repo.ListModules()
	result := make([]string, len(modules))
	for i, module := range modules {
		result[i] = module.ID()
	}
	return result
}

// GetModuleConfig returns configuration for a module
func (m *ModuleManager) GetModuleConfig(moduleID string) (core.ModuleConfig, bool) {
	return m.config.GetModule(moduleID)
}
