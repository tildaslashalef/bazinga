package llm

import (
	"context"
	"time"
)

// Provider represents an LLM provider interface
type Provider interface {
	// Name returns the provider name (e.g., "bedrock", "openai")
	Name() string

	// GenerateResponse generates a response from the LLM
	GenerateResponse(ctx context.Context, req *GenerateRequest) (*Response, error)

	// StreamResponse streams a response from the LLM
	StreamResponse(ctx context.Context, req *GenerateRequest) (<-chan *StreamChunk, error)

	// SupportsFunctionCalling returns true if the provider supports function calling
	SupportsFunctionCalling() bool

	// GetAvailableModels returns the list of available models
	GetAvailableModels() []Model

	// GetDefaultModel returns the default model for this provider
	GetDefaultModel() string

	// EstimateTokens estimates token count for given text
	EstimateTokens(text string) int

	// GetTokenLimit returns the max token limit for the current model
	GetTokenLimit() int

	// Close cleans up provider resources
	Close() error
}

// GenerateRequest represents a request to generate content
type GenerateRequest struct {
	Messages    []Message              `json:"messages"`
	Model       string                 `json:"model,omitempty"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	Tools       []Tool                 `json:"tools,omitempty"`
	ToolChoice  interface{}            `json:"tool_choice,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Response represents a response from the LLM
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
	ToolInputDelta string          `json:"tool_input_delta,omitempty"`
	ToolCompletion *ToolCompletion `json:"tool_completion,omitempty"`
}

// Delta represents incremental content in a stream
type Delta struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Message represents a conversation message
type Message struct {
	Role       string      `json:"role"`    // "user", "assistant", "system", "tool"
	Content    interface{} `json:"content"` // string or []ContentBlock
	Name       string      `json:"name,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ContentBlock represents structured content
type ContentBlock struct {
	Type    string       `json:"type"` // "text", "image", "tool_use", "tool_result"
	Text    string       `json:"text,omitempty"`
	Source  *ImageSource `json:"source,omitempty"`
	ToolUse *ToolUse     `json:"tool_use,omitempty"`
	Content interface{}  `json:"content,omitempty"`
	IsError bool         `json:"is_error,omitempty"`
}

// ImageSource represents image data
type ImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "image/jpeg", "image/png"
	Data      string `json:"data"`       // base64 encoded
}

// Tool represents a function tool
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
	Name     string                 `json:"name,omitempty"`
	Input    map[string]interface{} `json:"input,omitempty"`
}

// ToolUse represents tool usage in content
type ToolUse struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// Function represents a function call
type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolCompletion represents the completion of a tool execution
type ToolCompletion struct {
	ToolName  string                 `json:"tool_name"`
	Args      map[string]interface{} `json:"args"`
	Result    string                 `json:"result"`
	Error     string                 `json:"error,omitempty"`
	State     string                 `json:"state"`                // "complete" or "error"
	TaskGroup string                 `json:"task_group,omitempty"` // Optional task group for UI organization
}

// Model represents an LLM model
type Model struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Provider        string  `json:"provider"`
	MaxTokens       int     `json:"max_tokens"`
	SupportsTools   bool    `json:"supports_tools"`
	CostPer1KTokens float64 `json:"cost_per_1k_tokens"`
}

// ProviderConfig represents provider configuration
type ProviderConfig struct {
	Type    string                 `yaml:"type"`    // "bedrock", "openai", etc.
	Name    string                 `yaml:"name"`    // Custom name for this provider instance
	Models  []string               `yaml:"models"`  // Available models
	Default string                 `yaml:"default"` // Default model
	Config  map[string]interface{} `yaml:"config"`  // Provider-specific config
	Enabled bool                   `yaml:"enabled"` // Whether this provider is enabled
}
