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

// ProcessMessage processes a user message with the AI
func (s *Session) ProcessMessage(ctx context.Context, message string) (*llm.Response, error) {
	// Add user message to history
	userMsg := llm.Message{
		Role:    "user",
		Content: message,
	}
	s.History = append(s.History, userMsg)

	// Auto-save session after adding user message
	if err := s.Save(); err != nil {
		loggy.Warn("Failed to auto-save session after user message", "session_id", s.ID, "error", err)
	}

	// Use intelligent context management
	messages, err := s.contextManager.BuildOptimizedContext(s, s.History, message)
	if err != nil {
		return nil, fmt.Errorf("failed to build context: %w", err)
	}

	// Create LLM request
	req := &llm.GenerateRequest{
		Messages:    messages,
		Model:       s.Model,
		MaxTokens:   s.config.LLM.MaxTokens,
		Temperature: s.config.LLM.Temperature,
		Tools:       s.getAvailableTools(),
	}

	// Generate response
	provider, err := s.llmManager.GetProvider(s.Provider)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	response, err := provider.GenerateResponse(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate response: %w", err)
	}

	// Add assistant response to history
	assistantMsg := llm.Message{
		Role:    "assistant",
		Content: response.Content,
	}
	s.History = append(s.History, assistantMsg)

	s.UpdatedAt = time.Now()
	return response, nil
}

// ProcessMessageStream processes a user message with streaming AI response
func (s *Session) ProcessMessageStream(ctx context.Context, message string) (<-chan *llm.StreamChunk, error) {
	loggy.Debug("Session ProcessMessageStream", "starting", "true", "message", message)

	// Add user message to history
	userMsg := llm.Message{
		Role:    "user",
		Content: message,
	}
	s.History = append(s.History, userMsg)

	// Auto-save session after adding user message
	if err := s.Save(); err != nil {
		loggy.Warn("Failed to auto-save session after user message", "session_id", s.ID, "error", err)
	}

	// Use intelligent context management
	messages, err := s.contextManager.BuildOptimizedContext(s, s.History, message)
	if err != nil {
		loggy.Error("Session ProcessMessageStream", "context_build_failed", err)
		return nil, fmt.Errorf("failed to build context: %w", err)
	}

	loggy.Debug("Session ProcessMessageStream", "context_built", "true", "context_messages_count", len(messages))

	// Create LLM request
	req := &llm.GenerateRequest{
		Messages:    messages,
		Model:       s.Model,
		MaxTokens:   s.config.LLM.MaxTokens,
		Temperature: s.config.LLM.Temperature,
		Tools:       s.getAvailableTools(),
	}

	loggy.Debug("Session ProcessMessageStream", "llm_request_created", "true", "model", s.Model, "provider", s.Provider)

	// Generate streaming response
	providerName := s.Provider

	// Try to get the provider
	provider, err := s.llmManager.GetProvider(providerName)
	if err != nil {
		loggy.Error("Session ProcessMessageStream", "get_provider_failed", err, "provider", providerName)

		// Try to fall back to any available provider
		providers := s.llmManager.ListProviders()
		if len(providers) > 0 {
			loggy.Info("Session ProcessMessageStream", "fallback_provider_attempt", "true", "original", providerName, "fallback", providers[0])
			provider, err = s.llmManager.GetProvider(providers[0])
			if err == nil && provider != nil {
				// Update session provider to the one that worked
				s.Provider = providers[0]
				loggy.Info("Session ProcessMessageStream", "provider_fallback_successful", "true", "new_provider", s.Provider)
			} else {
				return nil, fmt.Errorf("failed to get provider and fallback attempt failed: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to get provider and no fallbacks available: %w", err)
		}
	}

	if provider == nil {
		loggy.Error("Session ProcessMessageStream", "provider_is_nil", fmt.Errorf("provider is nil"), "provider_name", providerName)
		return nil, fmt.Errorf("provider is nil, cannot proceed")
	}

	loggy.Debug("Session ProcessMessageStream", "calling_provider_stream", "true", "provider", s.Provider)

	providerChan, err := provider.StreamResponse(ctx, req)
	if err != nil {
		loggy.Error("Session ProcessMessageStream", "provider_stream_failed", err)
		return nil, fmt.Errorf("failed to generate streaming response: %w", err)
	}

	if providerChan == nil {
		loggy.Error("Session ProcessMessageStream", "provider_returned_nil_channel", fmt.Errorf("provider returned nil channel"))
		return nil, fmt.Errorf("provider returned nil channel")
	}

	loggy.Debug("Session ProcessMessageStream", "provider_stream_successful", "true", "creating_ui_channel", "true")

	// Create a new channel for the UI
	uiChan := make(chan *llm.StreamChunk, 10)

	// Fan out the stream to both UI and session processing
	go func() {
		defer func() {
			loggy.Debug("Session ProcessMessageStream", "fan_out_goroutine_closing", "true")
		}()

		var fullResponse strings.Builder
		var toolCalls []llm.ToolCall
		var pendingToolCalls map[string]*llm.ToolCall // Track incomplete tool calls
		var toolInputBuffers map[string]string        // Buffer for accumulating tool input JSON
		chunkCount := 0
		hasContent := false

		pendingToolCalls = make(map[string]*llm.ToolCall)
		toolInputBuffers = make(map[string]string)

		loggy.Debug("Session ProcessMessageStream", "fan_out_goroutine_starting", "true")

		for chunk := range providerChan {
			chunkCount++
			loggy.Debug("Session ProcessMessageStream", "received_chunk_from_provider", "true", "chunk_count", chunkCount, "chunk_type", chunk.Type, "chunk_content", chunk.Content)

			// Send chunk to UI
			select {
			case uiChan <- chunk:
				loggy.Debug("Session ProcessMessageStream", "sent_chunk_to_ui", "true", "chunk_count", chunkCount)
			default:
				loggy.Warn("Session ProcessMessageStream", "ui_channel_blocked", "chunk_count", chunkCount)
			}

			// Collect for session processing
			if chunk.Content != "" {
				fullResponse.WriteString(chunk.Content)
				hasContent = true
			}

			// Handle tool calls - support streaming tool input accumulation
			if chunk.ToolCall != nil {
				toolCall := *chunk.ToolCall
				if toolCall.ID != "" {
					// Store or update the tool call
					pendingToolCalls[toolCall.ID] = &toolCall
					loggy.Debug("Session ProcessMessageStream", "tool_call_started", toolCall.Name, "tool_id", toolCall.ID)
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
					loggy.Debug("Session ProcessMessageStream", "tool_input_delta", "tool_id", latestToolID, "partial_json", chunk.ToolInputDelta)
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
							loggy.Debug("Session ProcessMessageStream", "tool_input_parsed", "tool_id", toolID, "input", inputMap)
						} else {
							loggy.Error("Session ProcessMessageStream", "tool_input_parse_failed", err, "tool_id", toolID, "json", jsonInput)
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
					loggy.Debug("Session ProcessMessageStream", "tool_call_finalized", "tool_id", toolID, "name", toolCall.Name, "input", toolCall.Input)
				}

				// Clear pending tool calls after processing
				pendingToolCalls = make(map[string]*llm.ToolCall)
				toolInputBuffers = make(map[string]string)
			}
		}

		loggy.Debug("Session ProcessMessageStream", "provider_channel_closed", "true", "total_chunks", chunkCount, "has_content", hasContent)

		// Handle empty stream case - send a fallback response
		if chunkCount == 0 {
			loggy.Warn("Session ProcessMessageStream", "empty_stream_detected", "sending_fallback_response")

			// Send a fallback message to UI
			fallbackChunk := &llm.StreamChunk{
				Type:    "content_block_delta",
				Content: "I received your message but didn't generate a response. Please try again.",
			}

			select {
			case uiChan <- fallbackChunk:
				loggy.Debug("Session ProcessMessageStream", "sent_fallback_to_ui", "true")
			default:
				loggy.Warn("Session ProcessMessageStream", "ui_channel_blocked_for_fallback")
			}

			fullResponse.WriteString(fallbackChunk.Content)
			hasContent = true
		}

		// Add final assistant response to history only if we have content
		if hasContent {
			assistantMsg := llm.Message{
				Role:    "assistant",
				Content: fullResponse.String(),
			}
			s.History = append(s.History, assistantMsg)
		}

		// Check if we should group tools under a task
		var taskGroup string
		if len(toolCalls) > 1 {
			// Generate task name based on tool types
			taskGroup = s.generateTaskName(toolCalls)

			// Send task group header
			taskStartChunk := &llm.StreamChunk{
				Type:    "task_start",
				Content: "",
				ToolCompletion: &llm.ToolCompletion{
					ToolName: "task_group",
					Args:     map[string]interface{}{"task_name": taskGroup},
					Result:   "",
					Error:    "",
					State:    "task_start",
				},
			}
			select {
			case uiChan <- taskStartChunk:
				loggy.Debug("Sent task start to UI", "task_name", taskGroup)
			default:
				loggy.Warn("UI channel blocked for task start", "task_name", taskGroup)
			}
		}

		// Execute tool calls if any
		for _, toolCall := range toolCalls {
			loggy.Debug("Session ProcessMessageStream", "executing_tool", toolCall.Name, "tool_input", toolCall.Input, "tool_id", toolCall.ID, "tool_type", toolCall.Type)

			// Validate tool call before execution
			if toolCall.Name == "" {
				loggy.Error("Session ProcessMessageStream", "invalid_tool_call", "empty tool name", "tool_call", toolCall)
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
							ToolName:  toolName,
							Args:      args,
							Result:    result,
							Error:     err.Error(),
							State:     "error",
							TaskGroup: taskGroup,
						},
					}
				} else {
					completionChunk = &llm.StreamChunk{
						Type:    "tool_completion",
						Content: "",
						ToolCompletion: &llm.ToolCompletion{
							ToolName:  toolName,
							Args:      args,
							Result:    result,
							Error:     "",
							State:     "complete",
							TaskGroup: taskGroup,
						},
					}
				}

				// Send completion chunk to UI
				select {
				case uiChan <- completionChunk:
					loggy.Debug("Sent tool completion to UI", "tool_name", toolName, "state", completionChunk.ToolCompletion.State)
				default:
					loggy.Warn("UI channel blocked for tool completion", "tool_name", toolName)
				}
			}

			// Send tool start notification before execution
			startChunk := &llm.StreamChunk{
				Type:    "tool_start",
				Content: "",
				ToolCompletion: &llm.ToolCompletion{
					ToolName:  toolCall.Name,
					Args:      toolCall.Input,
					Result:    "",
					Error:     "",
					State:     "start",
					TaskGroup: taskGroup,
				},
			}

			select {
			case uiChan <- startChunk:
				loggy.Debug("Sent tool start to UI", "tool_name", toolCall.Name, "args", toolCall.Input)
			default:
				loggy.Warn("UI channel blocked for tool start", "tool_name", toolCall.Name)
			}

			// Execute tool with notification
			if err := s.executeToolCallWithNotification(ctx, &toolCall, notifier); err != nil {
				loggy.Error("Session ProcessMessageStream", "tool_execution_failed", err, "tool_name", toolCall.Name, "tool_input", toolCall.Input)
			}
		}

		// Re-invoke LLM after all tool calls are executed
		if len(toolCalls) > 0 {
			loggy.Info("Session ProcessMessageStream", "reinvoking_llm_after_tool_calls", "true", "tool_count", len(toolCalls))

			// Use the streaming follow-up request method
			loggy.Info("About to send streaming follow-up request", "uiChan_address", fmt.Sprintf("%p", uiChan))
			if err := s.sendStreamingFollowUpRequest(ctx, uiChan); err != nil {
				loggy.Error("Session ProcessMessageStream", "reinvoke_after_tools_failed", err)
			}
			loggy.Info("Completed streaming follow-up request")
		}

		// Close the UI channel after all processing is complete
		loggy.Info("About to close UI channel", "uiChan_address", fmt.Sprintf("%p", uiChan))
		close(uiChan)
		loggy.Info("Closed UI channel")

		s.UpdatedAt = time.Now()
		loggy.Debug("Session ProcessMessageStream", "fan_out_goroutine_complete", "true")
	}()

	loggy.Debug("Session ProcessMessageStream", "returning_ui_channel", "true")
	return uiChan, nil
}

// generateTaskName creates a descriptive task name based on the types of tools being executed
func (s *Session) generateTaskName(toolCalls []llm.ToolCall) string {
	if len(toolCalls) == 0 {
		return "Task"
	}

	// Count tool types
	toolTypes := make(map[string]int)
	for _, tool := range toolCalls {
		switch tool.Name {
		case "read_file":
			toolTypes["read"]++
		case "write_file", "create_file", "edit_file", "multi_edit_file":
			toolTypes["edit"]++
		case "bash":
			toolTypes["run"]++
		case "grep", "find", "fuzzy_search":
			toolTypes["search"]++
		case "git_status", "git_diff", "git_add", "git_commit", "git_log", "git_branch":
			toolTypes["git"]++
		case "todo_read", "todo_write":
			toolTypes["todo"]++
		default:
			toolTypes["other"]++
		}
	}

	// Generate descriptive name based on dominant tool types
	var taskName string

	if toolTypes["search"] > 0 && toolTypes["read"] > 0 {
		taskName = "Find and analyze code"
	} else if toolTypes["edit"] > 0 && toolTypes["run"] > 0 {
		taskName = "Modify and test code"
	} else if toolTypes["git"] > 0 && toolTypes["edit"] > 0 {
		taskName = "Update and commit changes"
	} else if toolTypes["run"] > 0 {
		taskName = "Execute commands"
	} else if toolTypes["edit"] > 0 {
		taskName = "Modify files"
	} else if toolTypes["search"] > 0 {
		taskName = "Search codebase"
	} else if toolTypes["read"] > 0 {
		taskName = "Analyze files"
	} else if toolTypes["git"] > 0 {
		taskName = "Git operations"
	} else {
		taskName = "Multiple operations"
	}

	return taskName
}
