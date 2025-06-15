package session

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"strings"
	"time"
)

// ToolCompletionNotifier is a function type for notifying about tool completion
type ToolCompletionNotifier func(toolName string, args map[string]interface{}, result string, err error)

// executeToolCallWithNotification executes a tool call and notifies about completion
func (s *Session) executeToolCallWithNotification(ctx context.Context, toolCall *llm.ToolCall, notifier ToolCompletionNotifier) error {
	if s.toolExecutor == nil {
		return fmt.Errorf("tool executor not available")
	}

	loggy.Debug("executeToolCallWithNotification", "tool_name", toolCall.Name, "input", toolCall.Input, "id", toolCall.ID, "type", toolCall.Type)

	// Check permissions before executing the tool
	if s.permissionManager != nil {
		if !s.permissionManager.CheckPermission(toolCall) {
			// Permission denied - log and return error
			loggy.Warn("Tool execution denied by permission system", "tool_name", toolCall.Name, "risk", s.permissionManager.GetToolRisk(toolCall))

			permissionErr := fmt.Errorf("permission denied for %s tool", toolCall.Name)
			errorResultMsg := s.buildToolResultMessage(toolCall, "", permissionErr)
			s.History = append(s.History, errorResultMsg)

			// Notify UI about error if notifier is provided
			if notifier != nil {
				notifier(toolCall.Name, toolCall.Input, "", permissionErr)
			}

			return permissionErr
		}

		loggy.Debug("Tool execution permitted", "tool_name", toolCall.Name, "risk", s.permissionManager.GetToolRisk(toolCall))
	} else {
		loggy.Warn("Permission manager not available, executing tool without permission check", "tool_name", toolCall.Name)
	}

	result, err := s.toolExecutor.ExecuteTool(ctx, toolCall)
	if err != nil {
		loggy.Error("executeToolCallWithNotification failed", "tool_name", toolCall.Name, "error", err, "input", toolCall.Input)

		errorResultMsg := s.buildToolResultMessage(toolCall, "", err)
		s.History = append(s.History, errorResultMsg)

		// Notify UI about error if notifier is provided
		if notifier != nil {
			notifier(toolCall.Name, toolCall.Input, "", err)
		}

		return err
	}

	loggy.Debug("executeToolCallWithNotification success", "tool_name", toolCall.Name, "result_length", len(result))

	toolResultMsg := s.buildToolResultMessage(toolCall, result, nil)
	s.History = append(s.History, toolResultMsg)

	// Notify UI about successful completion if notifier is provided
	if notifier != nil {
		notifier(toolCall.Name, toolCall.Input, result, nil)
	}

	// Log the tool execution and result to help debug tool flow issues
	loggy.Info("Tool execution completed",
		"tool_name", toolCall.Name,
		"result_length", len(result),
		"adding_to_history", "true")

	// Update session timestamp
	s.UpdatedAt = time.Now()

	return nil
}

func (s *Session) buildToolResultMessage(toolCall *llm.ToolCall, result string, err error) llm.Message {
	var content string
	if err != nil {
		content = fmt.Sprintf("<tool_result tool=\"%s\" tool_id=\"%s\" error=\"true\">\nError: %s\n</tool_result>",
			toolCall.Name, toolCall.ID, err.Error())
	} else {
		content = fmt.Sprintf("<tool_result tool=\"%s\" tool_id=\"%s\">\n%s\n</tool_result>",
			toolCall.Name, toolCall.ID, result)
	}

	return llm.Message{
		Role:    "user", // Use user role for tool results to maintain alternation
		Content: content,
	}
}

// ExecuteToolCall executes a tool call from the AI (legacy method for compatibility)
func (s *Session) ExecuteToolCall(ctx context.Context, toolCall *llm.ToolCall) error {
	if s.toolExecutor == nil {
		return fmt.Errorf("tool executor not available")
	}

	loggy.Debug("ExecuteToolCall", "tool_name", toolCall.Name, "input", toolCall.Input, "id", toolCall.ID, "type", toolCall.Type)

	// Check permissions before executing the tool
	if s.permissionManager != nil {
		if !s.permissionManager.CheckPermission(toolCall) {
			// Permission denied - log and return error
			loggy.Warn("Tool execution denied by permission system", "tool_name", toolCall.Name, "risk", s.permissionManager.GetToolRisk(toolCall))
			return fmt.Errorf("permission denied for %s tool", toolCall.Name)
		}

		loggy.Debug("Tool execution permitted", "tool_name", toolCall.Name, "risk", s.permissionManager.GetToolRisk(toolCall))
	} else {
		loggy.Warn("Permission manager not available, executing tool without permission check", "tool_name", toolCall.Name)
	}

	result, err := s.toolExecutor.ExecuteTool(ctx, toolCall)
	if err != nil {
		loggy.Error("ExecuteToolCall failed", "tool_name", toolCall.Name, "error", err, "input", toolCall.Input)
		return err
	}

	loggy.Debug("ExecuteToolCall success", "tool_name", toolCall.Name, "result_length", len(result))

	// Add tool result to conversation history so AI can see it
	// Use 'system' role since 'tool' role isn't supported by all providers (especially Bedrock)
	toolResultMsg := llm.Message{
		Role:    "system", // Using system instead of tool for compatibility
		Content: fmt.Sprintf("Tool results from '%s':\n%s", toolCall.Name, result),
	}

	// Log the tool execution and result to help debug tool flow issues
	loggy.Info("Tool execution completed",
		"tool_name", toolCall.Name,
		"result_length", len(result),
		"adding_to_history", "true")
	s.History = append(s.History, toolResultMsg)

	// Update session timestamp
	s.UpdatedAt = time.Now()

	// Note: Follow-up request will be handled by the caller
	// Don't trigger individual follow-ups for each tool call

	return nil
}

// getLastUserMessage finds the most recent user message in history
func (s *Session) getLastUserMessage() string {
	// Search backwards through history to find the last user message
	for i := len(s.History) - 1; i >= 0; i-- {
		if s.History[i].Role == "user" {
			if content, ok := s.History[i].Content.(string); ok {
				return content
			}
		}
	}
	return "analyze the information provided"
}

// countRecentToolCalls counts tool calls in recent conversation to prevent infinite loops
func (s *Session) countRecentToolCalls() int {
	count := 0
	// Count tool calls in the last 10 messages
	start := len(s.History) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(s.History); i++ {
		msg := s.History[i]
		// Check if message content contains tool use blocks
		if msg.Role == "assistant" {
			if contentBlocks, ok := msg.Content.([]llm.ContentBlock); ok {
				for _, block := range contentBlocks {
					if block.Type == "tool_use" {
						count++
					}
				}
			}
		}
	}
	return count
}

// sendStreamingFollowUpRequest re-invokes the LLM with streaming for UI updates
func (s *Session) sendStreamingFollowUpRequest(ctx context.Context, uiChan chan<- *llm.StreamChunk) error {
	// Count current tool invocation depth to prevent infinite loops
	toolDepth := s.countRecentToolCalls()
	maxToolDepth := 10 // Configurable limit
	loggy.Info("Sending streaming follow-up request to LLM", "provider", s.Provider, "model", s.Model)

	// Find the original user request to provide proper context for follow-up
	originalRequest := s.getLastUserMessage()

	// Create a follow-up instruction that reminds the AI what to do with tool results
	// For code review requests, be more explicit about reading multiple files
	var followUpInstruction string
	if isCodeReviewRequest(originalRequest) {
		followUpInstruction = fmt.Sprintf("Continue reading relevant files to complete the comprehensive code review requested: %s. Read additional files as needed to provide thorough analysis of the codebase structure, patterns, and implementation details.", originalRequest)
	} else {
		followUpInstruction = fmt.Sprintf("Based on the tool results above, please complete the user's request: %s", originalRequest)
	}

	loggy.Debug("Follow-up instruction", "original_request", originalRequest, "instruction", followUpInstruction)

	// Use intelligent context management for the follow-up request
	messages, err := s.contextManager.BuildOptimizedContext(s, s.History, followUpInstruction)
	if err != nil {
		return fmt.Errorf("failed to build context for follow-up: %w", err)
	}

	// Create LLM request with updated conversation history
	// Re-enable tools for follow-up requests with depth control
	var tools []llm.Tool
	if toolDepth < maxToolDepth {
		tools = s.toolExecutor.GetAvailableTools()
		loggy.Debug("Tools enabled for follow-up", "tool_depth", toolDepth, "max_depth", maxToolDepth)
	} else {
		tools = []llm.Tool{} // Disable tools if we've reached max depth
		loggy.Warn("Tools disabled due to depth limit", "tool_depth", toolDepth, "max_depth", maxToolDepth)
	}

	req := &llm.GenerateRequest{
		Messages:    messages,
		Model:       s.Model,
		MaxTokens:   s.config.LLM.MaxTokens,
		Temperature: s.config.LLM.Temperature,
		Tools:       tools,
	}

	loggy.Debug("Streaming follow-up request prepared", "model", s.Model, "message_count", len(messages))

	// Get the provider
	provider, err := s.llmManager.GetProvider(s.Provider)
	if err != nil {
		return fmt.Errorf("failed to get provider for follow-up: %w", err)
	}

	if provider == nil {
		return fmt.Errorf("provider is nil, cannot proceed with follow-up")
	}

	// Stream the response to UI channel
	reinvokeProviderChan, err := provider.StreamResponse(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to generate streaming follow-up response: %w", err)
	}

	if reinvokeProviderChan == nil {
		return fmt.Errorf("provider returned nil channel for follow-up response")
	}

	// Process the stream and collect tool calls
	var reinvokeResponse strings.Builder
	var toolCalls []llm.ToolCall
	var pendingToolCalls map[string]*llm.ToolCall
	var toolInputBuffers map[string]string
	chunkCount := 0

	pendingToolCalls = make(map[string]*llm.ToolCall)
	toolInputBuffers = make(map[string]string)

	loggy.Info("Starting to process follow-up stream", "uiChan_nil", uiChan == nil)

	for chunk := range reinvokeProviderChan {
		chunkCount++
		loggy.Debug("Received follow-up chunk from provider", "chunk_count", chunkCount, "content_length", len(chunk.Content), "chunk_type", chunk.Type)

		// Send chunk to UI if a channel is provided
		if uiChan != nil {
			select {
			case uiChan <- chunk:
			default:
				loggy.Error("UI channel blocked when sending follow-up", "chunk_count", chunkCount, "content_length", len(chunk.Content))
			}
		} else {
			loggy.Error("UI channel is nil, cannot send follow-up chunk", "chunk_count", chunkCount)
		}

		// Collect for history
		if chunk.Content != "" {
			reinvokeResponse.WriteString(chunk.Content)
		}

		// Handle tool calls - support streaming tool input accumulation
		if chunk.ToolCall != nil {
			toolCall := *chunk.ToolCall
			if toolCall.ID != "" {
				// Store or update the tool call
				pendingToolCalls[toolCall.ID] = &toolCall
				loggy.Debug("Follow-up tool call started", "tool_name", toolCall.Name, "tool_id", toolCall.ID)
			}
		}

		// Handle tool input deltas (for streaming providers like Bedrock)
		if chunk.ToolInputDelta != "" && len(pendingToolCalls) > 0 {
			// Find the most recent tool call to append input to
			var latestToolID string
			for id := range pendingToolCalls {
				latestToolID = id // In practice, there's usually only one active tool call
			}

			if latestToolID != "" {
				toolInputBuffers[latestToolID] += chunk.ToolInputDelta
				loggy.Debug("Follow-up tool input delta", "tool_id", latestToolID, "partial_json", chunk.ToolInputDelta)
			}
		}

		// Handle end of content blocks - finalize tool calls
		if chunk.Type == "content_block_stop" && len(pendingToolCalls) > 0 {
			for toolID, toolCall := range pendingToolCalls {
				// Parse accumulated JSON input if we have it
				if jsonInput, exists := toolInputBuffers[toolID]; exists && jsonInput != "" {
					var inputMap map[string]interface{}
					if err := json.Unmarshal([]byte(jsonInput), &inputMap); err == nil {
						toolCall.Input = inputMap
						loggy.Debug("Follow-up tool input parsed", "tool_id", toolID, "input", inputMap)
					} else {
						loggy.Warn("Follow-up failed to parse tool input JSON", "tool_id", toolID, "json", jsonInput, "error", err)
						// Ensure Input is not nil
						if toolCall.Input == nil {
							toolCall.Input = make(map[string]interface{})
						}
					}
				} else {
					// Ensure Input is not nil even if no JSON was accumulated
					if toolCall.Input == nil {
						toolCall.Input = make(map[string]interface{})
					}
				}

				// Add to final tool calls
				toolCalls = append(toolCalls, *toolCall)
				loggy.Debug("Follow-up tool call finalized", "tool_id", toolID, "name", toolCall.Name, "input", toolCall.Input)
			}

			// Clear pending tool calls after processing
			pendingToolCalls = make(map[string]*llm.ToolCall)
			toolInputBuffers = make(map[string]string)
		}
	}

	loggy.Info("Finished processing follow-up stream", "total_chunks", chunkCount, "response_length", reinvokeResponse.Len(), "tool_calls", len(toolCalls))

	// Execute tool calls if any (just like in ProcessMessageStream)
	for _, toolCall := range toolCalls {
		loggy.Debug("Executing follow-up tool call", "tool_name", toolCall.Name, "tool_input", toolCall.Input, "tool_id", toolCall.ID)

		// Validate tool call before execution
		if toolCall.Name == "" {
			loggy.Error("Invalid follow-up tool call", "empty tool name", "tool_call", toolCall)
			continue
		}

		// Create a notifier that sends tool completion to UI channel
		notifier := func(toolName string, args map[string]interface{}, result string, err error) {
			// Create a special chunk for tool completion notification
			var completionChunk *llm.StreamChunk
			if err != nil {
				completionChunk = &llm.StreamChunk{
					Type:    "tool_completion",
					Content: "",
					ToolCompletion: &llm.ToolCompletion{
						ToolName: toolName,
						Args:     args,
						Result:   result,
						Error:    err.Error(),
						State:    "error",
					},
				}
			} else {
				completionChunk = &llm.StreamChunk{
					Type:    "tool_completion",
					Content: "",
					ToolCompletion: &llm.ToolCompletion{
						ToolName: toolName,
						Args:     args,
						Result:   result,
						Error:    "",
						State:    "complete",
					},
				}
			}

			// Send completion chunk to UI
			if uiChan != nil {
				select {
				case uiChan <- completionChunk:
					loggy.Debug("Sent follow-up tool completion to UI", "tool_name", toolName, "state", completionChunk.ToolCompletion.State)
				default:
					loggy.Warn("UI channel blocked for follow-up tool completion", "tool_name", toolName)
				}
			}
		}

		// Send tool start notification before execution
		startChunk := &llm.StreamChunk{
			Type:    "tool_start",
			Content: "",
			ToolCompletion: &llm.ToolCompletion{
				ToolName: toolCall.Name,
				Args:     toolCall.Input,
				Result:   "",
				Error:    "",
				State:    "start",
			},
		}

		if uiChan != nil {
			select {
			case uiChan <- startChunk:
				loggy.Debug("Sent follow-up tool start to UI", "tool_name", toolCall.Name, "args", toolCall.Input)
			default:
				loggy.Warn("UI channel blocked for follow-up tool start", "tool_name", toolCall.Name)
			}
		}

		// Execute tool with notification
		if err := s.executeToolCallWithNotification(ctx, &toolCall, notifier); err != nil {
			loggy.Error("Follow-up tool execution failed", "tool_name", toolCall.Name, "tool_input", toolCall.Input, "error", err)
		}
	}

	// Recursively handle follow-up requests if new tool calls were made
	if len(toolCalls) > 0 {
		loggy.Info("Follow-up request generated tool calls, sending recursive follow-up", "tool_count", len(toolCalls))

		// Check recursion depth to prevent infinite loops
		currentDepth := s.countRecentToolCalls()
		if currentDepth < 10 { // Same limit as before
			if err := s.sendStreamingFollowUpRequest(ctx, uiChan); err != nil {
				loggy.Error("Recursive follow-up request failed", "error", err)
			}
		} else {
			loggy.Warn("Maximum follow-up recursion depth reached, stopping", "depth", currentDepth)
		}
		return nil // Don't add this response to history since we're doing another follow-up
	}

	// Handle empty follow-up response - send fallback
	if chunkCount == 0 || reinvokeResponse.Len() == 0 {
		loggy.Warn("Follow-up response was empty, sending fallback message to UI")

		fallbackContent := "I executed the tool successfully, but encountered an issue generating a follow-up response. The tool operation completed as requested."

		// Send fallback chunk to UI
		if uiChan != nil {
			fallbackChunk := &llm.StreamChunk{
				Type:    "content_block_delta",
				Content: fallbackContent,
			}

			select {
			case uiChan <- fallbackChunk:
				loggy.Info("Sent fallback follow-up chunk to UI", "content", fallbackContent)
			default:
				loggy.Error("UI channel blocked when sending fallback follow-up")
			}
		}

		// Add fallback to history
		fallbackMsg := llm.Message{
			Role:    "assistant",
			Content: fallbackContent,
		}
		s.History = append(s.History, fallbackMsg)
		loggy.Info("Added fallback follow-up response to history", "length", len(fallbackContent))

	} else {
		// Add the successful response to conversation history
		followUpMsg := llm.Message{
			Role:    "assistant",
			Content: reinvokeResponse.String(),
		}
		s.History = append(s.History, followUpMsg)
		loggy.Info("Added follow-up response to history", "length", reinvokeResponse.Len())
	}

	return nil
}

// isCodeReviewRequest checks if the user request is asking for code review/analysis
func isCodeReviewRequest(request string) bool {
	request = strings.ToLower(request)

	// Common code review/analysis keywords
	reviewKeywords := []string{
		"review", "analyze", "examine", "assess", "evaluate", "inspect", "check",
		"code review", "code analysis", "codebase", "project structure",
		"architecture", "implementation", "patterns", "quality",
		"overview", "summary", "understanding", "explain the code",
		"how does", "what does", "structure of", "organization of",
	}

	for _, keyword := range reviewKeywords {
		if strings.Contains(request, keyword) {
			return true
		}
	}

	return false
}
