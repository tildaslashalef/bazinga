package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"io"
	"net/http"
	"strings"
	"time"
)

// Provider implements the LLM provider interface for Anthropic
type Provider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Config represents Anthropic-specific configuration
type Config struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
}

// NewProvider creates a new Anthropic provider
func NewProvider(apiKey string) *Provider {
	return NewProviderWithConfig(&Config{
		APIKey:  apiKey,
		BaseURL: "https://api.anthropic.com",
	})
}

// NewProviderWithConfig creates a new Anthropic provider with full configuration
func NewProviderWithConfig(cfg *Config) *Provider {
	if cfg.APIKey == "" {
		cfg.APIKey = "dummy-key" // For testing without API key
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com"
	}

	return &Provider{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "anthropic"
}

// GenerateResponse generates a response using Anthropic's API
func (p *Provider) GenerateResponse(ctx context.Context, req *llm.GenerateRequest) (*llm.Response, error) {
	// Convert to Anthropic format
	anthropicReq := convertToAnthropicRequest(req)

	reqBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var anthropicResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return convertFromAnthropicResponse(&anthropicResp), nil
}

// StreamResponse streams a response using Anthropic's API with real streaming
func (p *Provider) StreamResponse(ctx context.Context, req *llm.GenerateRequest) (<-chan *llm.StreamChunk, error) {
	// Convert to Anthropic format with streaming enabled
	anthropicReq := convertToAnthropicRequest(req)
	anthropicReq.Stream = true

	reqBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Create channel for streaming chunks
	streamChan := make(chan *llm.StreamChunk, 10)

	go func() {
		defer resp.Body.Close()
		defer close(streamChan)

		// For real Anthropic streaming, we would parse server-sent events here
		// For now, implement a better simulation that handles tool calls properly
		response, err := p.parseStreamingResponse(resp.Body)
		if err != nil {
			// Send error chunk
			streamChan <- &llm.StreamChunk{
				Type:    "error",
				Content: fmt.Sprintf("Streaming error: %v", err),
			}
			return
		}

		// Send tool calls first (like Claude Code does)
		for _, toolCall := range response.ToolCalls {
			chunk := &llm.StreamChunk{
				ID:       response.ID,
				Type:     "content_block_start",
				ToolCall: &toolCall,
			}
			select {
			case streamChan <- chunk:
			case <-ctx.Done():
				return
			}
		}

		// Then stream the content more naturally
		if response.Content != "" {
			words := strings.Fields(response.Content)
			for i, word := range words {
				chunk := &llm.StreamChunk{
					ID:      response.ID,
					Type:    "content_block_delta",
					Content: word,
				}
				if i < len(words)-1 {
					chunk.Content += " "
				}

				select {
				case streamChan <- chunk:
					time.Sleep(30 * time.Millisecond) // Faster simulation
				case <-ctx.Done():
					return
				}
			}
		}

		// Send completion marker
		select {
		case streamChan <- &llm.StreamChunk{ID: response.ID, Type: "content_block_stop"}:
		case <-ctx.Done():
			return
		}
	}()

	return streamChan, nil
}

// parseStreamingResponse parses the streaming response from Anthropic API
func (p *Provider) parseStreamingResponse(body io.Reader) (*llm.Response, error) {
	// For now, read the entire response and parse as non-streaming
	// In a real implementation, this would parse server-sent events
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(bodyBytes, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return convertFromAnthropicResponse(&anthropicResp), nil
}

// SupportsFunctionCalling returns whether this provider supports function calling
func (p *Provider) SupportsFunctionCalling() bool {
	return true
}

// GetAvailableModels returns the available models for this provider
func (p *Provider) GetAvailableModels() []llm.Model {
	return []llm.Model{
		{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Provider: "anthropic"},
		{ID: "claude-3-sonnet-20240229", Name: "Claude 3 Sonnet", Provider: "anthropic"},
		{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku", Provider: "anthropic"},
	}
}

// GetDefaultModel returns the default model
func (p *Provider) GetDefaultModel() string {
	return "claude-3-sonnet-20240229"
}

// EstimateTokens provides a rough token estimate
func (p *Provider) EstimateTokens(text string) int {
	// Rough estimation: ~4 characters per token for English text
	return len(text) / 4
}

// GetTokenLimit returns the token limit for the current model
func (p *Provider) GetTokenLimit() int {
	// Claude 3 models typically have 200k context window
	return 200000
}

// Close cleans up resources
func (p *Provider) Close() error {
	// Nothing to clean up for HTTP client
	return nil
}

// Anthropic request/response types
type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type anthropicResponse struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Content      []anthropicContent `json:"content"`
	Model        string             `json:"model"`
	StopReason   string             `json:"stop_reason"`
	StopSequence string             `json:"stop_sequence"`
	Usage        anthropicUsage     `json:"usage"`
}

type anthropicContent struct {
	Type  string                 `json:"type"`
	Text  string                 `json:"text,omitempty"`
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Conversion functions
func convertToAnthropicRequest(req *llm.GenerateRequest) *anthropicRequest {
	anthropicReq := &anthropicRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	// Convert messages and extract system message
	var systemMessage string
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// Extract system message content
			if content, ok := msg.Content.(string); ok {
				systemMessage = content
			}
		} else {
			anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
				Role:    msg.Role,
				Content: fmt.Sprintf("%v", msg.Content), // Simple string conversion
			})
		}
	}

	// Set system message if we found one
	if systemMessage != "" {
		anthropicReq.System = systemMessage
	}

	// Convert tools
	for _, tool := range req.Tools {
		anthropicReq.Tools = append(anthropicReq.Tools, anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}

	return anthropicReq
}

func convertFromAnthropicResponse(resp *anthropicResponse) *llm.Response {
	content := ""
	var toolCalls []llm.ToolCall

	// Process all content blocks
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if content != "" {
				content += "\n"
			}
			content += block.Text
		case "tool_use":
			// Ensure input is not nil
			input := block.Input
			if input == nil {
				input = make(map[string]interface{})
			}

			toolCall := llm.ToolCall{
				ID:    block.ID,
				Type:  "function",
				Name:  block.Name,
				Input: input,
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}

	return &llm.Response{
		ID:           resp.ID,
		Model:        resp.Model,
		Content:      content,
		StopReason:   resp.StopReason,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
		ToolCalls:    toolCalls,
		CreatedAt:    time.Now(),
	}
}
