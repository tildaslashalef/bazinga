package anthropic

import (
	"context"
	"encoding/json"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
	}{
		{"with API key", "test-api-key"},
		{"empty API key", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewProvider(tt.apiKey)

			if provider == nil {
				t.Fatal("Provider should not be nil")
			}

			if tt.apiKey == "" {
				// Should use dummy key when empty
				if provider.apiKey != "dummy-key" {
					t.Errorf("Expected dummy-key, got %s", provider.apiKey)
				}
			} else {
				if provider.apiKey != tt.apiKey {
					t.Errorf("Expected %s, got %s", tt.apiKey, provider.apiKey)
				}
			}

			if provider.baseURL != "https://api.anthropic.com" {
				t.Errorf("Expected https://api.anthropic.com, got %s", provider.baseURL)
			}

			if provider.httpClient == nil {
				t.Error("HTTP client should not be nil")
			}
		})
	}
}

func TestProvider_Name(t *testing.T) {
	provider := NewProvider("test-key")

	if provider.Name() != "anthropic" {
		t.Errorf("Expected 'anthropic', got %s", provider.Name())
	}
}

func TestProvider_SupportsFunctionCalling(t *testing.T) {
	provider := NewProvider("test-key")

	if !provider.SupportsFunctionCalling() {
		t.Error("Anthropic provider should support function calling")
	}
}

func TestProvider_GetAvailableModels(t *testing.T) {
	provider := NewProvider("test-key")
	models := provider.GetAvailableModels()

	expectedModels := []string{
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	}

	if len(models) != len(expectedModels) {
		t.Errorf("Expected %d models, got %d", len(expectedModels), len(models))
	}

	for i, expectedID := range expectedModels {
		if i >= len(models) {
			t.Errorf("Missing model %s", expectedID)
			continue
		}

		if models[i].ID != expectedID {
			t.Errorf("Expected model ID %s, got %s", expectedID, models[i].ID)
		}

		if models[i].Provider != "anthropic" {
			t.Errorf("Expected provider 'anthropic', got %s", models[i].Provider)
		}
	}
}

func TestProvider_GetDefaultModel(t *testing.T) {
	provider := NewProvider("test-key")

	defaultModel := provider.GetDefaultModel()
	if defaultModel != "claude-3-sonnet-20240229" {
		t.Errorf("Expected 'claude-3-sonnet-20240229', got %s", defaultModel)
	}
}

func TestProvider_EstimateTokens(t *testing.T) {
	provider := NewProvider("test-key")

	tests := []struct {
		text     string
		expected int
	}{
		{"", 0},
		{"test", 1},
		{"hello world", 2},
		{"this is a longer text string", 7},
	}

	for _, tt := range tests {
		result := provider.EstimateTokens(tt.text)
		if result != tt.expected {
			t.Errorf("EstimateTokens(%q) = %d, expected %d", tt.text, result, tt.expected)
		}
	}
}

func TestProvider_GetTokenLimit(t *testing.T) {
	provider := NewProvider("test-key")

	tokenLimit := provider.GetTokenLimit()
	if tokenLimit != 200000 {
		t.Errorf("Expected 200000, got %d", tokenLimit)
	}
}

func TestProvider_Close(t *testing.T) {
	provider := NewProvider("test-key")

	err := provider.Close()
	if err != nil {
		t.Errorf("Close should not return error, got %v", err)
	}
}

func TestProvider_GenerateResponse(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("x-api-key") == "" {
			t.Error("Expected x-api-key header")
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Expected Content-Type: application/json")
		}

		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Error("Expected anthropic-version header")
		}

		// Mock response
		response := anthropicResponse{
			ID:   "test-id",
			Type: "message",
			Role: "assistant",
			Content: []anthropicContent{
				{Type: "text", Text: "Hello, this is a test response"},
			},
			Model:      "claude-3-sonnet-20240229",
			StopReason: "end_turn",
			Usage: anthropicUsage{
				InputTokens:  10,
				OutputTokens: 20,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewProvider("test-api-key")
	provider.baseURL = server.URL // Override for testing

	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Hello"},
		},
		Model:       "claude-3-sonnet-20240229",
		MaxTokens:   100,
		Temperature: 0.7,
	}

	ctx := context.Background()
	response, err := provider.GenerateResponse(ctx, req)
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	if response == nil {
		t.Fatal("Response should not be nil")
	}

	if response.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", response.ID)
	}

	if response.Content != "Hello, this is a test response" {
		t.Errorf("Unexpected response content: %s", response.Content)
	}

	if response.Model != "claude-3-sonnet-20240229" {
		t.Errorf("Expected model 'claude-3-sonnet-20240229', got %s", response.Model)
	}

	if response.InputTokens != 10 {
		t.Errorf("Expected 10 input tokens, got %d", response.InputTokens)
	}

	if response.OutputTokens != 20 {
		t.Errorf("Expected 20 output tokens, got %d", response.OutputTokens)
	}
}

func TestProvider_GenerateResponse_APIError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(`{"error": "Invalid request"}`)); err != nil {
			t.Errorf("Failed to write error response: %v", err)
		}
	}))
	defer server.Close()

	provider := NewProvider("test-api-key")
	provider.baseURL = server.URL // Override for testing

	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx := context.Background()
	_, err := provider.GenerateResponse(ctx, req)

	if err == nil {
		t.Error("Expected error for API error response")
	}

	if !strings.Contains(err.Error(), "API request failed") {
		t.Errorf("Expected API error message, got: %v", err)
	}
}

func TestProvider_StreamResponse(t *testing.T) {
	// Create mock server for non-streaming response (simulated streaming)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := anthropicResponse{
			ID:   "test-stream-id",
			Type: "message",
			Role: "assistant",
			Content: []anthropicContent{
				{Type: "text", Text: "Hello world test"},
			},
			Model:      "claude-3-sonnet-20240229",
			StopReason: "end_turn",
			Usage: anthropicUsage{
				InputTokens:  5,
				OutputTokens: 10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewProvider("test-api-key")
	provider.baseURL = server.URL // Override for testing

	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Hello"},
		},
		Model: "claude-3-sonnet-20240229",
	}

	ctx := context.Background()
	streamChan, err := provider.StreamResponse(ctx, req)
	if err != nil {
		t.Fatalf("StreamResponse failed: %v", err)
	}

	if streamChan == nil {
		t.Fatal("Stream channel should not be nil")
	}

	// Collect all chunks
	var chunks []*llm.StreamChunk
	for chunk := range streamChan {
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
	}

	// Verify chunks have correct ID
	for _, chunk := range chunks {
		if chunk.ID != "test-stream-id" {
			t.Errorf("Expected chunk ID 'test-stream-id', got %s", chunk.ID)
		}
	}

	// Reconstruct content from chunks
	var content strings.Builder
	for _, chunk := range chunks {
		content.WriteString(chunk.Content)
	}

	expectedContent := "Hello world test"
	if content.String() != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, content.String())
	}
}

func TestProvider_StreamResponse_WithCancel(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := anthropicResponse{
			ID:   "test-cancel-id",
			Type: "message",
			Role: "assistant",
			Content: []anthropicContent{
				{Type: "text", Text: "This is a long response that should be canceled"},
			},
			Model:      "claude-3-sonnet-20240229",
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 5, OutputTokens: 15},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewProvider("test-api-key")
	provider.baseURL = server.URL

	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Tell me a long story"},
		},
	}

	// Create context that will be canceled
	ctx, cancel := context.WithCancel(context.Background())

	streamChan, err := provider.StreamResponse(ctx, req)
	if err != nil {
		t.Fatalf("StreamResponse failed: %v", err)
	}

	// Cancel context after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// Try to read from stream - should eventually close due to cancellation
	chunks := 0
	for chunk := range streamChan {
		chunks++
		_ = chunk // Just acknowledge we received it

		// Don't wait forever
		if chunks > 50 {
			t.Error("Stream should have been canceled by now")
			break
		}
	}
}

func TestConvertToAnthropicRequest(t *testing.T) {
	req := &llm.GenerateRequest{
		Model:       "claude-3-sonnet-20240229",
		MaxTokens:   100,
		Temperature: 0.7,
		Messages: []llm.Message{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "Hello"},
		},
		Tools: []llm.Tool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: map[string]interface{}{"type": "object"},
			},
		},
	}

	anthropicReq := convertToAnthropicRequest(req)

	if anthropicReq.Model != "claude-3-sonnet-20240229" {
		t.Errorf("Expected model 'claude-3-sonnet-20240229', got %s", anthropicReq.Model)
	}

	if anthropicReq.MaxTokens != 100 {
		t.Errorf("Expected MaxTokens 100, got %d", anthropicReq.MaxTokens)
	}

	if anthropicReq.Temperature != 0.7 {
		t.Errorf("Expected Temperature 0.7, got %f", anthropicReq.Temperature)
	}

	// Should skip system messages
	if len(anthropicReq.Messages) != 1 {
		t.Errorf("Expected 1 message (skipping system), got %d", len(anthropicReq.Messages))
	}

	if anthropicReq.Messages[0].Role != "user" {
		t.Errorf("Expected user message, got %s", anthropicReq.Messages[0].Role)
	}

	if len(anthropicReq.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(anthropicReq.Tools))
	}

	if anthropicReq.Tools[0].Name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got %s", anthropicReq.Tools[0].Name)
	}
}

func TestConvertFromAnthropicResponse(t *testing.T) {
	anthropicResp := &anthropicResponse{
		ID:   "test-response-id",
		Type: "message",
		Role: "assistant",
		Content: []anthropicContent{
			{Type: "text", Text: "Hello, how can I help you?"},
		},
		Model:      "claude-3-sonnet-20240229",
		StopReason: "end_turn",
		Usage: anthropicUsage{
			InputTokens:  15,
			OutputTokens: 25,
		},
	}

	response := convertFromAnthropicResponse(anthropicResp)

	if response.ID != "test-response-id" {
		t.Errorf("Expected ID 'test-response-id', got %s", response.ID)
	}

	if response.Model != "claude-3-sonnet-20240229" {
		t.Errorf("Expected model 'claude-3-sonnet-20240229', got %s", response.Model)
	}

	if response.Content != "Hello, how can I help you?" {
		t.Errorf("Expected content 'Hello, how can I help you?', got %s", response.Content)
	}

	if response.StopReason != "end_turn" {
		t.Errorf("Expected StopReason 'end_turn', got %s", response.StopReason)
	}

	if response.InputTokens != 15 {
		t.Errorf("Expected InputTokens 15, got %d", response.InputTokens)
	}

	if response.OutputTokens != 25 {
		t.Errorf("Expected OutputTokens 25, got %d", response.OutputTokens)
	}

	if response.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestConvertFromAnthropicResponse_EmptyContent(t *testing.T) {
	anthropicResp := &anthropicResponse{
		ID:      "test-id",
		Content: []anthropicContent{}, // Empty content array
		Model:   "claude-3-sonnet-20240229",
		Usage:   anthropicUsage{InputTokens: 5, OutputTokens: 0},
	}

	response := convertFromAnthropicResponse(anthropicResp)

	if response.Content != "" {
		t.Errorf("Expected empty content, got %s", response.Content)
	}
}

func TestProvider_GenerateResponse_NetworkError(t *testing.T) {
	provider := NewProvider("test-api-key")
	provider.baseURL = "http://localhost:99999" // Invalid URL to trigger network error

	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx := context.Background()
	_, err := provider.GenerateResponse(ctx, req)

	if err == nil {
		t.Error("Expected network error")
	}

	if !strings.Contains(err.Error(), "failed to send request") {
		t.Errorf("Expected network error message, got: %v", err)
	}
}

func TestProvider_GenerateResponse_InvalidJSON(t *testing.T) {
	// Create mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"invalid": json`)); err != nil { // Invalid JSON
			t.Errorf("Failed to write invalid JSON response: %v", err)
		}
	}))
	defer server.Close()

	provider := NewProvider("test-api-key")
	provider.baseURL = server.URL

	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	ctx := context.Background()
	_, err := provider.GenerateResponse(ctx, req)

	if err == nil {
		t.Error("Expected JSON decode error")
	}

	if !strings.Contains(err.Error(), "failed to decode response") {
		t.Errorf("Expected JSON decode error message, got: %v", err)
	}
}
