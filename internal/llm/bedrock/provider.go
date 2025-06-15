package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// Provider implements the LLM Provider interface for AWS Bedrock
type Provider struct {
	client       *bedrockruntime.Client
	region       string
	defaultModel string
	models       map[string]llm.Model
}

// Config represents Bedrock-specific configuration
type Config struct {
	Region       string `yaml:"region"`
	AccessKeyID  string `yaml:"access_key_id"`
	SecretKey    string `yaml:"secret_access_key"`
	SessionToken string `yaml:"session_token"`
	Profile      string `yaml:"profile"`
	AuthMethod   string `yaml:"auth_method"`
}

// Claude model IDs for Bedrock
const (
	ModelClaudeSonnet = "anthropic.claude-3-sonnet-20240229-v1:0"
	ModelClaudeOpus   = "anthropic.claude-3-opus-20240229-v1:0"
	ModelClaudeHaiku  = "anthropic.claude-3-haiku-20240307-v1:0"
)

// NewProvider creates a new Bedrock provider
func NewProvider(cfg *Config) (*Provider, error) {
	// Create auth config
	authCfg := &AuthConfig{
		Method:          AuthMethodDefault,
		Region:          cfg.Region,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretKey,
		SessionToken:    cfg.SessionToken,
		Profile:         cfg.Profile,
	}

	// Determine auth method based on config
	if cfg.AuthMethod != "" {
		// Use explicitly configured auth method
		switch cfg.AuthMethod {
		case "static":
			authCfg.Method = AuthMethodStatic
		case "profile":
			authCfg.Method = AuthMethodProfile
		case "assume_role":
			authCfg.Method = AuthMethodRole
		case "environment":
			authCfg.Method = AuthMethodEnvironment
		case "default":
			authCfg.Method = AuthMethodDefault
		}
	} else {
		// Legacy logic: determine auth method based on config values
		if cfg.AccessKeyID != "" && cfg.SecretKey != "" {
			authCfg.Method = AuthMethodStatic
		} else if cfg.Profile != "" {
			authCfg.Method = AuthMethodProfile
		}
	}

	// Load AWS config with authentication
	awsCfg, err := LoadAWSConfig(context.TODO(), authCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Validate credentials
	if err := ValidateCredentials(context.TODO(), awsCfg); err != nil {
		return nil, fmt.Errorf("credential validation failed: %w", err)
	}

	// Create Bedrock Runtime client
	client := bedrockruntime.NewFromConfig(awsCfg)

	// Define available models
	models := map[string]llm.Model{
		ModelClaudeSonnet: {
			ID:              ModelClaudeSonnet,
			Name:            "Claude 3 Sonnet",
			Provider:        "bedrock",
			MaxTokens:       200000,
			SupportsTools:   true,
			CostPer1KTokens: 0.003, // Approximate pricing
		},
		ModelClaudeOpus: {
			ID:              ModelClaudeOpus,
			Name:            "Claude 3 Opus",
			Provider:        "bedrock",
			MaxTokens:       200000,
			SupportsTools:   true,
			CostPer1KTokens: 0.015, // Approximate pricing
		},
		ModelClaudeHaiku: {
			ID:              ModelClaudeHaiku,
			Name:            "Claude 3 Haiku",
			Provider:        "bedrock",
			MaxTokens:       200000,
			SupportsTools:   true,
			CostPer1KTokens: 0.00025, // Approximate pricing
		},
	}

	return &Provider{
		client:       client,
		region:       cfg.Region,
		defaultModel: ModelClaudeSonnet, // Default to Sonnet
		models:       models,
	}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "bedrock"
}

// GenerateResponse generates a response from Claude via Bedrock
func (p *Provider) GenerateResponse(ctx context.Context, req *llm.GenerateRequest) (*llm.Response, error) {
	start := time.Now()

	// Use default model if not specified
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	// Convert request to Bedrock format
	bedrockReq, err := p.convertRequest(req, model)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	// Make the API call
	resp, err := p.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(model),
		ContentType: aws.String("application/json"),
		Body:        bedrockReq,
	})
	if err != nil {
		return nil, fmt.Errorf("bedrock invoke model failed: %w", err)
	}

	// Parse response
	response, err := p.parseResponse(resp.Body, model)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	response.ProcessingTimeMs = time.Since(start).Milliseconds()
	response.CreatedAt = time.Now()

	return response, nil
}

// StreamResponse streams a response from Claude via Bedrock
func (p *Provider) StreamResponse(ctx context.Context, req *llm.GenerateRequest) (<-chan *llm.StreamChunk, error) {
	loggy.Debug("Bedrock StreamResponse", "starting", "true")

	// Use default model if not specified
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	// Log detailed message roles to debug message alternation issues
	var roles []string
	for _, msg := range req.Messages {
		roles = append(roles, msg.Role)
	}

	loggy.Debug("Bedrock StreamResponse",
		"model", model,
		"messages_count", len(req.Messages),
		"message_roles", strings.Join(roles, ","),
		"has_system_message", len(roles) > 0 && roles[0] == "system",
		"tools_count", len(req.Tools))

	// Convert request to Bedrock format
	bedrockReq, err := p.convertRequest(req, model)
	if err != nil {
		loggy.Error("Bedrock StreamResponse", "convert_request_failed", err)
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	loggy.Debug("Bedrock StreamResponse", "invoking_model_stream", "true")

	// Make streaming API call
	resp, err := p.client.InvokeModelWithResponseStream(ctx, &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(model),
		ContentType: aws.String("application/json"),
		Body:        bedrockReq,
	})
	if err != nil {
		loggy.Error("Bedrock StreamResponse", "invoke_model_stream_failed", err)
		return nil, fmt.Errorf("bedrock invoke model stream failed: %w", err)
	}

	loggy.Debug("Bedrock StreamResponse", "api_call_successful", "true", "creating_channel", "true")

	// Create channel for streaming chunks
	chunks := make(chan *llm.StreamChunk, 10)

	// Process stream in goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				loggy.Error("Bedrock StreamResponse", "panic_in_stream_processing", fmt.Errorf("panic: %v", r))
			}
			loggy.Debug("Bedrock StreamResponse", "closing_channel", "true")
			close(chunks)
		}()

		loggy.Debug("Bedrock StreamResponse", "starting_event_loop", "true")

		eventCount := 0
		stream := resp.GetStream()

		// Check if stream is nil
		if stream == nil {
			loggy.Error("Bedrock StreamResponse", "nil_stream", fmt.Errorf("response stream is nil"))
			chunks <- &llm.StreamChunk{
				Type:    "error",
				Content: "Response stream is nil",
			}
			return
		}

		// Ensure stream is closed when we're done
		defer func() {
			if err := stream.Close(); err != nil {
				loggy.Error("Bedrock StreamResponse", "stream_close_error", err)
			}
		}()

		// Process events
		eventChan := stream.Events()
		if eventChan == nil {
			loggy.Error("Bedrock StreamResponse", "nil_event_channel", fmt.Errorf("event channel is nil"))
			chunks <- &llm.StreamChunk{
				Type:    "error",
				Content: "Event channel is nil",
			}
			return
		}

		loggy.Debug("Bedrock StreamResponse", "about_to_read_events", "true")

		for {
			select {
			case event, ok := <-eventChan:
				if !ok {
					loggy.Debug("Bedrock StreamResponse", "event_channel_closed", "true", "total_events", eventCount)
					return
				}

				eventCount++
				loggy.Debug("Bedrock StreamResponse", "received_event", "true", "event_count", eventCount, "event_type", fmt.Sprintf("%T", event))

				// Check for stream errors
				if err := stream.Err(); err != nil {
					loggy.Error("Bedrock StreamResponse", "stream_error", err)
					chunks <- &llm.StreamChunk{
						Type:    "error",
						Content: fmt.Sprintf("Stream error: %v", err),
					}
					return
				}

				switch v := event.(type) {
				case *types.ResponseStreamMemberChunk:
					loggy.Debug("Bedrock StreamResponse", "processing_chunk", "true", "bytes_length", len(v.Value.Bytes))
					chunk, err := p.parseStreamChunk(v.Value.Bytes)
					if err != nil {
						loggy.Error("Bedrock StreamResponse", "parse_chunk_failed", err)
						// Send error chunk
						chunks <- &llm.StreamChunk{
							Type:    "error",
							Content: fmt.Sprintf("Parse error: %v", err),
						}
						return
					}
					if chunk != nil {
						loggy.Debug("Bedrock StreamResponse", "sending_chunk", "true", "chunk_type", chunk.Type, "chunk_content", chunk.Content)
						chunks <- chunk
					}
				case *types.UnknownUnionMember:
					loggy.Error("Bedrock StreamResponse", "unknown_union_member", fmt.Errorf("unknown event type: %s", v.Tag))
					chunks <- &llm.StreamChunk{
						Type:    "error",
						Content: fmt.Sprintf("Unknown event type: %s", v.Tag),
					}
					return
				default:
					loggy.Error("Bedrock StreamResponse", "unexpected_event_type", fmt.Errorf("unexpected event type: %T", v))
					chunks <- &llm.StreamChunk{
						Type:    "error",
						Content: fmt.Sprintf("Unexpected event type: %T", v),
					}
					return
				}
			case <-ctx.Done():
				loggy.Debug("Bedrock StreamResponse", "context_canceled", "true")
				return
			}
		}
	}()

	loggy.Debug("Bedrock StreamResponse", "returning_channel", "true")
	return chunks, nil
}

// SupportsFunctionCalling returns true as Claude supports function calling
func (p *Provider) SupportsFunctionCalling() bool {
	return true
}

// GetAvailableModels returns available Claude models
func (p *Provider) GetAvailableModels() []llm.Model {
	models := make([]llm.Model, 0, len(p.models))
	for _, model := range p.models {
		models = append(models, model)
	}
	return models
}

// GetDefaultModel returns the default model
func (p *Provider) GetDefaultModel() string {
	return p.defaultModel
}

// EstimateTokens provides a rough token estimate
func (p *Provider) EstimateTokens(text string) int {
	// Rough approximation: ~4 characters per token for English text
	return len(text) / 4
}

// GetTokenLimit returns the token limit for the current model
func (p *Provider) GetTokenLimit() int {
	if model, exists := p.models[p.defaultModel]; exists {
		return model.MaxTokens
	}
	return 200000 // Default Claude 3 limit
}

// Close cleans up resources
func (p *Provider) Close() error {
	// Bedrock client doesn't require explicit cleanup
	return nil
}

// convertRequest converts generic LLM request to Bedrock's Claude API format
func (p *Provider) convertRequest(req *llm.GenerateRequest, modelID string) ([]byte, error) {
	bedrockReq := map[string]interface{}{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        req.MaxTokens,
	}

	if req.MaxTokens == 0 {
		bedrockReq["max_tokens"] = 4096 // Default
	}

	if req.Temperature > 0 {
		bedrockReq["temperature"] = req.Temperature
	}

	// Convert messages
	messages := make([]map[string]interface{}, 0, len(req.Messages))
	var systemMessage string
	var userContent string

	// First pass: collect all messages and handle system messages
	var normalizedMessages []map[string]interface{}
	var lastAddedRole string

	for _, msg := range req.Messages {
		// Handle system messages - store separately for Claude's system parameter
		if msg.Role == "system" {
			if content, ok := msg.Content.(string); ok {
				// If we already have a system message, append to it
				if systemMessage != "" {
					systemMessage += "\n\n" + content
				} else {
					systemMessage = content
				}
			}
			continue
		}

		// Skip messages with empty roles
		if msg.Role == "" {
			// If there's content but no role, use it as generic user content
			if str, ok := msg.Content.(string); ok && str != "" {
				userContent = str
			}
			continue
		}

		// Create Claude message
		claudeMsg := map[string]interface{}{
			"role":    msg.Role,
			"content": p.convertContent(msg.Content),
		}

		// Handle tool messages - convert to assistant to maintain alternation
		if msg.Role == "tool" {
			// Convert tool messages to assistant messages to maintain alternation
			claudeMsg["role"] = "assistant"
		}

		normalizedMessages = append(normalizedMessages, claudeMsg)
	}

	// Second pass: ensure proper alternation of user/assistant roles
	if len(normalizedMessages) > 0 {
		lastAddedRole = ""
		loggy.Debug("Bedrock message processing", "total_normalized_messages", len(normalizedMessages))

		for i, msg := range normalizedMessages {
			role, _ := msg["role"].(string)

			// First message must be from user
			if i == 0 && role != "user" {
				// If first message isn't from user, skip it
				loggy.Debug("Bedrock message processing", "skipping_non_user_first_message", role)
				continue
			}

			// Enforce alternation - only add if this message's role differs from the last added
			if lastAddedRole == "" || role != lastAddedRole {
				messages = append(messages, msg)
				lastAddedRole = role
				loggy.Debug("Bedrock message processing", "added_message", role)
			} else {
				// If same role as previous, combine content if possible
				if len(messages) > 0 {
					lastMsg := messages[len(messages)-1]
					if content, ok := lastMsg["content"].(string); ok {
						if newContent, ok := msg["content"].(string); ok {
							// Combine text content
							lastMsg["content"] = content + "\n\n" + newContent
							loggy.Debug("Bedrock message processing", "combined_consecutive_messages", role)
						}
					}
				}
			}
		}

		loggy.Debug("Bedrock message processing", "final_messages_count", len(messages),
			"starts_with_user", len(messages) > 0 && func() bool { role, _ := messages[0]["role"].(string); return role == "user" }(),
			"proper_alternation", "enforced")
	}

	// If no valid messages were found but we have user content (e.g., from string prompt)
	// create a default user message
	if len(messages) == 0 && userContent != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": userContent,
		})
	}

	// If still no messages, create a minimal default message to satisfy API requirements
	if len(messages) == 0 {
		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": "Please assist me.",
		})
	}

	bedrockReq["messages"] = messages

	if systemMessage != "" {
		bedrockReq["system"] = systemMessage
	}

	// Add tools if present
	if len(req.Tools) > 0 {
		bedrockReq["tools"] = p.convertTools(req.Tools)
	}

	return json.Marshal(bedrockReq)
}

// convertContent converts message content to Claude format
func (p *Provider) convertContent(content interface{}) interface{} {
	switch v := content.(type) {
	case string:
		return v
	case []llm.ContentBlock:
		claudeContent := make([]map[string]interface{}, len(v))
		for i, block := range v {
			claudeContent[i] = p.convertContentBlock(block)
		}
		return claudeContent
	default:
		return fmt.Sprintf("%v", content)
	}
}

// convertContentBlock converts a content block to Claude format
func (p *Provider) convertContentBlock(block llm.ContentBlock) map[string]interface{} {
	result := map[string]interface{}{
		"type": block.Type,
	}

	switch block.Type {
	case "text":
		result["text"] = block.Text
	case "image":
		if block.Source != nil {
			result["source"] = map[string]interface{}{
				"type":       block.Source.Type,
				"media_type": block.Source.MediaType,
				"data":       block.Source.Data,
			}
		}
	case "tool_use":
		if block.ToolUse != nil {
			result["id"] = block.ToolUse.ID
			result["name"] = block.ToolUse.Name
			result["input"] = block.ToolUse.Input
		}
	case "tool_result":
		result["tool_use_id"] = block.Content
		if block.IsError {
			result["is_error"] = true
		}
	}

	return result
}

// convertTools converts tools to Claude format
func (p *Provider) convertTools(tools []llm.Tool) []map[string]interface{} {
	claudeTools := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		claudeTools[i] = map[string]interface{}{
			"name":         tool.Name,
			"description":  tool.Description,
			"input_schema": tool.InputSchema,
		}
	}
	return claudeTools
}

// parseResponse parses Bedrock response to LLM response format
func (p *Provider) parseResponse(body []byte, model string) (*llm.Response, error) {
	var bedrockResp struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Role    string `json:"role"`
		Content []struct {
			Type  string                 `json:"type"`
			Text  string                 `json:"text,omitempty"`
			ID    string                 `json:"id,omitempty"`
			Name  string                 `json:"name,omitempty"`
			Input map[string]interface{} `json:"input,omitempty"`
		} `json:"content"`
		Model        string `json:"model"`
		StopReason   string `json:"stop_reason"`
		StopSequence string `json:"stop_sequence"`
		Usage        struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &bedrockResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	response := &llm.Response{
		ID:           bedrockResp.ID,
		Model:        model,
		StopReason:   bedrockResp.StopReason,
		InputTokens:  bedrockResp.Usage.InputTokens,
		OutputTokens: bedrockResp.Usage.OutputTokens,
	}

	// Extract content and tool calls
	var contentParts []string
	var toolCalls []llm.ToolCall

	for _, content := range bedrockResp.Content {
		switch content.Type {
		case "text":
			contentParts = append(contentParts, content.Text)
		case "tool_use":
			// Ensure input is not nil
			input := content.Input
			if input == nil {
				input = make(map[string]interface{})
			}
			toolCalls = append(toolCalls, llm.ToolCall{
				ID:    content.ID,
				Type:  "function",
				Name:  content.Name,
				Input: input,
			})
		}
	}

	response.Content = strings.Join(contentParts, "\n")
	response.ToolCalls = toolCalls

	return response, nil
}

// parseStreamChunk parses a streaming chunk from Bedrock
func (p *Provider) parseStreamChunk(data []byte) (*llm.StreamChunk, error) {
	var chunkData struct {
		Type  string `json:"type"`
		Index int    `json:"index"`
		Delta struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			PartialJSON string `json:"partial_json"`
		} `json:"delta"`
		ContentBlock struct {
			Type  string                 `json:"type"`
			ID    string                 `json:"id"`
			Name  string                 `json:"name"`
			Input map[string]interface{} `json:"input"`
		} `json:"content_block"`
	}

	if err := json.Unmarshal(data, &chunkData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chunk: %w", err)
	}

	chunk := &llm.StreamChunk{
		Type:  chunkData.Type,
		Index: chunkData.Index,
	}

	switch chunkData.Type {
	case "content_block_start":
		if chunkData.ContentBlock.Type == "tool_use" {
			chunk.ToolCall = &llm.ToolCall{
				ID:    chunkData.ContentBlock.ID,
				Name:  chunkData.ContentBlock.Name,
				Type:  "function",
				Input: chunkData.ContentBlock.Input,
			}
		}
	case "content_block_delta":
		chunk.Delta = &llm.Delta{
			Type: chunkData.Delta.Type,
			Text: chunkData.Delta.Text,
		}
		// Handle text deltas
		if chunkData.Delta.Type == "text_delta" {
			chunk.Content = chunkData.Delta.Text
		}
		// Handle tool input deltas - accumulate JSON input
		if chunkData.Delta.Type == "input_json_delta" {
			chunk.ToolInputDelta = chunkData.Delta.PartialJSON
		}
	case "content_block_stop":
		// End of content block - finalize tool call if needed
		if chunk.ToolCall != nil && chunk.ToolCall.Input == nil {
			chunk.ToolCall.Input = make(map[string]interface{})
		}
	case "message_start", "message_delta", "message_stop":
		// Message-level events
	}

	return chunk, nil
}
