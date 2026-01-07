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
	OpenAIAPIURL = "https://api.openai.com/v1/chat/completions"
	DefaultOpenAIModel = "gpt-4o"
)

// OpenAIClient handles communication with the OpenAI API
type OpenAIClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey, model string) *OpenAIClient {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if model == "" {
		model = DefaultOpenAIModel
	}
	return &OpenAIClient{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{},
	}
}

// openAIRequest is the request body for OpenAI API
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Tools       []openAITool    `json:"tools,omitempty"`
	Stream      bool            `json:"stream"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

type openAIMessage struct {
	Role       string          `json:"role"`
	Content    any             `json:"content"` // string or array
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  InputSchema `json:"parameters"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Stream sends a request to OpenAI and returns a channel of streaming events
func (c *OpenAIClient) Stream(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) <-chan StreamEvent {
	events := make(chan StreamEvent, 10)

	go func() {
		defer close(events)

		// Convert messages to OpenAI format
		oaiMessages := []openAIMessage{}

		// Add system message
		if systemPrompt != "" {
			oaiMessages = append(oaiMessages, openAIMessage{
				Role:    "system",
				Content: systemPrompt,
			})
		}

		// Convert messages
		for _, msg := range messages {
			oaiMsg := convertMessage(msg)
			oaiMessages = append(oaiMessages, oaiMsg...)
		}

		// Convert tools to OpenAI format
		oaiTools := make([]openAITool, len(tools))
		for i, tool := range tools {
			oaiTools[i] = openAITool{
				Type: "function",
				Function: openAIFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
			}
		}

		reqBody := openAIRequest{
			Model:     c.model,
			Messages:  oaiMessages,
			Tools:     oaiTools,
			Stream:    true,
			MaxTokens: 4096,
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			events <- StreamEvent{Error: fmt.Errorf("failed to marshal request: %w", err)}
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", OpenAIAPIURL, bytes.NewReader(jsonBody))
		if err != nil {
			events <- StreamEvent{Error: fmt.Errorf("failed to create request: %w", err)}
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

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

// convertMessage converts our Message format to OpenAI format
func convertMessage(msg Message) []openAIMessage {
	result := []openAIMessage{}

	if msg.Role == "user" {
		// Check if it's a tool result
		for _, part := range msg.Content {
			if part.Type == "tool_result" {
				result = append(result, openAIMessage{
					Role:       "tool",
					Content:    part.Content,
					ToolCallID: part.ToolUseID,
				})
			}
		}
		if len(result) > 0 {
			return result
		}

		// Regular user message
		var textContent string
		for _, part := range msg.Content {
			if part.Type == "text" {
				textContent += part.Text
			}
		}
		return []openAIMessage{{Role: "user", Content: textContent}}
	}

	if msg.Role == "assistant" {
		oaiMsg := openAIMessage{Role: "assistant"}
		var textContent string
		var toolCalls []openAIToolCall

		for _, part := range msg.Content {
			if part.Type == "text" {
				textContent += part.Text
			} else if part.Type == "tool_use" {
				inputJSON, _ := json.Marshal(part.Input)
				toolCalls = append(toolCalls, openAIToolCall{
					ID:   part.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      part.Name,
						Arguments: string(inputJSON),
					},
				})
			}
		}

		if textContent != "" {
			oaiMsg.Content = textContent
		}
		if len(toolCalls) > 0 {
			oaiMsg.ToolCalls = toolCalls
		}
		return []openAIMessage{oaiMsg}
	}

	return result
}

// parseSSEStream parses Server-Sent Events from the response body
func (c *OpenAIClient) parseSSEStream(body io.Reader, events chan<- StreamEvent) {
	scanner := bufio.NewScanner(body)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	// Track tool calls being built
	toolCalls := make(map[int]*struct {
		ID        string
		Name      string
		Arguments strings.Builder
	})

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := line[6:]

			if data == "[DONE]" {
				// Emit any completed tool calls
				for _, tc := range toolCalls {
					var input map[string]interface{}
					json.Unmarshal([]byte(tc.Arguments.String()), &input)
					events <- StreamEvent{
						Type:      "tool_use",
						ToolUseID: tc.ID,
						ToolName:  tc.Name,
						ToolInput: input,
					}
				}
				events <- StreamEvent{Type: "end"}
				return
			}

			var chunk openAIStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			for _, choice := range chunk.Choices {
				delta := choice.Delta

				// Handle text content
				if delta.Content != "" {
					events <- StreamEvent{
						Type: "text",
						Text: delta.Content,
					}
				}

				// Handle tool calls
				for _, tc := range delta.ToolCalls {
					if _, exists := toolCalls[tc.Index]; !exists {
						toolCalls[tc.Index] = &struct {
							ID        string
							Name      string
							Arguments strings.Builder
						}{
							ID:   tc.ID,
							Name: tc.Function.Name,
						}
					}
					if tc.Function.Arguments != "" {
						toolCalls[tc.Index].Arguments.WriteString(tc.Function.Arguments)
					}
				}

				// Handle finish reason
				if choice.FinishReason == "tool_calls" {
					// Emit tool calls
					for _, tc := range toolCalls {
						var input map[string]interface{}
						json.Unmarshal([]byte(tc.Arguments.String()), &input)
						events <- StreamEvent{
							Type:      "tool_use",
							ToolUseID: tc.ID,
							ToolName:  tc.Name,
							ToolInput: input,
						}
					}
					events <- StreamEvent{
						Type:       "stop",
						StopReason: "tool_use",
					}
					// Clear tool calls for next iteration
					toolCalls = make(map[int]*struct {
						ID        string
						Name      string
						Arguments strings.Builder
					})
				} else if choice.FinishReason == "stop" {
					events <- StreamEvent{
						Type:       "stop",
						StopReason: "stop",
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		events <- StreamEvent{Error: fmt.Errorf("stream read error: %w", err)}
	}
}

type openAIStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Choices []struct {
		Index        int    `json:"index"`
		Delta        openAIDelta `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

type openAIDelta struct {
	Role      string `json:"role,omitempty"`
	Content   string `json:"content,omitempty"`
	ToolCalls []struct {
		Index    int    `json:"index"`
		ID       string `json:"id,omitempty"`
		Type     string `json:"type,omitempty"`
		Function struct {
			Name      string `json:"name,omitempty"`
			Arguments string `json:"arguments,omitempty"`
		} `json:"function"`
	} `json:"tool_calls,omitempty"`
}
