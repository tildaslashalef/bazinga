package types

import (
	"context"
	"time"
)

// Provider interface for LLM providers
type Provider interface {
	GetProviderName() string
	GenerateResponse(ctx context.Context, req *GenerateRequest) (*Response, error)
	StreamResponse(ctx context.Context, req *GenerateRequest) (<-chan *StreamChunk, error)
	SupportsFunctionCalling() bool
	GetAvailableModels() []Model
	Close() error
}

// GenerateRequest represents a request to generate text
type GenerateRequest struct {
	Messages    []Message `json:"messages"`
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
}

// Response represents the response from an LLM
type Response struct {
	ID               string     `json:"id"`
	Model            string     `json:"model"`
	Content          string     `json:"content"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	StopReason       string     `json:"stop_reason"`
	InputTokens      int        `json:"input_tokens"`
	OutputTokens     int        `json:"output_tokens"`
	ProcessingTimeMs int64      `json:"processing_time_ms"`
	CreatedAt        time.Time  `json:"created_at"`
}

// StreamChunk represents a chunk of streamed response
type StreamChunk struct {
	ID             string          `json:"id"`
	Type           string          `json:"type"` // "content_block_start", "content_block_delta", "content_block_stop", "tool_completion"
	Index          int             `json:"index,omitempty"`
	Delta          *Delta          `json:"delta,omitempty"`
	Content        string          `json:"content,omitempty"`
	ToolCall       *ToolCall       `json:"tool_call,omitempty"`
	ToolCompletion *ToolCompletion `json:"tool_completion,omitempty"`
}

// Delta represents incremental content
type Delta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Message represents a conversation message
type Message struct {
	Role       string      `json:"role"`    // "user", "assistant", "system", "tool"
	Content    interface{} `json:"content"` // string or []ContentBlock
	Name       string      `json:"name,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// Tool represents a tool available to the LLM
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ToolCall represents a tool call from the LLM
type ToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"` // "function"
	Function *Function              `json:"function,omitempty"`
	Name     string                 `json:"name"`
	Input    map[string]interface{} `json:"input"`
}

// Function represents a function call
type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCompletion represents the completion of a tool execution
type ToolCompletion struct {
	ToolName string                 `json:"tool_name"`
	Args     map[string]interface{} `json:"args"`
	Result   string                 `json:"result"`
	Error    string                 `json:"error,omitempty"`
	State    string                 `json:"state"` // "complete" or "error"
}

// Model represents an available model
type Model struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
}
