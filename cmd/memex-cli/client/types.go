package client

import (
	"context"
	"time"
)

// LLMClient interface for LLM providers (OpenAI, Anthropic, etc.)
type LLMClient interface {
	Stream(ctx context.Context, systemPrompt string, messages []Message, tools []Tool) <-chan StreamEvent
}

// Node represents a node in the Memex graph
type Node struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Meta     map[string]interface{} `json:"properties,omitempty"`
	Content  string                 `json:"content,omitempty"`
	Created  time.Time              `json:"created,omitempty"`
	Modified time.Time              `json:"modified,omitempty"`
}

// Link represents a relationship between nodes
type Link struct {
	Source   string                 `json:"source"`
	Target   string                 `json:"target"`
	Type     string                 `json:"type"`
	Meta     map[string]interface{} `json:"properties,omitempty"`
	Created  time.Time              `json:"created,omitempty"`
	Modified time.Time              `json:"modified,omitempty"`
}

// SearchResult from Memex query
type SearchResult struct {
	Nodes []Node `json:"nodes"`
	Total int    `json:"total,omitempty"`
}

// Subgraph from traversal queries
type Subgraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Link `json:"edges"`
}

// Message for Claude API
type Message struct {
	Role    string        `json:"role"`
	Content []ContentPart `json:"content"`
}

// ContentPart for multipart messages
type ContentPart struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Input     interface{} `json:"input,omitempty"`
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   string      `json:"content,omitempty"`
}

// Tool definition for Claude
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"input_schema"`
}

// InputSchema for tool parameters
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property for schema
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// StreamEvent from Claude API
type StreamEvent struct {
	Type       string
	Text       string
	ToolUseID  string
	ToolName   string
	ToolInput  map[string]interface{}
	StopReason string
	Error      error
}

// ToolResult to send back to Claude
type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}
