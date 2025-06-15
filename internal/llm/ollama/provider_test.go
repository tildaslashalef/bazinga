package ollama

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
	provider := NewProvider()

	if provider == nil {
		t.Fatal("NewProvider returned nil")
	}

	if provider.Name() != "ollama" {
		t.Errorf("Expected provider name 'ollama', got %s", provider.Name())
	}

	if provider.baseURL != "http://localhost:11434" {
		t.Errorf("Expected default baseURL 'http://localhost:11434', got %s", provider.baseURL)
	}

	if len(provider.GetAvailableModels()) == 0 {
		t.Error("Expected at least one available model")
	}
}

func TestNewProviderWithConfig(t *testing.T) {
	cfg := &Config{
		BaseURL: "http://custom:8080",
		Model:   "custom-model:latest",
	}

	provider := NewProviderWithConfig(cfg)

	if provider.baseURL != "http://custom:8080" {
		t.Errorf("Expected baseURL 'http://custom:8080', got %s", provider.baseURL)
	}

	if provider.GetDefaultModel() != "custom-model:latest" {
		t.Errorf("Expected default model 'custom-model:latest', got %s", provider.GetDefaultModel())
	}

	models := provider.GetAvailableModels()
	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}

	if models[0].ID != "custom-model:latest" {
		t.Errorf("Expected model ID 'custom-model:latest', got %s", models[0].ID)
	}
}

func TestProvider_GenerateResponse(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("Expected path '/api/chat', got %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		// Verify request body
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if req.Model != "qwen3:latest" {
			t.Errorf("Expected model 'qwen3:latest', got %s", req.Model)
		}

		if len(req.Messages) != 1 || req.Messages[0].Content != "Hello, world!" {
			t.Errorf("Unexpected messages: %+v", req.Messages)
		}

		// Send response
		response := ollamaResponse{
			Model: "qwen3:latest",
			Message: ollamaMessage{
				Role:    "assistant",
				Content: "Hello! How can I help you today?",
			},
			Done:      true,
			CreatedAt: time.Now().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create provider with test server
	cfg := &Config{
		BaseURL: server.URL,
	}
	provider := NewProviderWithConfig(cfg)

	// Test request
	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: "Hello, world!",
			},
		},
		Model:       "qwen3:latest",
		MaxTokens:   100,
		Temperature: 0.7,
	}

	ctx := context.Background()
	response, err := provider.GenerateResponse(ctx, req)

	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	if response == nil {
		t.Fatal("Response is nil")
	}

	if response.Model != "qwen3:latest" {
		t.Errorf("Expected model 'qwen3:latest', got %s", response.Model)
	}

	if response.Content != "Hello! How can I help you today?" {
		t.Errorf("Unexpected response content: %s", response.Content)
	}

	if response.StopReason != "stop" {
		t.Errorf("Expected stop reason 'stop', got %s", response.StopReason)
	}
}

func TestProvider_GenerateResponse_WithTools(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Verify tools were sent
		if len(req.Tools) != 1 || req.Tools[0].Function.Name != "get_weather" {
			t.Errorf("Expected 1 tool 'get_weather', got: %+v", req.Tools)
		}

		// Send response with tool call
		response := ollamaResponse{
			Model: "qwen3:latest",
			Message: ollamaMessage{
				Role: "assistant",
				ToolCalls: []ollamaToolCall{
					{
						ID:   "tool_123",
						Type: "function",
						Function: ollamaToolCallFunction{
							Name:      "get_weather",
							Arguments: `{"location": "San Francisco"}`,
						},
					},
				},
			},
			Done:      true,
			CreatedAt: time.Now().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewProviderWithConfig(&Config{BaseURL: server.URL})

	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "What's the weather in San Francisco?"},
		},
		Model: "qwen3:latest",
		Tools: []llm.Tool{
			{
				Name:        "get_weather",
				Description: "Get weather for a location",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The location to get weather for",
						},
					},
					"required": []string{"location"},
				},
			},
		},
	}

	ctx := context.Background()
	response, err := provider.GenerateResponse(ctx, req)

	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	if len(response.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(response.ToolCalls))
	}

	if response.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("Expected tool call 'get_weather', got %s", response.ToolCalls[0].Function.Name)
	}
}

func TestProvider_StreamResponse(t *testing.T) {
	// Create test server that sends streaming responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if !req.Stream {
			t.Error("Expected stream=true in request")
		}

		w.Header().Set("Content-Type", "application/json")

		// Send multiple chunks
		chunks := []string{
			"Hello",
			" there!",
			" How",
			" can",
			" I",
			" help?",
		}

		for i, chunk := range chunks {
			response := ollamaStreamResponse{
				Model: "qwen3:latest",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: chunk,
				},
				Done:      i == len(chunks)-1,
				CreatedAt: time.Now().Format(time.RFC3339),
			}

			data, _ := json.Marshal(response)
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n"))

			// Flush to send the chunk immediately
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}

			// Small delay between chunks
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	provider := NewProviderWithConfig(&Config{BaseURL: server.URL})

	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Hello"},
		},
		Model: "qwen3:latest",
	}

	ctx := context.Background()
	streamChan, err := provider.StreamResponse(ctx, req)

	if err != nil {
		t.Fatalf("StreamResponse failed: %v", err)
	}

	var chunks []string
	for chunk := range streamChan {
		if chunk.Content != "" {
			chunks = append(chunks, chunk.Content)
		}
	}

	expectedContent := "Hello there! How can I help?"
	actualContent := strings.Join(chunks, "")

	if actualContent != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, actualContent)
	}

	if len(chunks) != 6 {
		t.Errorf("Expected 6 chunks, got %d", len(chunks))
	}
}

func TestProvider_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	provider := NewProviderWithConfig(&Config{BaseURL: server.URL})

	req := &llm.GenerateRequest{
		Messages: []llm.Message{{Role: "user", Content: "test"}},
		Model:    "qwen3:latest",
	}

	ctx := context.Background()
	_, err := provider.GenerateResponse(ctx, req)

	if err == nil {
		t.Error("Expected error for HTTP 500")
	}

	if !strings.Contains(err.Error(), "API request failed with status 500") {
		t.Errorf("Expected API error message, got: %v", err)
	}
}

func TestProvider_SupportsFunctionCalling(t *testing.T) {
	provider := NewProvider()
	if !provider.SupportsFunctionCalling() {
		t.Error("Expected Ollama provider to support function calling")
	}
}

func TestProvider_EstimateTokens(t *testing.T) {
	provider := NewProvider()

	text := "Hello, world! This is a test."
	tokens := provider.EstimateTokens(text)

	// Should be roughly len(text)/4
	expected := len(text) / 4
	if tokens != expected {
		t.Errorf("Expected ~%d tokens, got %d", expected, tokens)
	}
}

func TestProvider_GetTokenLimit(t *testing.T) {
	provider := NewProvider()
	limit := provider.GetTokenLimit()

	if limit <= 0 {
		t.Error("Token limit should be positive")
	}
}

func TestProvider_Close(t *testing.T) {
	provider := NewProvider()
	err := provider.Close()

	if err != nil {
		t.Errorf("Close should not return error, got: %v", err)
	}
}

func TestExtractTextFromContent(t *testing.T) {
	tests := []struct {
		name     string
		content  interface{}
		expected string
	}{
		{
			name:     "string content",
			content:  "Hello, world!",
			expected: "Hello, world!",
		},
		{
			name: "content blocks",
			content: []llm.ContentBlock{
				{Type: "text", Text: "Hello"},
				{Type: "text", Text: "world"},
			},
			expected: "Hello\nworld",
		},
		{
			name:     "other type",
			content:  map[string]string{"text": "hello"},
			expected: `{"text":"hello"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTextFromContent(tt.content)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
