package ui

import (
	"context"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"github.com/tildaslashalef/bazinga/internal/project"
	"github.com/tildaslashalef/bazinga/internal/session"
	"github.com/tildaslashalef/bazinga/internal/ui/commands"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Custom message types for Bubble Tea
type ResponseMsg struct {
	Content          string
	ToolCalls        []llm.ToolCall
	IsSystemMessage  bool
	ShouldStream     bool
	RequiresResponse bool
	OnResponse       func(string) tea.Msg
}

type ErrorMsg struct {
	Error error
}

type StatusUpdateMsg struct {
	Status []StatusItem
}

type StreamChunkMsg struct {
	Chunk *llm.StreamChunk
}

type StreamCompleteMsg struct {
	ToolCalls []llm.ToolCall
}

// Permission-related message types
type PermissionRequestMsg struct {
	ToolID        string
	ToolCall      *llm.ToolCall
	RiskLevel     string
	RiskReasons   []string
	AffectedFiles []string
	QueuePosition int
	TotalQueued   int
	PromptText    string
	ResponseChan  chan bool
}

type PermissionResponseMsg struct {
	ToolID         string
	ToolCall       *llm.ToolCall
	Approved       bool
	RememberChoice bool
	ApplyToSimilar bool
}

// Permission batch message for handling multiple tools at once
type PermissionBatchMsg struct {
	Tools []ToolExecutionInfo
}

type ToolExecutionInfo struct {
	ID            string
	ToolCall      *llm.ToolCall
	RiskLevel     string
	AffectedFiles []string
}

// Permission decision message for responses
type PermissionDecisionMsg struct {
	ToolID         string
	Approved       bool
	RememberChoice bool
	ApplyToSimilar bool
}

// CommandAdapter adapts the Model to the commands.CommandModel interface
type CommandAdapter struct {
	model *Model
}

func (a *CommandAdapter) GetSession() commands.Session {
	return &SessionAdapter{session: a.model.session}
}

func (a *CommandAdapter) GetSessionManager() commands.SessionManager {
	return &SessionManagerAdapter{sm: a.model.sessionManager}
}

func (a *CommandAdapter) LoadFiles() {
	a.model.loadFiles()
}

func (a *CommandAdapter) AddMessage(role, content string, streaming bool) {
	a.model.addMessage(ChatMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
		Streaming: streaming,
	})
}

// SessionAdapter adapts the session to the commands.Session interface
type SessionAdapter struct {
	session *session.Session
}

func (s *SessionAdapter) GetFiles() []string {
	return s.session.GetFiles()
}

func (s *SessionAdapter) GetProject() commands.Project {
	proj := s.session.GetProject()
	if proj == nil {
		return nil
	}
	return &ProjectAdapter{project: proj}
}

func (s *SessionAdapter) GetRootPath() string {
	return s.session.GetRootPath()
}

func (s *SessionAdapter) AddFile(ctx context.Context, path string) error {
	return s.session.AddFile(ctx, path)
}

func (s *SessionAdapter) GetDiffOutput() (string, error) {
	return s.session.GetDiffOutput()
}

func (s *SessionAdapter) CommitChanges(ctx context.Context, message string) error {
	return s.session.CommitChanges(ctx, message)
}

func (s *SessionAdapter) CommitWithAI(ctx context.Context) (string, error) {
	return s.session.CommitWithAI(ctx)
}

func (s *SessionAdapter) SetModel(model string) error {
	return s.session.SetModel(model)
}

func (s *SessionAdapter) GetModel() string {
	return s.session.GetModel()
}

func (s *SessionAdapter) SetProvider(provider string) error {
	return s.session.SetProvider(provider)
}

func (s *SessionAdapter) GetProvider() string {
	return s.session.GetProvider()
}

func (s *SessionAdapter) GetAvailableProviders() []string {
	return s.session.GetAvailableProviders()
}

func (s *SessionAdapter) GetAvailableModels() map[string][]commands.ModelInfo {
	models := s.session.GetAvailableModels()
	result := make(map[string][]commands.ModelInfo)

	for provider, modelList := range models {
		var infos []commands.ModelInfo
		for _, m := range modelList {
			// Convert llm.Model to commands.ModelInfo
			infos = append(infos, commands.ModelInfo{
				ID:   m.ID,
				Name: m.Name,
			})
		}
		result[provider] = infos
	}

	return result
}

func (s *SessionAdapter) GetProjectSummary() string {
	return s.session.GetProjectSummary()
}

func (s *SessionAdapter) GetBranchInfo() (string, error) {
	return s.session.GetBranchInfo()
}

func (s *SessionAdapter) GetCommitHistory(limit int) (string, error) {
	return s.session.GetCommitHistory(limit)
}

func (s *SessionAdapter) GetMemoryContent() *commands.MemoryContent {
	content := s.session.GetMemoryContent()
	if content == nil {
		return nil
	}

	return &commands.MemoryContent{
		UserMemory:    content.UserMemory,
		ProjectMemory: content.ProjectMemory,
		ImportedFiles: content.ImportedFiles,
	}
}

func (s *SessionAdapter) GetMemoryFilePaths() (string, string) {
	return s.session.GetMemoryFilePaths()
}

func (s *SessionAdapter) CreateMemoryFile(ctx context.Context, isUserMemory bool) error {
	return s.session.CreateMemoryFile(ctx, isUserMemory)
}

func (s *SessionAdapter) ReloadMemory(ctx context.Context) error {
	return s.session.ReloadMemory(ctx)
}

func (s *SessionAdapter) AddQuickMemory(ctx context.Context, note string, isUserMemory bool) error {
	return s.session.AddQuickMemory(ctx, note, isUserMemory)
}

func (s *SessionAdapter) GetPermissionManager() commands.PermissionManager {
	pm := s.session.GetPermissionManager()
	if pm == nil {
		return nil
	}
	return &PermissionManagerAdapter{pm: pm}
}

func (s *SessionAdapter) ID() string {
	return s.session.GetID()
}

// ProjectAdapter adapts the project to the commands.Project interface
type ProjectAdapter struct {
	project *project.Project
}

func (p *ProjectAdapter) GetRelevantFiles(limit int) []string {
	return p.project.GetRelevantFiles(limit)
}

func (p *ProjectAdapter) Root() string {
	return p.project.Root
}

// SessionManagerAdapter adapts the session manager
type SessionManagerAdapter struct {
	sm *session.Manager
}

func (sm *SessionManagerAdapter) SaveSession(session commands.Session) error {
	// Convert back to original session type
	if adapter, ok := session.(*SessionAdapter); ok {
		return sm.sm.SaveSession(adapter.session)
	}
	return fmt.Errorf("invalid session type")
}

func (sm *SessionManagerAdapter) ListSavedSessions() ([]commands.SavedSessionInfo, error) {
	sessions, err := sm.sm.ListSavedSessions()
	if err != nil {
		return nil, err
	}

	var result []commands.SavedSessionInfo
	for _, sess := range sessions {
		result = append(result, commands.SavedSessionInfo{
			ID:        sess.ID,
			Name:      sess.Name,
			CreatedAt: sess.CreatedAt,
		})
	}

	return result, nil
}

// PermissionManagerAdapter adapts the permission manager
type PermissionManagerAdapter struct {
	pm *session.PermissionManager
}

func (pma *PermissionManagerAdapter) GetToolRisk(toolCall interface{}) string {
	// For the simplified interface, we just return a static value
	// In a full implementation, this would analyze the actual tool call
	return "medium"
}

// handleSendMessage processes user input and sends to AI
func (m *Model) handleSendMessage() tea.Cmd {
	input := strings.TrimSpace(m.textarea.Value())
	loggy.Info("handleSendMessage: received input", "input", input, "length", len(input))
	if input == "" {
		loggy.Info("handleSendMessage: empty input, returning nil")
		return nil
	}

	// Clear the input
	m.textarea.Reset()

	// Add user message to chat
	m.addMessage(ChatMessage{
		Role:      "user",
		Content:   input,
		Timestamp: time.Now(),
	})

	// Check if it's a session command
	if strings.HasPrefix(input, "/") || strings.HasPrefix(input, "#") {
		loggy.Info("handleSendMessage: detected command", "input", input)
		loggy.Debug("handleSendMessage: detected command", "input", input)
		return m.handleSessionCommand(input)
	}

	// Set thinking state and add placeholder message
	m.isThinking = true
	m.thinkingStartTime = time.Now()
	// Estimate input tokens (rough approximation: 1 token ≈ 4 characters)
	m.inputTokens = len(input) / 4
	if m.inputTokens < 1 {
		m.inputTokens = 1
	}
	m.addMessage(ChatMessage{
		Role:      "assistant",
		Content:   "",
		Timestamp: time.Now(),
		Streaming: true,
	})

	// Send to AI
	loggy.Debug("UI handling send message", "component", "handleSendMessage", "action", "calling_sendToAI", "input", input)
	return m.sendToAI(input)
}

// handleSessionCommand processes session commands using the new command registry
func (m *Model) handleSessionCommand(command string) tea.Cmd {
	// Initialize command registry if not already done
	if m.commandRegistry == nil {
		m.commandRegistry = commands.NewRegistry()
	}

	return func() tea.Msg {
		loggy.Info("handleSessionCommand: processing command", "command", command)
		loggy.Debug("handleSessionCommand: processing command", "command", command)

		// Create command adapter
		adapter := &CommandAdapter{model: m}

		// Execute command through registry
		result := m.commandRegistry.Execute(context.Background(), command, adapter)

		loggy.Info("handleSessionCommand: command completed", "command", command)
		loggy.Debug("handleSessionCommand: command completed", "command", command)

		// Handle special message types that need status updates
		switch msg := result.(type) {
		case commands.LLMRequestMsg:
			// Return the message for processing in the main Update loop
			return msg
		case commands.StatusUpdateMsg:
			// Update status for model/provider changes
			for i := range m.status {
				if strings.Contains(m.status[i].Text, "Model:") && msg.ModelName != "" {
					m.status[i].Text = "Model: " + msg.ModelName
					break
				}
				if strings.Contains(m.status[i].Text, "Provider:") && msg.ModelName != "" {
					m.status[i].Text = "Provider: " + msg.ModelName
					break
				}
			}
			return ResponseMsg{Content: msg.Response}
		case commands.ResponseMsg:
			return ResponseMsg{Content: msg.Content, ToolCalls: nil}
		default:
			return result
		}
	}
}

// sendToAI sends user message to AI and returns streaming response
func (m *Model) sendToAI(message string) tea.Cmd {
	return func() tea.Msg {
		loggy.Debug("UI sending to AI", "component", "sendToAI", "action", "starting", "message", message)

		// Use streaming response for real-time updates
		streamChan, err := m.session.ProcessMessageStream(context.Background(), message)
		if err != nil {
			loggy.Error("UI send to AI failed", "component", "sendToAI", "error", "ProcessMessageStream_failed", "err", err, "message", message)
			return ErrorMsg{Error: fmt.Errorf("failed to process message: %w", err)}
		}

		// Test that we got a valid channel
		if streamChan == nil {
			loggy.Error("UI send to AI failed", "component", "sendToAI", "error", "nil_stream_channel", "err", fmt.Errorf("received nil stream channel"))
			return ErrorMsg{Error: fmt.Errorf("received nil stream channel")}
		}

		loggy.Debug("UI sending to AI", "component", "sendToAI", "action", "stream_channel_created", "returning", "StreamStartMsg")
		// Return a command that will listen for stream chunks
		return StreamStartMsg{StreamChan: streamChan}
	}
}

// StreamStartMsg indicates streaming has started
type StreamStartMsg struct {
	StreamChan <-chan *llm.StreamChunk
}

// listenForStreamChunks creates a command to listen for streaming chunks
func listenForStreamChunks(streamChan <-chan *llm.StreamChunk) tea.Cmd {
	return func() tea.Msg {
		loggy.Debug("UI stream listener", "event", "listening_for_chunks", "action", "waiting_for_data")
		chunk, ok := <-streamChan
		if !ok {
			loggy.Debug("UI stream listener", "event", "stream_complete", "action", "channel_closed")
			// Stream is complete
			return StreamCompleteMsg{}
		}
		loggy.Debug("UI stream listener", "event", "chunk_received", "content", chunk.Content, "type", chunk.Type)
		return StreamChunkMsg{Chunk: chunk}
	}
}

// handleResponse processes AI response
func (m *Model) handleResponse(msg ResponseMsg) {
	loggy.Debug("handleResponse: processing response", "content_length", len(msg.Content), "tool_calls_count", len(msg.ToolCalls))

	// Update the last streaming message or add new one
	if len(m.messages) > 0 && m.messages[len(m.messages)-1].Streaming {
		// Update the streaming message
		loggy.Debug("handleResponse: updating streaming message", "action", "update_streaming")
		m.messages[len(m.messages)-1].Content = msg.Content
		m.messages[len(m.messages)-1].Streaming = false
	} else {
		// Add new message
		loggy.Debug("handleResponse: adding new message", "action", "add_new_message", "content_length", len(msg.Content))
		chatMsg := ChatMessage{
			Role:      "assistant",
			Content:   msg.Content,
			Timestamp: time.Now(),
		}
		m.addMessage(chatMsg)
		loggy.Debug("handleResponse: message added", "total_messages", len(m.messages))
	}

	// Handle tool calls if any
	if len(msg.ToolCalls) > 0 {
		go m.executeTools(msg.ToolCalls)
	}

	// Scroll to bottom
	m.chatViewport.GotoBottom()
}

// handleError processes error messages
func (m *Model) handleError(msg ErrorMsg) {
	// Remove streaming message if present
	if len(m.messages) > 0 && m.messages[len(m.messages)-1].Streaming {
		m.messages = m.messages[:len(m.messages)-1]
	}

	// Add error message
	m.addMessage(ChatMessage{
		Role:      "system",
		Content:   "❌ Error: " + msg.Error.Error(),
		Timestamp: time.Now(),
	})

	m.chatViewport.GotoBottom()
}

// executeTools executes tool calls and sends results back to UI
func (m *Model) executeTools(toolCalls []llm.ToolCall) {
	for _, toolCall := range toolCalls {
		m.addBazingaToolMessage(&toolCall, "start")

		// Execute the tool using session's tool executor
		var result string
		var err error

		if toolExecutor := m.session.GetToolExecutor(); toolExecutor != nil {
			result, err = toolExecutor.ExecuteTool(context.Background(), &toolCall)
		} else {
			err = fmt.Errorf("tool executor not available")
		}

		m.addBazingaToolMessage(&toolCall, "complete", result, err)

		// Log completion for debugging (simplified tracking)
		loggy.Debug("Tool execution completed", "tool_name", toolCall.Name, "tool_id", toolCall.ID, "result_length", len(result), "has_error", err != nil)

		// Refresh file list if it's a file operation
		if toolCall.Name == "write_file" || toolCall.Name == "create_file" || toolCall.Name == "edit_file" {
			m.loadFiles()
		}
	}
}

// addBazingaToolMessage adds tool execution messages to the chat
func (m *Model) addBazingaToolMessage(toolCall *llm.ToolCall, phase string, result ...interface{}) {
	var message string

	switch phase {
	case "start":
		fileName := m.getToolFileName(toolCall)
		action := m.getToolActionName(toolCall.Name)
		if fileName != "" {
			message = fmt.Sprintf("• %s (%s)", action, fileName)
		} else {
			message = fmt.Sprintf("• %s", action)
		}

	case "complete":
		if len(result) > 0 {
			resultStr, _ := result[0].(string)
			errorObj := interface{}(nil)
			if len(result) > 1 {
				errorObj = result[1]
			}

			if err, isError := errorObj.(error); isError && err != nil {
				// Error case
				message = fmt.Sprintf("❌ Error: %s", err.Error())
			} else {
				message = m.formatToolResult(toolCall, resultStr)
			}
		}
	}

	if message != "" {
		m.addMessage(ChatMessage{
			Role:      "system",
			Content:   message,
			Timestamp: time.Now(),
			IsToolMsg: true,
			ToolName:  toolCall.Name,
		})
	}
}

// getToolFileName extracts filename from tool call arguments
func (m *Model) getToolFileName(toolCall *llm.ToolCall) string {
	if filePath, ok := toolCall.Input["file_path"].(string); ok {
		// Get relative path if possible
		if relPath, err := filepath.Rel(m.session.GetRootPath(), filePath); err == nil && !strings.HasPrefix(relPath, "..") {
			return relPath
		}
		return filepath.Base(filePath)
	}

	// For other tool types
	switch toolCall.Name {
	case "bash":
		if command, ok := toolCall.Input["command"].(string); ok {
			words := strings.Fields(command)
			if len(words) > 0 {
				return words[0] // First word of command
			}
		}
	case "grep":
		if pattern, ok := toolCall.Input["pattern"].(string); ok {
			return fmt.Sprintf("'%s'", pattern)
		}
	case "web_fetch":
		if url, ok := toolCall.Input["url"].(string); ok {
			return url
		}
	}

	return ""
}

// getToolActionName returns user-friendly action name for tool
func (m *Model) getToolActionName(toolName string) string {
	switch toolName {
	case "read_file":
		return "Read"
	case "write_file":
		return "Write"
	case "create_file":
		return "Create"
	case "edit_file":
		return "Edit"
	case "bash":
		return "Run"
	case "grep":
		return "Search"
	case "web_fetch":
		return "Fetch"
	default:
		return strings.ToUpper(string(toolName[0])) + strings.ToLower(strings.ReplaceAll(toolName[1:], "_", " "))
	}
}

// formatToolResult formats tool execution results for display
func (m *Model) formatToolResult(toolCall *llm.ToolCall, result string) string {
	switch toolCall.Name {
	case "read_file":
		// Extract line count from our modified readFile result
		if strings.Contains(result, "Lines: ") {
			// Parse "File: path\nLines: 123\nContent:\n..." format
			lines := strings.Split(result, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "Lines: ") {
					lineCount := strings.TrimPrefix(line, "Lines: ")
					return fmt.Sprintf("Read %s lines (ctrl+r to expand)", lineCount)
				}
			}
		}
		// Fallback - count lines manually
		lines := strings.Count(result, "\n")
		return fmt.Sprintf("Read %d lines (ctrl+r to expand)", lines)

	case "write_file", "create_file":
		// Extract file size info
		if strings.Contains(result, " bytes)") {
			// Extract size from "File ... written successfully (123 bytes)"
			if idx := strings.LastIndex(result, "("); idx != -1 {
				if endIdx := strings.Index(result[idx:], " bytes)"); endIdx != -1 {
					sizeStr := result[idx+1 : idx+endIdx]
					return fmt.Sprintf("Written %s bytes", sizeStr)
				}
			}
		}
		return "File written successfully"

	case "bash":
		// For bash commands, show brief execution result
		if len(result) > 100 {
			return fmt.Sprintf("Command executed (%d chars output)", len(result))
		}
		return "Command executed"

	default:
		return "Completed"
	}
}
