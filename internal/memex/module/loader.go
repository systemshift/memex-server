package module

import (
	"fmt"
	"os/exec"
)

// LoadModule loads a module from a path
func LoadModule(modulePath string) error {
	// Add replace directive to use local module
	cmd := exec.Command("go", "mod", "edit", "-replace", fmt.Sprintf("github.com/systemshift/%s=./%s", modulePath, modulePath))
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("adding replace directive: %w", err)
	}

	// Run go mod tidy to download dependencies
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tidying modules: %w", err)
	}

	// The module's init() function will register it when imported
	return nil
}
