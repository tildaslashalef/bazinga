package openai

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

// Provider implements the LLM provider interface for OpenAI
type Provider struct {
	apiKey     string
	baseURL    string
	orgID      string
	httpClient *http.Client
}

// Config represents OpenAI-specific configuration
type Config struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
	OrgID   string `yaml:"org_id"`
}

// NewProvider creates a new OpenAI provider
func NewProvider(apiKey string) *Provider {
	return NewProviderWithConfig(&Config{
		APIKey:  apiKey,
		BaseURL: "https://api.openai.com/v1",
	})
}

// NewProviderWithConfig creates a new OpenAI provider with full configuration
func NewProviderWithConfig(cfg *Config) *Provider {
	if cfg.APIKey == "" {
		cfg.APIKey = "dummy-key" // For testing without API key
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}

	return &Provider{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		orgID:   cfg.OrgID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "openai"
}

// GenerateResponse generates a response using OpenAI's API
func (p *Provider) GenerateResponse(ctx context.Context, req *llm.GenerateRequest) (*llm.Response, error) {
	// Convert to OpenAI format
	openAIReq := convertToOpenAIRequest(req)

	reqBody, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	if p.orgID != "" {
		httpReq.Header.Set("OpenAI-Organization", p.orgID)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var openAIResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return convertFromOpenAIResponse(&openAIResp), nil
}

// StreamResponse streams a response using OpenAI's API
func (p *Provider) StreamResponse(ctx context.Context, req *llm.GenerateRequest) (<-chan *llm.StreamChunk, error) {
	// For MVP, use non-streaming and simulate streaming
	response, err := p.GenerateResponse(ctx, req)
	if err != nil {
		return nil, err
	}

	// Create a channel and simulate streaming
	streamChan := make(chan *llm.StreamChunk, 1)
	go func() {
		defer close(streamChan)

		// Simulate streaming by sending the content in chunks
		words := strings.Fields(response.Content)
		for i, word := range words {
			chunk := &llm.StreamChunk{
				ID:      response.ID,
				Content: word,
			}
			if i < len(words)-1 {
				chunk.Content += " "
			}

			select {
			case streamChan <- chunk:
				time.Sleep(50 * time.Millisecond) // Simulate streaming delay
			case <-ctx.Done():
				return
			}
		}

		// Send tool calls if any
		for _, toolCall := range response.ToolCalls {
			chunk := &llm.StreamChunk{
				ID:       response.ID,
				ToolCall: &toolCall,
			}
			select {
			case streamChan <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()

	return streamChan, nil
}

// SupportsFunctionCalling returns whether this provider supports function calling
func (p *Provider) SupportsFunctionCalling() bool {
	return true
}

// GetAvailableModels returns the available models for this provider
func (p *Provider) GetAvailableModels() []llm.Model {
	return []llm.Model{
		{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
		{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Provider: "openai"},
		{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Provider: "openai"},
	}
}

// GetDefaultModel returns the default model
func (p *Provider) GetDefaultModel() string {
	return "gpt-4-turbo"
}

// EstimateTokens provides a rough token estimate
func (p *Provider) EstimateTokens(text string) int {
	// Rough estimation: ~4 characters per token for English text
	return len(text) / 4
}

// GetTokenLimit returns the token limit for the current model
func (p *Provider) GetTokenLimit() int {
	// GPT-4 models typically have 8k-128k context window, using conservative estimate
	return 8000
}

// Close cleans up resources
func (p *Provider) Close() error {
	// Nothing to clean up for HTTP client
	return nil
}

// OpenAI request/response types
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Tools       []openAITool    `json:"tools,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type openAIResponse struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

type openAIChoice struct {
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Conversion functions
func convertToOpenAIRequest(req *llm.GenerateRequest) *openAIRequest {
	openAIReq := &openAIRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	// Convert messages
	for _, msg := range req.Messages {
		openAIReq.Messages = append(openAIReq.Messages, openAIMessage{
			Role:    msg.Role,
			Content: fmt.Sprintf("%v", msg.Content), // Simple string conversion
		})
	}

	// Convert tools
	for _, tool := range req.Tools {
		openAIReq.Tools = append(openAIReq.Tools, openAITool{
			Type: "function",
			Function: openAIToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}

	return openAIReq
}

func convertFromOpenAIResponse(resp *openAIResponse) *llm.Response {
	if len(resp.Choices) == 0 {
		return &llm.Response{
			ID:    resp.ID,
			Model: resp.Model,
		}
	}

	choice := resp.Choices[0]
	return &llm.Response{
		ID:           resp.ID,
		Model:        resp.Model,
		Content:      choice.Message.Content,
		StopReason:   choice.FinishReason,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
		CreatedAt:    time.Now(),
	}
}
