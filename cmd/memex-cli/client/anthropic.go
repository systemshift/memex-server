package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	AnthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	AnthropicAPIVersion = "2023-06-01"
	DefaultModel        = "claude-sonnet-4-20250514"
)

// AnthropicClient handles communication with the Claude API
type AnthropicClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewAnthropicClient creates a new Anthropic client
func NewAnthropicClient(apiKey, model string) *AnthropicClient {
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if model == "" {
		model = DefaultModel
	}
	return &AnthropicClient{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{},
	}
}

// messagesRequest is the request body for Claude API
type messagesRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
	Tools     []Tool    `json:"tools,omitempty"`
	Stream    bool      `json:"stream"`
}

// Stream sends a request to Claude and returns a channel of streaming events
func (c *AnthropicClient) Stream(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) <-chan StreamEvent {
	events := make(chan StreamEvent, 10)

	go func() {
		defer close(events)

		reqBody := messagesRequest{
			Model:     c.model,
			MaxTokens: 4096,
			System:    systemPrompt,
			Messages:  messages,
			Tools:     tools,
			Stream:    true,
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			events <- StreamEvent{Error: fmt.Errorf("failed to marshal request: %w", err)}
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", AnthropicAPIURL, bytes.NewReader(jsonBody))
		if err != nil {
			events <- StreamEvent{Error: fmt.Errorf("failed to create request: %w", err)}
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", AnthropicAPIVersion)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			events <- StreamEvent{Error: fmt.Errorf("request failed: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			events <- StreamEvent{Error: fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))}
			return
		}

		// Parse SSE stream
		c.parseSSEStream(resp.Body, events)
	}()

	return events
}

// parseSSEStream parses Server-Sent Events from the response body
func (c *AnthropicClient) parseSSEStream(body io.Reader, events chan<- StreamEvent) {
	scanner := bufio.NewScanner(body)

	// Increase buffer size for large responses
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var currentToolUse struct {
		ID    string
		Name  string
		Input strings.Builder
	}

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// Parse SSE data lines
		if strings.HasPrefix(line, "data: ") {
			data := line[6:]

			// Handle [DONE] signal
			if data == "[DONE]" {
				events <- StreamEvent{Type: "end"}
				return
			}

			var event sseEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue // Skip malformed events
			}

			switch event.Type {
			case "content_block_start":
				if event.ContentBlock.Type == "tool_use" {
					currentToolUse.ID = event.ContentBlock.ID
					currentToolUse.Name = event.ContentBlock.Name
					currentToolUse.Input.Reset()
				}

			case "content_block_delta":
				if event.Delta.Type == "text_delta" {
					events <- StreamEvent{
						Type: "text",
						Text: event.Delta.Text,
					}
				} else if event.Delta.Type == "input_json_delta" {
					currentToolUse.Input.WriteString(event.Delta.PartialJSON)
				}

			case "content_block_stop":
				if currentToolUse.ID != "" {
					var toolInput map[string]interface{}
					json.Unmarshal([]byte(currentToolUse.Input.String()), &toolInput)

					events <- StreamEvent{
						Type:      "tool_use",
						ToolUseID: currentToolUse.ID,
						ToolName:  currentToolUse.Name,
						ToolInput: toolInput,
					}

					currentToolUse.ID = ""
					currentToolUse.Name = ""
					currentToolUse.Input.Reset()
				}

			case "message_delta":
				if event.Delta.StopReason != "" {
					events <- StreamEvent{
						Type:       "stop",
						StopReason: event.Delta.StopReason,
					}
				}

			case "message_stop":
				events <- StreamEvent{Type: "end"}
				return

			case "error":
				events <- StreamEvent{
					Error: fmt.Errorf("API error: %s", event.Error.Message),
				}
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		events <- StreamEvent{Error: fmt.Errorf("stream read error: %w", err)}
	}
}

// sseEvent represents a parsed SSE event from Claude
type sseEvent struct {
	Type         string       `json:"type"`
	ContentBlock contentBlock `json:"content_block,omitempty"`
	Delta        delta        `json:"delta,omitempty"`
	Error        apiError     `json:"error,omitempty"`
	Index        int          `json:"index,omitempty"`
}

type contentBlock struct {
	Type  string `json:"type"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Text  string `json:"text,omitempty"`
	Input any    `json:"input,omitempty"`
}

type delta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// CreateUserMessage creates a user message with text content
func CreateUserMessage(text string) Message {
	return Message{
		Role: "user",
		Content: []ContentPart{
			{Type: "text", Text: text},
		},
	}
}

// CreateAssistantMessage creates an assistant message with text content
func CreateAssistantMessage(text string) Message {
	return Message{
		Role: "assistant",
		Content: []ContentPart{
			{Type: "text", Text: text},
		},
	}
}

// CreateToolResultMessage creates a message with tool results
func CreateToolResultMessage(results []ToolResult) Message {
	parts := make([]ContentPart, len(results))
	for i, r := range results {
		parts[i] = ContentPart{
			Type:      "tool_result",
			ToolUseID: r.ToolUseID,
			Content:   r.Content,
		}
	}
	return Message{
		Role:    "user",
		Content: parts,
	}
}

// CreateToolUseMessage creates an assistant message with tool use
func CreateToolUseMessage(toolUseID, toolName string, input map[string]interface{}) Message {
	return Message{
		Role: "assistant",
		Content: []ContentPart{
			{
				Type:  "tool_use",
				ID:    toolUseID,
				Name:  toolName,
				Input: input,
			},
		},
	}
}
