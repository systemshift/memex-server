package agent

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/systemshift/memex/cmd/memex-cli/client"
)

// Agent orchestrates the conversation with LLM and tool execution
type Agent struct {
	llm      client.LLMClient
	executor *Executor
	tools    []client.Tool
	messages []client.Message
	output   io.Writer
}

// NewAgent creates a new agent
func NewAgent(llm client.LLMClient, memex *client.MemexClient, output io.Writer) *Agent {
	return &Agent{
		llm:      llm,
		executor: NewExecutor(memex),
		tools:    GetTools(),
		messages: []client.Message{},
		output:   output,
	}
}

// Run processes a user query and streams the response
func (a *Agent) Run(ctx context.Context, query string) error {
	// Add user message
	a.messages = append(a.messages, client.CreateUserMessage(query))

	// Run the agent loop
	return a.loop(ctx)
}

// loop runs the agent loop until completion
func (a *Agent) loop(ctx context.Context) error {
	for {
		// Stream from LLM
		events := a.llm.Stream(ctx, SystemPrompt(), a.messages, a.tools)

		var textContent strings.Builder
		var toolCalls []toolCall
		var currentToolCall *toolCall

		for event := range events {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if event.Error != nil {
				return event.Error
			}

			switch event.Type {
			case "text":
				// Stream text directly to output
				fmt.Fprint(a.output, event.Text)
				textContent.WriteString(event.Text)

			case "tool_use":
				// Collect tool call
				currentToolCall = &toolCall{
					ID:    event.ToolUseID,
					Name:  event.ToolName,
					Input: event.ToolInput,
				}
				toolCalls = append(toolCalls, *currentToolCall)

			case "stop":
				if event.StopReason == "tool_use" && len(toolCalls) > 0 {
					// Need to execute tools and continue
					break
				}
				// Normal end
				if textContent.Len() > 0 {
					fmt.Fprintln(a.output) // Ensure newline at end
				}
				return nil

			case "end":
				if len(toolCalls) == 0 {
					if textContent.Len() > 0 {
						fmt.Fprintln(a.output)
					}
					return nil
				}
			}
		}

		// If we have tool calls, execute them and continue
		if len(toolCalls) > 0 {
			// Add assistant message with tool uses
			assistantContent := []client.ContentPart{}
			if textContent.Len() > 0 {
				assistantContent = append(assistantContent, client.ContentPart{
					Type: "text",
					Text: textContent.String(),
				})
			}
			for _, tc := range toolCalls {
				assistantContent = append(assistantContent, client.ContentPart{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Input,
				})
			}
			a.messages = append(a.messages, client.Message{
				Role:    "assistant",
				Content: assistantContent,
			})

			// Execute tools and collect results
			results := a.executeTools(toolCalls)

			// Add tool results
			a.messages = append(a.messages, client.CreateToolResultMessage(results))

			// Clear for next iteration
			toolCalls = nil
			textContent.Reset()

			// Continue the loop
			continue
		}

		// No tool calls and stream ended
		return nil
	}
}

// toolCall represents a tool call from Claude
type toolCall struct {
	ID    string
	Name  string
	Input map[string]interface{}
}

// executeTools executes tool calls and returns results
func (a *Agent) executeTools(calls []toolCall) []client.ToolResult {
	results := make([]client.ToolResult, len(calls))

	for i, tc := range calls {
		// Show tool execution
		fmt.Fprintf(a.output, "\n[%s", tc.Name)
		if query, ok := tc.Input["query"].(string); ok {
			fmt.Fprintf(a.output, ": %s", query)
		} else if id, ok := tc.Input["id"].(string); ok {
			fmt.Fprintf(a.output, ": %s", id)
		} else if start, ok := tc.Input["start"].(string); ok {
			fmt.Fprintf(a.output, ": %s", start)
		}
		fmt.Fprint(a.output, "]\n")

		// Execute the tool
		result, err := a.executor.Execute(tc.Name, tc.Input)

		if err != nil {
			results[i] = client.ToolResult{
				ToolUseID: tc.ID,
				Content:   fmt.Sprintf("Error: %s", err.Error()),
				IsError:   true,
			}
		} else {
			results[i] = client.ToolResult{
				ToolUseID: tc.ID,
				Content:   result,
			}
		}
	}

	return results
}

// Reset clears the conversation history
func (a *Agent) Reset() {
	a.messages = []client.Message{}
}

// AddContext adds context to the conversation without running
func (a *Agent) AddContext(role, content string) {
	var msg client.Message
	if role == "user" {
		msg = client.CreateUserMessage(content)
	} else {
		msg = client.CreateAssistantMessage(content)
	}
	a.messages = append(a.messages, msg)
}
