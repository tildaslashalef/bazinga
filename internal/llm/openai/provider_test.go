package openai

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

			if provider.baseURL != "https://api.openai.com/v1" {
				t.Errorf("Expected https://api.openai.com/v1, got %s", provider.baseURL)
			}

			if provider.httpClient == nil {
				t.Error("HTTP client should not be nil")
			}
		})
	}
}

func TestProvider_Name(t *testing.T) {
	provider := NewProvider("test-key")

	if provider.Name() != "openai" {
		t.Errorf("Expected 'openai', got %s", provider.Name())
	}
}

func TestProvider_SupportsFunctionCalling(t *testing.T) {
	provider := NewProvider("test-key")

	if !provider.SupportsFunctionCalling() {
		t.Error("OpenAI provider should support function calling")
	}
}

func TestProvider_GetAvailableModels(t *testing.T) {
	provider := NewProvider("test-key")
	models := provider.GetAvailableModels()

	expectedModels := []string{
		"gpt-4",
		"gpt-4-turbo",
		"gpt-3.5-turbo",
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

		if models[i].Provider != "openai" {
			t.Errorf("Expected provider 'openai', got %s", models[i].Provider)
		}
	}
}

func TestProvider_GetDefaultModel(t *testing.T) {
	provider := NewProvider("test-key")

	defaultModel := provider.GetDefaultModel()
	if defaultModel != "gpt-4-turbo" {
		t.Errorf("Expected 'gpt-4-turbo', got %s", defaultModel)
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
	if tokenLimit != 8000 {
		t.Errorf("Expected 8000, got %d", tokenLimit)
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

		if r.Header.Get("Authorization") == "" {
			t.Error("Expected Authorization header")
		}

		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("Expected Bearer token in Authorization header")
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Expected Content-Type: application/json")
		}

		// Verify request body structure
		var reqBody openAIRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if reqBody.Model == "" {
			t.Error("Request should include model")
		}

		if len(reqBody.Messages) == 0 {
			t.Error("Request should include messages")
		}

		// Mock response
		response := openAIResponse{
			ID:    "chatcmpl-test-id",
			Model: "gpt-4-turbo",
			Choices: []openAIChoice{
				{
					Message: openAIMessage{
						Role:    "assistant",
						Content: "Hello! This is a test response from OpenAI.",
					},
					FinishReason: "stop",
				},
			},
			Usage: openAIUsage{
				PromptTokens:     15,
				CompletionTokens: 25,
				TotalTokens:      40,
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
		Model:       "gpt-4-turbo",
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

	if response.ID != "chatcmpl-test-id" {
		t.Errorf("Expected ID 'chatcmpl-test-id', got %s", response.ID)
	}

	if response.Content != "Hello! This is a test response from OpenAI." {
		t.Errorf("Unexpected response content: %s", response.Content)
	}

	if response.Model != "gpt-4-turbo" {
		t.Errorf("Expected model 'gpt-4-turbo', got %s", response.Model)
	}

	if response.InputTokens != 15 {
		t.Errorf("Expected 15 input tokens, got %d", response.InputTokens)
	}

	if response.OutputTokens != 25 {
		t.Errorf("Expected 25 output tokens, got %d", response.OutputTokens)
	}

	if response.StopReason != "stop" {
		t.Errorf("Expected stop reason 'stop', got %s", response.StopReason)
	}
}

func TestProvider_GenerateResponse_WithTools(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request includes tools
		var reqBody openAIRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if len(reqBody.Tools) == 0 {
			t.Error("Request should include tools")
		}

		if reqBody.Tools[0].Type != "function" {
			t.Errorf("Expected tool type 'function', got %s", reqBody.Tools[0].Type)
		}

		if reqBody.Tools[0].Function.Name != "get_weather" {
			t.Errorf("Expected function name 'get_weather', got %s", reqBody.Tools[0].Function.Name)
		}

		// Mock response with function call
		response := openAIResponse{
			ID:    "chatcmpl-test-id",
			Model: "gpt-4-turbo",
			Choices: []openAIChoice{
				{
					Message: openAIMessage{
						Role:    "assistant",
						Content: "I'll check the weather for you.",
					},
					FinishReason: "function_call",
				},
			},
			Usage: openAIUsage{
				PromptTokens:     20,
				CompletionTokens: 10,
				TotalTokens:      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewProvider("test-api-key")
	provider.baseURL = server.URL

	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "What's the weather like?"},
		},
		Model: "gpt-4-turbo",
		Tools: []llm.Tool{
			{
				Name:        "get_weather",
				Description: "Get current weather",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "Location to get weather for",
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	response, err := provider.GenerateResponse(ctx, req)
	if err != nil {
		t.Fatalf("GenerateResponse with tools failed: %v", err)
	}

	if response.StopReason != "function_call" {
		t.Errorf("Expected stop reason 'function_call', got %s", response.StopReason)
	}
}

func TestProvider_GenerateResponse_APIError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		if _, err := w.Write([]byte(`{"error": {"message": "Invalid API key", "type": "invalid_request_error"}}`)); err != nil {
			t.Errorf("Failed to write error response: %v", err)
		}
	}))
	defer server.Close()

	provider := NewProvider("invalid-api-key")
	provider.baseURL = server.URL

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
		response := openAIResponse{
			ID:    "chatcmpl-stream-test",
			Model: "gpt-4-turbo",
			Choices: []openAIChoice{
				{
					Message: openAIMessage{
						Role:    "assistant",
						Content: "Hello streaming world",
					},
					FinishReason: "stop",
				},
			},
			Usage: openAIUsage{
				PromptTokens:     10,
				CompletionTokens: 15,
				TotalTokens:      25,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider := NewProvider("test-api-key")
	provider.baseURL = server.URL

	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Hello"},
		},
		Model: "gpt-4-turbo",
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
		if chunk.ID != "chatcmpl-stream-test" {
			t.Errorf("Expected chunk ID 'chatcmpl-stream-test', got %s", chunk.ID)
		}
	}

	// Reconstruct content from chunks
	var content strings.Builder
	for _, chunk := range chunks {
		content.WriteString(chunk.Content)
	}

	expectedContent := "Hello streaming world"
	if content.String() != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, content.String())
	}
}

func TestProvider_StreamResponse_WithCancel(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := openAIResponse{
			ID:    "chatcmpl-cancel-test",
			Model: "gpt-4-turbo",
			Choices: []openAIChoice{
				{
					Message: openAIMessage{
						Role:    "assistant",
						Content: "This is a long response that should be canceled",
					},
					FinishReason: "stop",
				},
			},
			Usage: openAIUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
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

func TestConvertToOpenAIRequest(t *testing.T) {
	req := &llm.GenerateRequest{
		Model:       "gpt-4-turbo",
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
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"param": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		},
	}

	openAIReq := convertToOpenAIRequest(req)

	if openAIReq.Model != "gpt-4-turbo" {
		t.Errorf("Expected model 'gpt-4-turbo', got %s", openAIReq.Model)
	}

	if openAIReq.MaxTokens != 100 {
		t.Errorf("Expected MaxTokens 100, got %d", openAIReq.MaxTokens)
	}

	if openAIReq.Temperature != 0.7 {
		t.Errorf("Expected Temperature 0.7, got %f", openAIReq.Temperature)
	}

	if len(openAIReq.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(openAIReq.Messages))
	}

	if openAIReq.Messages[0].Role != "system" {
		t.Errorf("Expected first message role 'system', got %s", openAIReq.Messages[0].Role)
	}

	if openAIReq.Messages[1].Role != "user" {
		t.Errorf("Expected second message role 'user', got %s", openAIReq.Messages[1].Role)
	}

	if len(openAIReq.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(openAIReq.Tools))
	}

	if openAIReq.Tools[0].Type != "function" {
		t.Errorf("Expected tool type 'function', got %s", openAIReq.Tools[0].Type)
	}

	if openAIReq.Tools[0].Function.Name != "test_tool" {
		t.Errorf("Expected function name 'test_tool', got %s", openAIReq.Tools[0].Function.Name)
	}
}

func TestConvertFromOpenAIResponse(t *testing.T) {
	openAIResp := &openAIResponse{
		ID:    "chatcmpl-test-response",
		Model: "gpt-4-turbo",
		Choices: []openAIChoice{
			{
				Message: openAIMessage{
					Role:    "assistant",
					Content: "Hello, how can I help you today?",
				},
				FinishReason: "stop",
			},
		},
		Usage: openAIUsage{
			PromptTokens:     20,
			CompletionTokens: 30,
			TotalTokens:      50,
		},
	}

	response := convertFromOpenAIResponse(openAIResp)

	if response.ID != "chatcmpl-test-response" {
		t.Errorf("Expected ID 'chatcmpl-test-response', got %s", response.ID)
	}

	if response.Model != "gpt-4-turbo" {
		t.Errorf("Expected model 'gpt-4-turbo', got %s", response.Model)
	}

	if response.Content != "Hello, how can I help you today?" {
		t.Errorf("Expected content 'Hello, how can I help you today?', got %s", response.Content)
	}

	if response.StopReason != "stop" {
		t.Errorf("Expected StopReason 'stop', got %s", response.StopReason)
	}

	if response.InputTokens != 20 {
		t.Errorf("Expected InputTokens 20, got %d", response.InputTokens)
	}

	if response.OutputTokens != 30 {
		t.Errorf("Expected OutputTokens 30, got %d", response.OutputTokens)
	}

	if response.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestConvertFromOpenAIResponse_NoChoices(t *testing.T) {
	openAIResp := &openAIResponse{
		ID:      "chatcmpl-no-choices",
		Model:   "gpt-4-turbo",
		Choices: []openAIChoice{}, // Empty choices
		Usage:   openAIUsage{PromptTokens: 5, CompletionTokens: 0, TotalTokens: 5},
	}

	response := convertFromOpenAIResponse(openAIResp)

	if response.Content != "" {
		t.Errorf("Expected empty content for no choices, got %s", response.Content)
	}

	if response.StopReason != "" {
		t.Errorf("Expected empty stop reason for no choices, got %s", response.StopReason)
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
		if _, err := w.Write([]byte(`{"invalid": json response`)); err != nil { // Invalid JSON
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

func TestConvertToOpenAIRequest_NoTools(t *testing.T) {
	req := &llm.GenerateRequest{
		Model: "gpt-3.5-turbo",
		Messages: []llm.Message{
			{Role: "user", Content: "Simple question"},
		},
		MaxTokens: 50,
	}

	openAIReq := convertToOpenAIRequest(req)

	if len(openAIReq.Tools) != 0 {
		t.Errorf("Expected no tools, got %d", len(openAIReq.Tools))
	}

	if openAIReq.Stream {
		t.Error("Stream should be false for non-streaming request")
	}
}

func TestProvider_MessageConversion(t *testing.T) {
	tests := []struct {
		name        string
		content     interface{}
		expectedStr string
	}{
		{"string content", "Hello world", "Hello world"},
		{"number content", 42, "42"},
		{"boolean content", true, "true"},
		{"map content", map[string]string{"key": "value"}, "map[key:value]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &llm.GenerateRequest{
				Messages: []llm.Message{
					{Role: "user", Content: tt.content},
				},
			}

			openAIReq := convertToOpenAIRequest(req)

			if len(openAIReq.Messages) != 1 {
				t.Fatalf("Expected 1 message, got %d", len(openAIReq.Messages))
			}

			if openAIReq.Messages[0].Content != tt.expectedStr {
				t.Errorf("Expected content '%s', got '%s'", tt.expectedStr, openAIReq.Messages[0].Content)
			}
		})
	}
}
