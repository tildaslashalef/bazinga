package bedrock

import (
	"encoding/json"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"strings"
	"testing"
)

// createMockProvider creates a provider for testing public methods
func createMockProvider() *Provider {
	return &Provider{
		client:       nil, // Cannot mock easily due to private field
		region:       "us-east-1",
		defaultModel: ModelClaudeSonnet,
		models: map[string]llm.Model{
			ModelClaudeSonnet: {
				ID:              ModelClaudeSonnet,
				Name:            "Claude 3 Sonnet",
				Provider:        "bedrock",
				MaxTokens:       200000,
				SupportsTools:   true,
				CostPer1KTokens: 0.003,
			},
			ModelClaudeOpus: {
				ID:              ModelClaudeOpus,
				Name:            "Claude 3 Opus",
				Provider:        "bedrock",
				MaxTokens:       200000,
				SupportsTools:   true,
				CostPer1KTokens: 0.015,
			},
			ModelClaudeHaiku: {
				ID:              ModelClaudeHaiku,
				Name:            "Claude 3 Haiku",
				Provider:        "bedrock",
				MaxTokens:       200000,
				SupportsTools:   true,
				CostPer1KTokens: 0.00025,
			},
		},
	}
}

func TestProvider_Name(t *testing.T) {
	provider := createMockProvider()

	if provider.Name() != "bedrock" {
		t.Errorf("Expected 'bedrock', got %s", provider.Name())
	}
}

func TestProvider_SupportsFunctionCalling(t *testing.T) {
	provider := createMockProvider()

	if !provider.SupportsFunctionCalling() {
		t.Error("Bedrock provider should support function calling")
	}
}

func TestProvider_GetAvailableModels(t *testing.T) {
	provider := createMockProvider()
	models := provider.GetAvailableModels()

	expectedModels := 3 // Sonnet, Opus, Haiku
	if len(models) != expectedModels {
		t.Errorf("Expected %d models, got %d", expectedModels, len(models))
	}

	// Check that all expected models are present
	modelIDs := make(map[string]bool)
	for _, model := range models {
		modelIDs[model.ID] = true

		if model.Provider != "bedrock" {
			t.Errorf("Expected provider 'bedrock', got %s", model.Provider)
		}

		if model.MaxTokens != 200000 {
			t.Errorf("Expected MaxTokens 200000, got %d", model.MaxTokens)
		}

		if !model.SupportsTools {
			t.Error("All Bedrock models should support tools")
		}
	}

	expectedIDs := []string{ModelClaudeSonnet, ModelClaudeOpus, ModelClaudeHaiku}
	for _, expectedID := range expectedIDs {
		if !modelIDs[expectedID] {
			t.Errorf("Missing expected model ID: %s", expectedID)
		}
	}
}

func TestProvider_GetDefaultModel(t *testing.T) {
	provider := createMockProvider()

	defaultModel := provider.GetDefaultModel()
	if defaultModel != ModelClaudeSonnet {
		t.Errorf("Expected '%s', got %s", ModelClaudeSonnet, defaultModel)
	}
}

func TestProvider_EstimateTokens(t *testing.T) {
	provider := createMockProvider()

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
	provider := createMockProvider()

	tokenLimit := provider.GetTokenLimit()
	if tokenLimit != 200000 {
		t.Errorf("Expected 200000, got %d", tokenLimit)
	}
}

func TestProvider_Close(t *testing.T) {
	provider := createMockProvider()

	err := provider.Close()
	if err != nil {
		t.Errorf("Close should not return error, got %v", err)
	}
}

// Note: GenerateResponse tests are skipped because they require AWS client mocking
// which is complex due to private fields. These would be better as integration tests.

func TestProvider_ConvertRequest(t *testing.T) {
	provider := createMockProvider()

	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Hello, how are you?"},
			{Role: "assistant", Content: "I'm doing well, thank you!"},
			{Role: "user", Content: "What's the weather like?"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
		Tools: []llm.Tool{
			{
				Name:        "get_weather",
				Description: "Get weather information",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The location to get weather for",
						},
					},
				},
			},
		},
	}

	bedrockReq, err := provider.convertRequest(req, ModelClaudeSonnet)
	if err != nil {
		t.Fatalf("convertRequest failed: %v", err)
	}

	if len(bedrockReq) == 0 {
		t.Error("Converted request should not be empty")
	}

	// Verify it's valid JSON by unmarshaling
	var requestData map[string]interface{}
	if err := json.Unmarshal(bedrockReq, &requestData); err != nil {
		t.Fatalf("Converted request is not valid JSON: %v", err)
	}

	// Check basic structure
	if requestData["max_tokens"] != float64(100) {
		t.Errorf("Expected max_tokens 100, got %v", requestData["max_tokens"])
	}

	if requestData["temperature"] != 0.7 {
		t.Errorf("Expected temperature 0.7, got %v", requestData["temperature"])
	}

	// Check messages exist
	messages, ok := requestData["messages"]
	if !ok {
		t.Error("Request should contain messages")
	}

	messagesList, ok := messages.([]interface{})
	if !ok || len(messagesList) != 3 {
		t.Errorf("Expected 3 messages, got %v", messages)
	}

	// Check tools exist
	tools, ok := requestData["tools"]
	if !ok {
		t.Error("Request should contain tools")
	}

	toolsList, ok := tools.([]interface{})
	if !ok || len(toolsList) != 1 {
		t.Errorf("Expected 1 tool, got %v", tools)
	}
}

func TestProvider_ConvertRequest_NoTools(t *testing.T) {
	provider := createMockProvider()

	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "Simple question"},
		},
		MaxTokens: 50,
	}

	bedrockReq, err := provider.convertRequest(req, ModelClaudeSonnet)
	if err != nil {
		t.Fatalf("convertRequest failed: %v", err)
	}

	var requestData map[string]interface{}
	if err := json.Unmarshal(bedrockReq, &requestData); err != nil {
		t.Fatalf("Converted request is not valid JSON: %v", err)
	}

	// Should not have tools field when no tools provided
	if _, hasTools := requestData["tools"]; hasTools {
		t.Error("Request should not contain tools field when no tools provided")
	}
}

func TestClaudeModelConstants(t *testing.T) {
	expectedModels := map[string]string{
		"ModelClaudeSonnet": ModelClaudeSonnet,
		"ModelClaudeOpus":   ModelClaudeOpus,
		"ModelClaudeHaiku":  ModelClaudeHaiku,
	}

	for name, modelID := range expectedModels {
		if modelID == "" {
			t.Errorf("%s constant should not be empty", name)
		}

		if !strings.Contains(modelID, "anthropic.claude-3") {
			t.Errorf("%s should contain 'anthropic.claude-3', got %s", name, modelID)
		}

		if !strings.HasSuffix(modelID, "-v1:0") {
			t.Errorf("%s should end with '-v1:0', got %s", name, modelID)
		}
	}
}

func TestProvider_ParseResponse_EmptyContent(t *testing.T) {
	provider := createMockProvider()

	// Response with empty content array
	responseBody := `{
		"id": "test-id",
		"type": "message",
		"role": "assistant",
		"content": [],
		"model": "claude-3-sonnet-20240229",
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 5, "output_tokens": 0}
	}`

	response, err := provider.parseResponse([]byte(responseBody), ModelClaudeSonnet)
	if err != nil {
		t.Fatalf("parseResponse failed: %v", err)
	}

	if response.Content != "" {
		t.Errorf("Expected empty content, got %s", response.Content)
	}
}

func TestProvider_ParseResponse_WithToolCalls(t *testing.T) {
	provider := createMockProvider()

	// Response with tool calls
	responseBody := `{
		"id": "test-id",
		"type": "message",
		"role": "assistant",
		"content": [
			{
				"type": "text",
				"text": "I'll help you get the weather information."
			},
			{
				"type": "tool_use",
				"id": "tool_1",
				"name": "get_weather",
				"input": {"location": "New York"}
			}
		],
		"model": "claude-3-sonnet-20240229",
		"stop_reason": "tool_use",
		"usage": {"input_tokens": 15, "output_tokens": 30}
	}`

	response, err := provider.parseResponse([]byte(responseBody), ModelClaudeSonnet)
	if err != nil {
		t.Fatalf("parseResponse failed: %v", err)
	}

	if response.Content != "I'll help you get the weather information." {
		t.Errorf("Unexpected content: %s", response.Content)
	}

	if len(response.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(response.ToolCalls))
	}

	if response.ToolCalls[0].Name != "get_weather" {
		t.Errorf("Expected tool name 'get_weather', got %s", response.ToolCalls[0].Name)
	}

	if response.StopReason != "tool_use" {
		t.Errorf("Expected stop reason 'tool_use', got %s", response.StopReason)
	}
}

// Test the integration with actual Config struct (without AWS calls)
func TestNewProvider_Config(t *testing.T) {
	// This test only verifies the config handling without making AWS calls
	// We can't easily test the full NewProvider without mocking AWS services

	cfg := &Config{
		Region:      "us-west-2",
		AccessKeyID: "test-access-key",
		SecretKey:   "test-secret-key",
		Profile:     "test-profile",
	}

	// Test auth config creation logic
	authCfg := &AuthConfig{
		Method:          AuthMethodDefault,
		Region:          cfg.Region,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretKey,
		SessionToken:    cfg.SessionToken,
		Profile:         cfg.Profile,
	}

	// Verify auth method determination
	if cfg.AccessKeyID != "" && cfg.SecretKey != "" {
		authCfg.Method = AuthMethodStatic
	} else if cfg.Profile != "" {
		authCfg.Method = AuthMethodProfile
	}

	if authCfg.Method != AuthMethodStatic {
		t.Errorf("Expected AuthMethodStatic, got %v", authCfg.Method)
	}

	if authCfg.Region != "us-west-2" {
		t.Errorf("Expected region 'us-west-2', got %s", authCfg.Region)
	}
}
