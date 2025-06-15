package ollama

import (
	"bufio"
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

// Provider implements the LLM provider interface for Ollama
type Provider struct {
	baseURL      string
	httpClient   *http.Client
	defaultModel string
}

// Config represents Ollama-specific configuration
type Config struct {
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"` // Default model to use
}

// NewProvider creates a new Ollama provider with default configuration
func NewProvider() *Provider {
	return NewProviderWithConfig(&Config{
		BaseURL: "http://localhost:11434",
		Model:   "qwen3:latest",
	})
}

// NewProviderWithConfig creates a new Ollama provider with full configuration
func NewProviderWithConfig(cfg *Config) *Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}

	if cfg.Model == "" {
		cfg.Model = "qwen3:latest"
	}

	return &Provider{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // Ollama can be slow for large models
		},
		defaultModel: cfg.Model,
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "ollama"
}

// GenerateResponse generates a response using Ollama's API
func (p *Provider) GenerateResponse(ctx context.Context, req *llm.GenerateRequest) (*llm.Response, error) {
	// Convert to Ollama format
	ollamaReq := convertToOllamaRequest(req)

	reqBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	startTime := time.Now()
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return convertFromOllamaResponse(&ollamaResp, time.Since(startTime)), nil
}

// StreamResponse streams a response using Ollama's API
func (p *Provider) StreamResponse(ctx context.Context, req *llm.GenerateRequest) (<-chan *llm.StreamChunk, error) {
	// Convert to Ollama format with streaming enabled
	ollamaReq := convertToOllamaRequest(req)
	ollamaReq.Stream = true

	reqBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	streamChan := make(chan *llm.StreamChunk, 10)

	go func() {
		defer close(streamChan)
		defer func() { _ = resp.Body.Close() }()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var streamResp ollamaStreamResponse
			if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
				continue // Skip malformed lines
			}

			chunk := convertFromOllamaStreamResponse(&streamResp)
			if chunk != nil {
				select {
				case streamChan <- chunk:
				case <-ctx.Done():
					return
				}
			}

			// Check if this is the final chunk
			if streamResp.Done {
				break
			}
		}
	}()

	return streamChan, nil
}

// SupportsFunctionCalling returns true if the provider supports function calling
func (p *Provider) SupportsFunctionCalling() bool {
	return true // Many modern Ollama models support function calling
}

// GetAvailableModels returns the list of available models
func (p *Provider) GetAvailableModels() []llm.Model {
	// Return a single model based on the configured default
	return []llm.Model{
		{
			ID:              p.defaultModel,
			Name:            p.defaultModel,
			Provider:        "ollama",
			MaxTokens:       4096,
			SupportsTools:   true,
			CostPer1KTokens: 0.0, // Free local inference
		},
	}
}

// GetDefaultModel returns the default model for this provider
func (p *Provider) GetDefaultModel() string {
	return p.defaultModel
}

// EstimateTokens estimates token count for given text (rough approximation)
func (p *Provider) EstimateTokens(text string) int {
	// Rough approximation: ~4 characters per token for English text
	return len(text) / 4
}

// GetTokenLimit returns the max token limit for the current model
func (p *Provider) GetTokenLimit() int {
	// Return a reasonable default
	return 4096
}

// Close cleans up provider resources
func (p *Provider) Close() error {
	// No persistent connections to close for Ollama
	return nil
}

// Ollama API types
type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream,omitempty"`
	Options  *ollamaOptions  `json:"options,omitempty"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"` // max_tokens equivalent
}

type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ollamaToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function ollamaToolCallFunction `json:"function"`
}

type ollamaToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ollamaResponse struct {
	Model     string        `json:"model"`
	Message   ollamaMessage `json:"message"`
	Done      bool          `json:"done"`
	CreatedAt string        `json:"created_at"`
}

type ollamaStreamResponse struct {
	Model     string        `json:"model"`
	Message   ollamaMessage `json:"message"`
	Done      bool          `json:"done"`
	CreatedAt string        `json:"created_at"`
}

// Conversion functions
func convertToOllamaRequest(req *llm.GenerateRequest) *ollamaRequest {
	ollamaReq := &ollamaRequest{
		Model:    req.Model,
		Messages: make([]ollamaMessage, len(req.Messages)),
		Stream:   false,
	}

	// Set default model if not specified
	if ollamaReq.Model == "" {
		ollamaReq.Model = "qwen3:latest"
	}

	// Convert messages
	for i, msg := range req.Messages {
		ollamaReq.Messages[i] = ollamaMessage{
			Role: msg.Role,
		}

		// Convert content (handle both string and structured content)
		if str, ok := msg.Content.(string); ok {
			ollamaReq.Messages[i].Content = str
		} else {
			// For structured content, extract text parts
			ollamaReq.Messages[i].Content = extractTextFromContent(msg.Content)
		}
	}

	// Set options
	if req.Temperature > 0 || req.MaxTokens > 0 {
		ollamaReq.Options = &ollamaOptions{}
		if req.Temperature > 0 {
			ollamaReq.Options.Temperature = req.Temperature
		}
		if req.MaxTokens > 0 {
			ollamaReq.Options.NumPredict = req.MaxTokens
		}
	}

	// Convert tools
	if len(req.Tools) > 0 {
		ollamaReq.Tools = make([]ollamaTool, len(req.Tools))
		for i, tool := range req.Tools {
			ollamaReq.Tools[i] = ollamaTool{
				Type: "function",
				Function: ollamaToolFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
			}
		}
	}

	return ollamaReq
}

func convertFromOllamaResponse(resp *ollamaResponse, duration time.Duration) *llm.Response {
	response := &llm.Response{
		ID:               fmt.Sprintf("ollama-%d", time.Now().UnixNano()),
		Model:            resp.Model,
		Content:          resp.Message.Content,
		StopReason:       "stop",
		ProcessingTimeMs: duration.Milliseconds(),
		CreatedAt:        time.Now(),
	}

	// Convert tool calls if present
	if len(resp.Message.ToolCalls) > 0 {
		response.ToolCalls = make([]llm.ToolCall, len(resp.Message.ToolCalls))
		for i, tc := range resp.Message.ToolCalls {
			response.ToolCalls[i] = llm.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: &llm.Function{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	// Estimate tokens (Ollama doesn't provide exact counts)
	response.InputTokens = len(resp.Message.Content) / 4 // Rough estimate
	response.OutputTokens = len(resp.Message.Content) / 4

	return response
}

func convertFromOllamaStreamResponse(resp *ollamaStreamResponse) *llm.StreamChunk {
	if resp.Message.Content == "" && len(resp.Message.ToolCalls) == 0 {
		return nil
	}

	chunk := &llm.StreamChunk{
		ID:      fmt.Sprintf("ollama-stream-%d", time.Now().UnixNano()),
		Type:    "content_block_delta",
		Content: resp.Message.Content,
	}

	// Handle tool calls in streaming
	if len(resp.Message.ToolCalls) > 0 {
		chunk.Type = "tool_completion"
		chunk.ToolCall = &llm.ToolCall{
			ID:   resp.Message.ToolCalls[0].ID,
			Type: resp.Message.ToolCalls[0].Type,
			Function: &llm.Function{
				Name:      resp.Message.ToolCalls[0].Function.Name,
				Arguments: resp.Message.ToolCalls[0].Function.Arguments,
			},
		}
	}

	return chunk
}

func extractTextFromContent(content interface{}) string {
	if str, ok := content.(string); ok {
		return str
	}

	if blocks, ok := content.([]llm.ContentBlock); ok {
		var texts []string
		for _, block := range blocks {
			if block.Type == "text" && block.Text != "" {
				texts = append(texts, block.Text)
			}
		}
		return strings.Join(texts, "\n")
	}

	// Try to convert to string as fallback
	if data, err := json.Marshal(content); err == nil {
		return string(data)
	}

	return ""
}
