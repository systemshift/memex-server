package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/systemshift/memex/cmd/memex-cli/agent"
	"github.com/systemshift/memex/cmd/memex-cli/client"
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
	// Get configuration from environment
	memexURL := os.Getenv("MEMEX_URL")
	if memexURL == "" {
		memexURL = "http://localhost:8080"
	}

	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		fmt.Fprintln(os.Stderr, errorStyle.Render("Error: OPENAI_API_KEY environment variable is required"))
		os.Exit(1)
	}

	// Create clients
	memexClient := client.NewMemexClient(memexURL)
	llmClient := client.NewOpenAIClient(openaiKey, "")

	// Check Memex connection
	if err := memexClient.Health(); err != nil {
		fmt.Fprintln(os.Stderr, errorStyle.Render(fmt.Sprintf("Warning: Cannot connect to Memex at %s: %v", memexURL, err)))
		fmt.Fprintln(os.Stderr, dimStyle.Render("Some features may not work."))
	}

	// Create agent
	ag := agent.NewAgent(llmClient, memexClient, os.Stdout)

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
		runQuery(ctx, ag, query)
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
			runQuery(ctx, ag, strings.TrimSpace(input.String()))
		}
		return
	}

	// Interactive REPL mode
	runREPL(ctx, ag)
}

func runQuery(ctx context.Context, ag *agent.Agent, query string) {
	if err := ag.Run(ctx, query); err != nil {
		if ctx.Err() != nil {
			return // Context cancelled, exit quietly
		}
		fmt.Fprintln(os.Stderr, errorStyle.Render(fmt.Sprintf("Error: %v", err)))
		os.Exit(1)
	}
}

func runREPL(ctx context.Context, ag *agent.Agent) {
	// Print banner
	fmt.Println()
	fmt.Println(titleStyle.Render("MEMEX"))
	fmt.Println(dimStyle.Render("Ask anything about your knowledge graph"))
	fmt.Println(dimStyle.Render("Type 'exit' or Ctrl+C to quit, 'clear' to reset context"))
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
		case "clear", "reset":
			ag.Reset()
			fmt.Println(dimStyle.Render("Context cleared."))
			continue
		case "help":
			printHelp()
			continue
		}

		// Run query
		fmt.Println()
		if err := ag.Run(ctx, input); err != nil {
			if ctx.Err() != nil {
				return
			}
			fmt.Fprintln(os.Stderr, errorStyle.Render(fmt.Sprintf("Error: %v", err)))
		}
		fmt.Println()
	}
}

func printHelp() {
	fmt.Println()
	fmt.Println(titleStyle.Render("Commands:"))
	fmt.Println("  exit, quit, q  - Exit the program")
	fmt.Println("  clear, reset   - Clear conversation context")
	fmt.Println("  help           - Show this help")
	fmt.Println()
	fmt.Println(titleStyle.Render("Example queries:"))
	fmt.Println("  who knows about kubernetes")
	fmt.Println("  what documents mention the Acme project")
	fmt.Println("  find all people in the company")
	fmt.Println("  tell me about [person name]")
	fmt.Println("  what's connected to [node id]")
	fmt.Println()
}
