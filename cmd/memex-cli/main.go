package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

func main() {
	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n" + dimStyle.Render("Interrupted"))
		cancel()
		os.Exit(0)
	}()

	// Check for command line argument (one-shot mode)
	if len(os.Args) > 1 {
		query := strings.Join(os.Args[1:], " ")
		runQuery(ctx, query)
		return
	}

	// Check for piped input
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Reading from pipe
		scanner := bufio.NewScanner(os.Stdin)
		var input strings.Builder
		for scanner.Scan() {
			input.WriteString(scanner.Text())
			input.WriteString("\n")
		}
		if input.Len() > 0 {
			runQuery(ctx, strings.TrimSpace(input.String()))
		}
		return
	}

	// Interactive REPL mode
	runREPL(ctx)
}

// findBridgePath locates the Python bridge script
func findBridgePath() string {
	// Try relative to executable
	execPath, err := os.Executable()
	if err == nil {
		bridgePath := filepath.Join(filepath.Dir(execPath), "..", "..", "bridge", "query.py")
		if _, err := os.Stat(bridgePath); err == nil {
			return bridgePath
		}
	}

	// Try relative to working directory
	cwd, err := os.Getwd()
	if err == nil {
		bridgePath := filepath.Join(cwd, "bridge", "query.py")
		if _, err := os.Stat(bridgePath); err == nil {
			return bridgePath
		}
		// Try parent directories
		for i := 0; i < 3; i++ {
			cwd = filepath.Dir(cwd)
			bridgePath = filepath.Join(cwd, "bridge", "query.py")
			if _, err := os.Stat(bridgePath); err == nil {
				return bridgePath
			}
		}
	}

	// Default: assume bridge is in memex repo
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "memex", "bridge", "query.py")
}

// findPython locates the Python interpreter (prefer venv)
func findPython(bridgePath string) string {
	bridgeDir := filepath.Dir(bridgePath)

	// Check for venv in bridge directory
	venvPython := filepath.Join(bridgeDir, "venv", "bin", "python")
	if _, err := os.Stat(venvPython); err == nil {
		return venvPython
	}

	// Fallback to system python
	return "python3"
}

func runQuery(ctx context.Context, query string) {
	bridgePath := findBridgePath()
	pythonPath := findPython(bridgePath)

	cmd := exec.CommandContext(ctx, pythonPath, bridgePath, query)
	cmd.Env = os.Environ()

	// Stream stdout
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render(fmt.Sprintf("Error: %v", err)))
		os.Exit(1)
	}

	// Stream stderr
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render(fmt.Sprintf("Error: %v", err)))
		os.Exit(1)
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render(fmt.Sprintf("Error starting bridge: %v", err)))
		os.Exit(1)
	}

	// Stream output in real-time
	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return // Context cancelled, exit quietly
		}
		// Don't print error if it's just a non-zero exit
		if _, ok := err.(*exec.ExitError); !ok {
			fmt.Fprintln(os.Stderr, errorStyle.Render(fmt.Sprintf("Error: %v", err)))
		}
	}
}

func runREPL(ctx context.Context) {
	// Print banner
	fmt.Println()
	fmt.Println(titleStyle.Render("MEMEX"))
	fmt.Println(dimStyle.Render("Ask anything about your knowledge graph"))
	fmt.Println(dimStyle.Render("Type 'exit' or Ctrl+C to quit"))
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print(promptStyle.Render("> "))

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			continue
		}

		// Handle commands
		switch strings.ToLower(input) {
		case "exit", "quit", "q":
			fmt.Println(dimStyle.Render("Goodbye!"))
			return
		case "help":
			printHelp()
			continue
		}

		// Run query via Python bridge
		fmt.Println()
		runQuery(ctx, input)
		fmt.Println()
	}
}

func printHelp() {
	fmt.Println()
	fmt.Println(titleStyle.Render("Commands:"))
	fmt.Println("  exit, quit, q  - Exit the program")
	fmt.Println("  help           - Show this help")
	fmt.Println()
	fmt.Println(titleStyle.Render("Example queries:"))
	fmt.Println("  who knows about kubernetes")
	fmt.Println("  list all people")
	fmt.Println("  what is person:000 connected to")
	fmt.Println("  search for documents about the Acme project")
	fmt.Println()
}
