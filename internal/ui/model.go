package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/tildaslashalef/bazinga/internal/llm"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"github.com/tildaslashalef/bazinga/internal/session"
	"github.com/tildaslashalef/bazinga/internal/tools"
	"github.com/tildaslashalef/bazinga/internal/ui/commands"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// ChatMessage represents a message in the chat
type ChatMessage struct {
	Role      string // "user", "assistant", "system", "tool"
	Content   string
	Timestamp time.Time
	Streaming bool                   // For streaming responses
	IsToolMsg bool                   // Flag for tool-related messages
	ToolName  string                 // Name of tool if it's a tool message
	ToolArgs  map[string]interface{} // Arguments for tool call
	ToolState string                 // "start", "complete", or "error"
	TaskGroup string                 // Optional task group for grouping related tools
}

// Model represents the main UI state for the chat interface
type Model struct {
	session    *session.Session
	viewport   viewport.Model
	textarea   textarea.Model
	messages   []ChatMessage
	isThinking bool
	width      int
	height     int
	ready      bool

	// Streaming state
	currentStream <-chan *llm.StreamChunk

	// Status tracking
	inputTokens       int
	outputTokens      int
	toolCount         int
	thinkingStartTime time.Time

	// Simplified tool tracking via chat messages

	// Compatibility fields for commands.go
	status          []StatusItem
	glamourRenderer *glamour.TermRenderer
	sessionManager  *session.Manager
	chatViewport    viewport.Model

	// Autocomplete system
	autocomplete *AutocompleteState

	// Shortcuts overlay system
	showShortcuts bool

	// Command registry for modular command handling
	commandRegistry *commands.Registry

	// File diff tracking
	fileDiffs []*FileDiff

	// Permission system state
	pendingPermission *PermissionRequest
	permissionHistory map[string]bool      // Remember permissions for session
	permissionQueue   []*PermissionRequest // Queue of pending permissions
}

// PermissionRequest represents a pending permission request
type PermissionRequest struct {
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

// SetupFileChangeCallback configures the file change callback for diff tracking
func (m *Model) SetupFileChangeCallback() {
	if m.session != nil {
		toolExecutor := m.session.GetToolExecutor()
		if toolExecutor != nil {
			toolExecutor.SetFileChangeCallback(func(change tools.FileChange) {
				diff := GenerateDiff(change.FilePath, change.Before, change.After, change.Operation)
				m.fileDiffs = append(m.fileDiffs, diff)

				// Add diff message to chat
				diffContent := diff.RenderDiff()
				if diffContent != "" {
					m.addMessage(ChatMessage{
						Role:      "system",
						Content:   diffContent,
						Timestamp: time.Now(),
					})
				}
			})
		}
	}
}

// SetupPermissionCallback configures the permission callback for tool execution approval
func (m *Model) SetupPermissionCallback() {
	if m.session != nil {
		permissionManager := m.session.GetPermissionManager()
		if permissionManager != nil {
			// Set up permission callback with terminator support
			permissionManager.SetPromptCallback(func(toolCall *llm.ToolCall) bool {
				// Check if terminator mode is enabled (bypass all permissions)
				if m.session.IsTerminatorMode() {
					loggy.Info("Terminator mode enabled, bypassing permission check", "tool", toolCall.Name)
					return true
				}

				// Check if we already have permission for this tool call (session memory)
				key := m.generatePermissionKey(toolCall)
				if approved, exists := m.permissionHistory[key]; exists {
					loggy.Debug("Using cached permission decision", "tool", toolCall.Name, "approved", approved)
					return approved
				}

				// Get risk level
				risk := permissionManager.GetToolRisk(toolCall)

				// Auto-approve low risk tools, prompt for medium/high risk
				switch risk {
				case "low":
					loggy.Debug("Auto-approving low risk tool", "tool", toolCall.Name)
					m.permissionHistory[key] = true
					return true
				case "medium", "high":
					// Use async permission system for real user prompts
					loggy.Info("Tool requires permission, requesting async approval", "tool", toolCall.Name, "risk", risk)

					// Create response channel for this permission request
					responseChan := make(chan bool, 1)

					// Create permission request
					promptText := permissionManager.FormatPermissionPrompt(toolCall)
					riskReasons := permissionManager.GetRiskReasons(toolCall)

					// Generate unique tool ID
					toolID := time.Now().Format("20060102-150405.000000")

					request := &PermissionRequest{
						ToolID:        toolID,
						ToolCall:      toolCall,
						RiskLevel:     risk,
						RiskReasons:   riskReasons,
						AffectedFiles: []string{}, // TODO: Extract from tool call
						QueuePosition: len(m.permissionQueue) + 1,
						TotalQueued:   len(m.permissionQueue) + 1,
						PromptText:    promptText,
						ResponseChan:  responseChan,
					}

					// Add to queue and set as current pending permission
					m.permissionQueue = append(m.permissionQueue, request)
					if m.pendingPermission == nil {
						m.pendingPermission = request
					}

					// Wait for user decision (this will block until user responds)
					approved := <-responseChan

					// Cache the decision
					m.permissionHistory[key] = approved

					// Remove from queue
					m.removePermissionFromQueue(toolID)

					return approved
				default:
					loggy.Warn("Unknown risk level, denying tool execution", "tool", toolCall.Name, "risk", risk)
					m.permissionHistory[key] = false
					return false
				}
			})
		}
	}
}

// generatePermissionKey creates a key for remembering permission decisions
func (m *Model) generatePermissionKey(toolCall *llm.ToolCall) string {
	// Create a simple key based on tool name and main parameters
	key := toolCall.Name
	if filePath, ok := toolCall.Input["file_path"].(string); ok {
		key += ":" + filePath
	}
	if command, ok := toolCall.Input["command"].(string); ok {
		key += ":" + command
	}
	return key
}

// removePermissionFromQueue removes a permission request from the queue and updates current pending
func (m *Model) removePermissionFromQueue(toolID string) {
	// Remove from queue
	for i, req := range m.permissionQueue {
		if req.ToolID == toolID {
			m.permissionQueue = append(m.permissionQueue[:i], m.permissionQueue[i+1:]...)
			break
		}
	}

	// Clear current pending permission if it matches
	if m.pendingPermission != nil && m.pendingPermission.ToolID == toolID {
		m.pendingPermission = nil

		// Set next permission as pending if queue is not empty
		if len(m.permissionQueue) > 0 {
			m.pendingPermission = m.permissionQueue[0]
		}
	}
}

// NewModel creates a new UI model for the chat interface
func NewModel(sess *session.Session, sessionManager *session.Manager) *Model {
	// Initialize textarea for input
	ta := textarea.New()
	ta.Placeholder = "Ask bazinga anything about your code..."
	ta.CharLimit = 4000
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.Focus()

	// Initialize viewport for chat
	vp := viewport.New(80, 20)

	// Initialize glamour renderer for markdown with custom Gruvbox styling
	// To use built-in Dracula theme instead: glamour.WithStylePath("dracula")
	// Other built-in themes: "dark", "light", "notty", "ascii"
	glamourRenderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dracula"),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		glamourRenderer = nil
		loggy.Warn("Failed to initialize glamour renderer", "error", err)
	}

	model := &Model{
		session:         sess,
		viewport:        vp,
		textarea:        ta,
		messages:        make([]ChatMessage, 0),
		isThinking:      false,
		status:          make([]StatusItem, 0),
		glamourRenderer: glamourRenderer,
		chatViewport:    vp, // Same as viewport for compatibility
		sessionManager:  sessionManager,
		autocomplete:    NewAutocompleteState(),
		// Tool display handled via chat messages
		permissionHistory: make(map[string]bool),
		permissionQueue:   make([]*PermissionRequest, 0),
	}

	welcomeMessage := model.createWelcomeMessage()
	model.addMessage(ChatMessage{
		Role:      "system",
		Content:   welcomeMessage,
		Timestamp: time.Now(),
	})

	// Setup file change callback for diff tracking
	model.SetupFileChangeCallback()

	// Setup permission callback for tool execution approval
	model.SetupPermissionCallback()

	loggy.Debug("UI model initialized", "component", "NewModel", "provider", sess.GetProvider(), "model", sess.GetModel())
	return model
}

// TickMsg is sent periodically to update the UI
type TickMsg time.Time

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.tickCmd(),
	)
}

// tickCmd returns a command that sends a tick message every second
func (m *Model) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Update handles messages and updates the model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateDimensions()
		if !m.ready {
			m.ready = true
		}

	case tea.MouseMsg:
		// Handle mouse events to prevent garbage output in input area
		switch msg.Action {
		case tea.MouseActionPress:
			// Handle wheel events via Button
			if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
				// Only allow scroll in viewport area, not in input area
				if m.isMouseInInputArea(msg.Y) {
					return m, nil // Ignore mouse scroll in input area
				}
				// Allow scroll in viewport
				m.viewport, cmd = m.viewport.Update(msg)
			}
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		default:
			// Block ALL other mouse events from reaching textarea to prevent escape sequences
			return m, nil
		}

	case tea.KeyMsg:
		key := msg.String()
		// Log ALL keys to debug shift+enter

		// Handle permission prompts first (highest priority)
		if m.pendingPermission != nil {
			switch key {
			case "y", "Y":
				// Send approval and cache decision
				m.pendingPermission.ResponseChan <- true
				m.permissionHistory[m.generatePermissionKey(m.pendingPermission.ToolCall)] = true

				// Remove from queue and move to next
				toolID := m.pendingPermission.ToolID
				m.removePermissionFromQueue(toolID)
				return m, nil

			case "n", "N", "esc":
				// Send denial and cache decision
				m.pendingPermission.ResponseChan <- false
				m.permissionHistory[m.generatePermissionKey(m.pendingPermission.ToolCall)] = false

				// Remove from queue and move to next
				toolID := m.pendingPermission.ToolID
				m.removePermissionFromQueue(toolID)
				return m, nil

			case "a", "A":
				// Approve and remember for session
				m.pendingPermission.ResponseChan <- true
				key := m.generatePermissionKey(m.pendingPermission.ToolCall)
				m.permissionHistory[key] = true

				// TODO: Add to session rules for pattern matching

				// Remove from queue and move to next
				toolID := m.pendingPermission.ToolID
				m.removePermissionFromQueue(toolID)
				return m, nil

			default:
				// Ignore all other keys when permission prompt is active
				return m, nil
			}
		}

		switch key {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+d":
			return m, tea.Quit
		case "?":
			// Toggle shortcuts overlay
			m.showShortcuts = !m.showShortcuts
			return m, nil
		case "shift+enter", "alt+enter", "ctrl+j", "ctrl+m":
			// Shift+Enter, Alt+Enter, Ctrl+J, or Ctrl+M: Insert new line
			loggy.Info("KeyMsg: New line key pressed - inserting new line", "key", key)
			// Manually insert newline
			currentValue := m.textarea.Value()
			m.textarea.SetValue(currentValue + "\n")
			m.textarea.CursorEnd()
			return m, nil
		case "enter":
			loggy.Info("KeyMsg: Enter pressed", "autocomplete_active", m.autocomplete.IsActive(), "is_thinking", m.isThinking, "textarea_value", m.textarea.Value())
			// Handle autocomplete selection
			if m.autocomplete.IsActive() {
				selected := m.autocomplete.GetSelected()
				currentInput := strings.TrimSpace(m.textarea.Value())

				if selected != nil {
					if selected.Command != currentInput {
						// Replace current input with selected command
						m.textarea.SetValue(selected.Command + " ")
						m.textarea.CursorEnd()
						m.autocomplete.Deactivate()
						loggy.Info("KeyMsg: Enter - autocomplete selection made", "selected_command", selected.Command, "was_different_from_input", true)
						return m, nil
					} else {
						// When exact match, deactivate autocomplete and proceed with execution
						m.autocomplete.Deactivate()
						loggy.Info("KeyMsg: Enter - exact match, deactivating autocomplete and executing command", "current_input", currentInput)
						// Continue to the execution code below - don't return here
					}
				} else {
					// No selection but autocomplete active, just deactivate
					m.autocomplete.Deactivate()
					loggy.Info("KeyMsg: Enter - deactivating autocomplete with no selection", "current_input", currentInput)
				}
			}

			// Normal message sending
			if !m.isThinking {
				loggy.Info("KeyMsg: Enter - calling handleSendMessage")
				return m, m.handleSendMessage()
			} else {
				loggy.Info("KeyMsg: Enter - blocked because isThinking=true")
			}
		case "tab":
			// Complete with first suggestion
			if m.autocomplete.IsActive() {
				selected := m.autocomplete.GetSelected()
				if selected != nil {
					m.textarea.SetValue(selected.Command + " ")
					m.textarea.CursorEnd()
					m.autocomplete.Deactivate()
					return m, nil
				}
			}
		case "up":
			if m.autocomplete.IsActive() {
				m.autocomplete.Navigate(-1)
				return m, nil
			}
		case "down":
			if m.autocomplete.IsActive() {
				m.autocomplete.Navigate(1)
				return m, nil
			}
		case "esc":
			if m.showShortcuts {
				m.showShortcuts = false
				return m, nil
			}
			if m.autocomplete.IsActive() {
				m.autocomplete.Deactivate()
				return m, nil
			}
			// Allow ESC to interrupt AI response
			if m.isThinking {
				m.isThinking = false
				m.currentStream = nil
				// Remove streaming message if present
				if len(m.messages) > 0 && m.messages[len(m.messages)-1].Streaming {
					m.messages = m.messages[:len(m.messages)-1]
				}
				// Add interruption message
				m.addMessage(ChatMessage{
					Role:      "system",
					Content:   "⚠️ Response interrupted by user",
					Timestamp: time.Now(),
				})
				return m, nil
			}
		}

	case StreamChunkMsg:
		loggy.Debug("UI model update", "event", "StreamChunkMsg_received", "content", msg.Chunk.Content)
		m.handleStreamChunk(msg)
		// Continue listening for more chunks
		if m.currentStream != nil {
			cmds = append(cmds, listenForStreamChunks(m.currentStream))
		}

	case StreamStartMsg:
		loggy.Debug("UI model update", "event", "StreamStartMsg_received", "action", "starting_listener")
		m.currentStream = msg.StreamChan
		cmds = append(cmds, listenForStreamChunks(msg.StreamChan))

	case StreamCompleteMsg:
		loggy.Debug("StreamCompleteMsg received", "tool_calls_count", len(msg.ToolCalls))
		m.handleStreamComplete()
		// Process tool completions from the completed stream
		for _, toolCall := range msg.ToolCalls {
			loggy.Debug("Processing tool completion from StreamCompleteMsg", "tool_name", toolCall.Name, "llm_tool_id", toolCall.ID)
			// Tool execution completed (tracked via simple messages now)
			loggy.Debug("Tool call completed", "tool_name", toolCall.Name, "tool_id", toolCall.ID)
		}
		// Stream completed - tool execution is now handled via simple messages
		loggy.Debug("StreamCompleteMsg: stream completed")

	case ErrorMsg:
		m.handleError(msg)

	case ResponseMsg:
		loggy.Debug("Model: received ResponseMsg", "content_length", len(msg.Content))
		m.handleResponse(msg)

	case commands.LLMRequestMsg:
		// Handle LLM request from commands
		loggy.Debug("Model: received LLMRequestMsg", "message_length", len(msg.Message))

		// Add user message and trigger LLM processing
		m.addMessage(ChatMessage{
			Role:      "user",
			Content:   msg.Message,
			Timestamp: time.Now(),
		})

		// Set thinking state and add placeholder message
		m.isThinking = true
		m.thinkingStartTime = time.Now()
		m.inputTokens = len(msg.Message) / 4
		if m.inputTokens < 1 {
			m.inputTokens = 1
		}
		m.addMessage(ChatMessage{
			Role:      "assistant",
			Content:   "",
			Timestamp: time.Now(),
			Streaming: true,
		})

		// Trigger LLM processing
		cmds = append(cmds, m.sendToAI(msg.Message))

	case TickMsg:
		// Continue ticking and force a re-render if thinking (for dynamic timer)
		cmds = append(cmds, m.tickCmd())
		if m.isThinking {
			// Force a re-render to update the thinking timer
			return m, tea.Batch(cmds...)
		}

	case PermissionRequestMsg:
		// Handle permission request from tool execution
		m.pendingPermission = &PermissionRequest{
			ToolCall:     msg.ToolCall,
			PromptText:   msg.PromptText,
			ResponseChan: msg.ResponseChan,
		}
		return m, nil

	case PermissionResponseMsg:
		// Handle user's permission response
		if m.pendingPermission != nil && m.pendingPermission.ToolCall == msg.ToolCall {
			m.pendingPermission.ResponseChan <- msg.Approved
			m.pendingPermission = nil
		}
		return m, nil
	}

	// Update textarea
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	// Update autocomplete based on current input
	m.autocomplete.Update(m.textarea.Value())

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the simplified chat interface
func (m *Model) View() string {
	if !m.ready {
		return "Starting bazinga..."
	}

	header := HeaderStyle.Width(m.width).Render("bazinga")

	chatContent := m.renderChatContent()
	m.viewport.SetContent(chatContent)

	statusBar := m.renderStatusBar()
	inputArea := m.renderInputWithBorder()
	helpMessages := m.renderHelpMessages()

	// Build autocomplete overlay if active
	var autocompleteOverlay string
	if m.autocomplete.IsActive() {
		autocompleteOverlay = m.autocomplete.Render(m.width)
	}

	// Build shortcuts overlay if active
	var shortcutsOverlay string
	if m.showShortcuts {
		shortcutsOverlay = m.renderShortcutsOverlay()
	}

	// Combine all parts
	parts := []string{
		header,
		m.viewport.View(),
		"", // Empty line for spacing
	}

	// Tool status is now handled via chat messages (no separate rendering needed)

	parts = append(parts, statusBar)

	// Add permission prompt if active (highest priority overlay)
	if m.pendingPermission != nil {
		permissionPrompt := m.renderPermissionPrompt()
		parts = append(parts, permissionPrompt)
	} else {
		// Add overlays before input (only when no permission prompt)
		if shortcutsOverlay != "" {
			parts = append(parts, shortcutsOverlay)
		} else if autocompleteOverlay != "" {
			parts = append(parts, autocompleteOverlay)
		}
	}

	parts = append(parts, inputArea, helpMessages)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderStatusBar renders the status bar showing thinking state
func (m *Model) renderStatusBar() string {
	var leftStatus, rightStatus string

	if m.isThinking {
		// Left side: " Thinking... (38s • ↑ 839 tokens • esc to interrupt)"
		thinkingDuration := time.Since(m.thinkingStartTime)
		seconds := int(thinkingDuration.Seconds())

		var statusParts []string
		statusParts = append(statusParts, fmt.Sprintf("%ds", seconds))

		if m.inputTokens > 0 {
			statusParts = append(statusParts, fmt.Sprintf("↑ %d tokens", m.inputTokens))
		}

		statusParts = append(statusParts, "esc to interrupt")

		leftStatus = lipgloss.NewStyle().Foreground(WarningColor).Render(
			fmt.Sprintf("✨ Thinking... (%s)", strings.Join(statusParts, " • ")))
	} else {
		leftStatus = lipgloss.NewStyle().Foreground(SuccessColor).Render("● Ready")
	}

	if m.isThinking {
		rightStatus = lipgloss.NewStyle().Foreground(TextSecondary).Render("AI responding...")
	} else {
		// Remove model display - keep right side empty when not thinking
		rightStatus = ""
	}

	// Create left/right layout
	leftWidth := lipgloss.Width(leftStatus)
	rightWidth := lipgloss.Width(rightStatus)
	padding := m.width - leftWidth - rightWidth - 2 // Account for padding

	if padding < 0 {
		padding = 0
	}

	spacer := strings.Repeat(" ", padding)

	statusContent := leftStatus + spacer + rightStatus
	return StatusBarStyle.Width(m.width).Render(statusContent)
}

// renderInputWithBorder renders the input area with prompt and border
func (m *Model) renderInputWithBorder() string {
	if m.isThinking {
		// Show a simple thinking indicator with border
		inputWidth := m.width - 4 // Match the chat width calculation
		thinking := lipgloss.NewStyle().
			Foreground(TextSecondary).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(0, 1).
			Width(inputWidth).
			Render(" AI is responding...")
		return thinking
	}

	// Create our custom prompt
	prompt := lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true).
		Render("> ")

	// Get the actual text content
	inputText := m.textarea.Value()

	// Handle empty input with placeholder
	if inputText == "" {
		placeholderText := lipgloss.NewStyle().
			Foreground(TextMuted).
			Render(m.textarea.Placeholder)

		// Add cursor right after the prompt, before placeholder
		cursor := ""
		if m.textarea.Focused() {
			cursor = lipgloss.NewStyle().
				Foreground(AccentColor).
				Render("▋")
		}

		content := prompt + cursor + placeholderText
		inputWidth := m.width - 4
		return InputStyle.Width(inputWidth).Height(3).Render(content)
	}

	// For text input, show cursor at the end of the last line
	lines := strings.Split(inputText, "\n")
	processedLines := make([]string, len(lines))

	for i, line := range lines {
		if i == 0 {
			// First line gets the prompt
			processedLines[i] = prompt + line
		} else {
			// Continuation lines get indentation
			processedLines[i] = "  " + line
		}
	}

	// Add cursor to the last line if focused
	if m.textarea.Focused() && len(processedLines) > 0 {
		cursor := lipgloss.NewStyle().
			Foreground(AccentColor).
			Render("▋")
		lastIdx := len(processedLines) - 1
		processedLines[lastIdx] += cursor
	}

	content := strings.Join(processedLines, "\n")

	// Apply border style with fixed width and minimum height
	inputWidth := m.width - 4                  // Match the chat width calculation
	minHeight := max(3, len(processedLines)+2) // At least 3 lines, or content + padding
	return InputStyle.Width(inputWidth).Height(minHeight).Render(content)
}

// renderHelpMessages renders contextual help text below the input
func (m *Model) renderHelpMessages() string {
	var helpText string

	if m.autocomplete.IsActive() {
		helpText = "↑↓ navigate • Enter/Tab to select • Esc to cancel"
	} else if m.isThinking {
		helpText = "AI is responding... • Esc to interrupt"
	} else if len(m.messages) <= 1 { // Only welcome message
		helpText = "? for shortcuts"
	} else {
		helpText = "? for shortcuts"
	}

	return HelpStyle.Render(helpText)
}

// renderPermissionPrompt renders the permission prompt overlay
func (m *Model) renderPermissionPrompt() string {
	if m.pendingPermission == nil {
		return ""
	}

	promptStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FFD700")). // Gold border for attention
		Padding(1, 2).
		Margin(1, 0).
		Background(lipgloss.Color("#1a1a1a")). // Dark background
		Width(m.width - 4)

	var content strings.Builder

	// Header with risk level
	riskColor := "#FFD700" // Default yellow
	switch m.pendingPermission.RiskLevel {
	case "low":
		riskColor = "#90EE90" // Light green
	case "medium":
		riskColor = "#FFD700" // Gold
	case "high":
		riskColor = "#FF6B6B" // Red
	}

	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color(riskColor)).
		Bold(true).
		Render(fmt.Sprintf("🛡️  Permission Required - %s Risk", strings.ToUpper(m.pendingPermission.RiskLevel)))

	content.WriteString(header)
	content.WriteString("\n\n")

	// Tool description
	content.WriteString(m.pendingPermission.PromptText)

	// Risk reasons if available
	if len(m.pendingPermission.RiskReasons) > 0 {
		content.WriteString("\n\n⚠️  Warnings:")
		for _, reason := range m.pendingPermission.RiskReasons {
			content.WriteString(fmt.Sprintf("\n  • %s", reason))
		}
	}

	// Affected files if available
	if len(m.pendingPermission.AffectedFiles) > 0 {
		content.WriteString("\n\n📁 Affected resources:")
		for _, file := range m.pendingPermission.AffectedFiles {
			content.WriteString(fmt.Sprintf("\n  • %s", file))
		}
	}

	// Queue information if multiple items
	if m.pendingPermission.TotalQueued > 1 {
		queueInfo := fmt.Sprintf("\n\n📋 Queue: %d of %d tools pending approval",
			m.pendingPermission.QueuePosition, m.pendingPermission.TotalQueued)
		content.WriteString(queueInfo)
	}

	// Response instructions with enhanced options
	content.WriteString("\n\n")
	instructions := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#87CEEB")). // Sky blue
		Render("🔑 (y) Approve  •  🚫 (n) Deny  •  🔒 (a) Approve & Remember  •  ⏎ (esc) Cancel")
	content.WriteString(instructions)

	return promptStyle.Render(content.String())
}

// handleStreamChunk processes streaming chunks
func (m *Model) handleStreamChunk(msg StreamChunkMsg) {
	if msg.Chunk.Content != "" {
		// Update the last streaming message, or create a new one if none exists
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Streaming {
			m.messages[len(m.messages)-1].Content += msg.Chunk.Content
		} else {
			// No streaming message exists, create a new one (for follow-up responses)
			m.addMessage(ChatMessage{
				Role:      "assistant",
				Content:   msg.Chunk.Content,
				Timestamp: time.Now(),
				Streaming: true,
			})
		}
	}

	// Track and display tool calls
	if msg.Chunk.ToolCall != nil {
		m.toolCount++

		loggy.Debug("Tool call detected", "tool_id", msg.Chunk.ToolCall.ID, "tool_name", msg.Chunk.ToolCall.Name, "input_args", msg.Chunk.ToolCall.Input)

		// Don't show tool start message here - it will be shown when execution actually starts
		// This is because streaming providers may not have complete arguments yet
	}

	// Handle tool start and completion notifications
	if msg.Chunk.ToolCompletion != nil {
		loggy.Debug("Tool notification received", "tool_name", msg.Chunk.ToolCompletion.ToolName, "state", msg.Chunk.ToolCompletion.State, "chunk_type", msg.Chunk.Type, "task_group", msg.Chunk.ToolCompletion.TaskGroup)

		switch msg.Chunk.ToolCompletion.State {
		case "task_start":
			// Handle task group start
			m.addTaskGroupMessage(msg.Chunk.ToolCompletion.Args["task_name"].(string))
		case "start":
			m.addToolMessageWithTask(
				msg.Chunk.ToolCompletion.ToolName,
				msg.Chunk.ToolCompletion.Args,
				"start",
				"",
				msg.Chunk.ToolCompletion.TaskGroup,
			)
		case "error":
			m.addToolMessageWithTask(
				msg.Chunk.ToolCompletion.ToolName,
				msg.Chunk.ToolCompletion.Args,
				"error",
				msg.Chunk.ToolCompletion.Error,
				msg.Chunk.ToolCompletion.TaskGroup,
			)
		case "complete":
			m.addToolMessageWithTask(
				msg.Chunk.ToolCompletion.ToolName,
				msg.Chunk.ToolCompletion.Args,
				"complete",
				msg.Chunk.ToolCompletion.Result,
				msg.Chunk.ToolCompletion.TaskGroup,
			)
		}
	}

	// Always force refresh to show updated content
	m.viewport.GotoBottom()

	// Store stream channel on first chunk (set by StreamStartMsg)

	// Auto-scroll to bottom
	m.viewport.GotoBottom()
}

// handleStreamComplete finalizes streaming response
func (m *Model) handleStreamComplete() {
	// Mark streaming as complete
	if len(m.messages) > 0 && m.messages[len(m.messages)-1].Streaming {
		m.messages[len(m.messages)-1].Streaming = false
		// Estimate output tokens from response content
		responseContent := m.messages[len(m.messages)-1].Content
		m.outputTokens = len(responseContent) / 4
		if m.outputTokens < 1 && responseContent != "" {
			m.outputTokens = 1
		}
	}

	m.isThinking = false
	m.currentStream = nil
}

// renderChatContent renders the chat messages with enhanced formatting
func (m *Model) renderChatContent() string {
	if len(m.messages) == 0 {
		return ""
	}

	var content []string
	for i, msg := range m.messages {
		var renderedMsg string

		switch msg.Role {
		case "user":
			bullet := lipgloss.NewStyle().
				Foreground(TextMuted).
				Render("• ")

			messageContent := lipgloss.NewStyle().
				Foreground(TextPrimary).
				Render(msg.Content)

			renderedMsg = bullet + messageContent

		case "assistant":
			messageContent := msg.Content

			// Apply glamour markdown rendering with built-in syntax highlighting
			if m.glamourRenderer != nil {
				if rendered, err := m.glamourRenderer.Render(messageContent); err == nil {
					messageContent = rendered
				}
				// If rendering fails, continue with original content
			}

			lines := strings.Split(messageContent, "\n")
			var formattedLines []string

			for j, line := range lines {
				if strings.TrimSpace(line) == "" {
					formattedLines = append(formattedLines, "")
					continue
				}

				// Add bullet to non-empty lines
				if j == 0 || (j > 0 && strings.TrimSpace(lines[j-1]) == "") {
					// First line of a paragraph gets a bullet
					bullet := lipgloss.NewStyle().Foreground(AccentColor).Render("• ")
					formattedLines = append(formattedLines, bullet+line)
				} else {
					// Continuation lines get proper indentation
					formattedLines = append(formattedLines, "  "+line)
				}
			}

			// Clean styling without borders or heavy formatting
			styledContent := lipgloss.NewStyle().
				Foreground(TextPrimary).
				Render(strings.Join(formattedLines, "\n"))

			renderedMsg = styledContent

			// Streaming cursor removed - no longer needed

		case "system":
			// System messages - keep minimal styling
			content := msg.Content

			// Clean icons without heavy styling
			if strings.HasPrefix(content, "✅") {
				content = strings.Replace(content, "✅", "✓", 1)
			} else if strings.HasPrefix(content, "❌") {
				content = strings.Replace(content, "❌", "✗", 1)
			} else if strings.HasPrefix(content, "🔧") {
				content = strings.Replace(content, "🔧", "⚡", 1)
			}

			renderedMsg = lipgloss.NewStyle().
				Foreground(TextMuted).
				Italic(true).
				Render(content)

		case "tool":
			// Tool messages are already pre-formatted
			renderedMsg = msg.Content

			// If it's a tool result with detailed content, render it with proper formatting
			if msg.ToolState == "result" && len(msg.Content) > 0 {
				// Add some indentation to tool results
				indentedLines := []string{}
				for _, line := range strings.Split(msg.Content, "\n") {
					indentedLines = append(indentedLines, "  "+line)
				}

				renderedMsg = lipgloss.NewStyle().
					Foreground(TextSecondary).
					Render(strings.Join(indentedLines, "\n"))
			}
		}

		content = append(content, renderedMsg)

		if i < len(m.messages)-1 {
			content = append(content, "") // Empty line for spacing
		}
	}

	return strings.Join(content, "\n")
}

// addMessage adds a message to the chat
func (m *Model) addMessage(msg ChatMessage) {
	m.messages = append(m.messages, msg)
	m.viewport.GotoBottom()
	loggy.Debug("UI message added",
		"component", "addMessage",
		"role", msg.Role,
		"content_length", len(msg.Content),
		"streaming", msg.Streaming,
		"is_tool_msg", msg.IsToolMsg,
		"tool_state", msg.ToolState)
}

// addTaskGroupMessage adds a task group header message
func (m *Model) addTaskGroupMessage(taskName string) {
	content := fmt.Sprintf("Task(%s)", taskName)
	m.addMessage(ChatMessage{
		Role:      "system",
		Content:   content,
		Timestamp: time.Now(),
		IsToolMsg: true,
		TaskGroup: taskName,
		ToolState: "task_start",
	})
}

// addToolMessageWithTask adds a tool execution message with optional task group
func (m *Model) addToolMessageWithTask(toolName string, args map[string]interface{}, state string, result string, taskGroup string) {
	var content string

	switch state {
	case "start":
		// Show simple "• ToolName (target)" format with indentation if part of task group
		if taskGroup != "" {
			content = "  " + m.formatToolStart(toolName, args)
		} else {
			content = m.formatToolStart(toolName, args)
		}
	case "complete":
		// Show result summary like "Read 665 lines (ctrl+r to expand)" with indentation if part of task group
		if taskGroup != "" {
			content = "     " + m.formatToolComplete(toolName, args, result)
		} else {
			content = m.formatToolComplete(toolName, args, result)
		}
	case "error":
		// Show error message with indentation if part of task group
		if taskGroup != "" {
			content = "     " + m.formatToolError(toolName, args, result)
		} else {
			content = m.formatToolError(toolName, args, result)
		}
	}

	if content != "" {
		m.addMessage(ChatMessage{
			Role:      "system",
			Content:   content,
			Timestamp: time.Now(),
			IsToolMsg: true,
			ToolName:  toolName,
			ToolArgs:  args,
			ToolState: state,
			TaskGroup: taskGroup,
		})
	}
}

// formatToolStart formats tool start message
func (m *Model) formatToolStart(toolName string, args map[string]interface{}) string {
	loggy.Debug("formatToolStart", "tool_name", toolName, "args", args)

	// Get the colored dot for tool start (blue/cyan)
	dot := lipgloss.NewStyle().Foreground(AccentColor).Render("⏺")

	switch toolName {
	case "read_file":
		if filePath, ok := args["file_path"].(string); ok {
			filename := m.getDisplayPath(filePath)
			loggy.Debug("formatToolStart read_file", "file_path", filePath, "display_name", filename)
			return fmt.Sprintf("%s Read(%s)", dot, filename)
		}
		loggy.Debug("formatToolStart read_file", "file_path_missing", true, "args_keys", getMapKeys(args))
		return fmt.Sprintf("%s Read(file)", dot)
	case "write_file":
		if filePath, ok := args["file_path"].(string); ok {
			filename := m.getDisplayPath(filePath)
			return fmt.Sprintf("%s Write(%s)", dot, filename)
		}
		return fmt.Sprintf("%s Write(file)", dot)
	case "create_file":
		if filePath, ok := args["file_path"].(string); ok {
			filename := m.getDisplayPath(filePath)
			return fmt.Sprintf("%s Create(%s)", dot, filename)
		}
		return fmt.Sprintf("%s Create(file)", dot)
	case "edit_file":
		if filePath, ok := args["file_path"].(string); ok {
			filename := m.getDisplayPath(filePath)
			return fmt.Sprintf("%s Edit(%s)", dot, filename)
		}
		return fmt.Sprintf("%s Edit(file)", dot)
	case "multi_edit_file":
		if filePath, ok := args["file_path"].(string); ok {
			filename := m.getDisplayPath(filePath)
			return fmt.Sprintf("%s Edit(%s)", dot, filename)
		}
		return fmt.Sprintf("%s Edit(file)", dot)
	case "bash":
		if command, ok := args["command"].(string); ok {
			// Show first word of command
			words := strings.Fields(command)
			if len(words) > 0 {
				return fmt.Sprintf("%s Run(%s)", dot, words[0])
			}
		}
		return fmt.Sprintf("%s Run(command)", dot)
	case "grep":
		if pattern, ok := args["pattern"].(string); ok {
			return fmt.Sprintf("%s Search('%s')", dot, pattern)
		}
		return fmt.Sprintf("%s Search", dot)
	case "list_files":
		if path, ok := args["path"].(string); ok {
			displayPath := m.getDisplayPath(path)
			return fmt.Sprintf("%s List(%s)", dot, displayPath)
		}
		return fmt.Sprintf("%s List(files)", dot)
	case "find":
		if name, ok := args["name"].(string); ok {
			return fmt.Sprintf("%s Find('%s')", dot, name)
		}
		return fmt.Sprintf("%s Find", dot)
	case "fuzzy_search":
		if query, ok := args["query"].(string); ok {
			return fmt.Sprintf("%s Search('%s')", dot, query)
		}
		return fmt.Sprintf("%s Search", dot)
	case "git_status":
		return fmt.Sprintf("%s Git(status)", dot)
	case "git_diff":
		return fmt.Sprintf("%s Git(diff)", dot)
	case "git_add":
		return fmt.Sprintf("%s Git(add)", dot)
	case "git_commit":
		return fmt.Sprintf("%s Git(commit)", dot)
	case "git_log":
		return fmt.Sprintf("%s Git(log)", dot)
	case "git_branch":
		return fmt.Sprintf("%s Git(branch)", dot)
	case "todo_read":
		return fmt.Sprintf("%s Todo(read)", dot)
	case "todo_write":
		return fmt.Sprintf("%s Todo(write)", dot)
	case "web_fetch":
		if url, ok := args["url"].(string); ok {
			return fmt.Sprintf("%s Web(%s)", dot, url)
		}
		return fmt.Sprintf("%s Web(fetch)", dot)
	case "move_file":
		if sourcePath, ok := args["source_path"].(string); ok {
			filename := m.getDisplayPath(sourcePath)
			return fmt.Sprintf("%s Move(%s)", dot, filename)
		}
		return fmt.Sprintf("%s Move(file)", dot)
	case "copy_file":
		if sourcePath, ok := args["source_path"].(string); ok {
			filename := m.getDisplayPath(sourcePath)
			return fmt.Sprintf("%s Copy(%s)", dot, filename)
		}
		return fmt.Sprintf("%s Copy(file)", dot)
	case "delete_file":
		if filePath, ok := args["file_path"].(string); ok {
			filename := m.getDisplayPath(filePath)
			return fmt.Sprintf("%s Delete(%s)", dot, filename)
		}
		return fmt.Sprintf("%s Delete(file)", dot)
	case "create_dir":
		if dirPath, ok := args["dir_path"].(string); ok {
			dirname := m.getDisplayPath(dirPath)
			return fmt.Sprintf("%s Create(%s)", dot, dirname)
		}
		return fmt.Sprintf("%s Create(dir)", dot)
	case "delete_dir":
		if dirPath, ok := args["dir_path"].(string); ok {
			dirname := m.getDisplayPath(dirPath)
			return fmt.Sprintf("%s Delete(%s)", dot, dirname)
		}
		return fmt.Sprintf("%s Delete(dir)", dot)
	default:
		return fmt.Sprintf("%s %s", dot, strings.ToUpper(string(toolName[0]))+strings.ToLower(toolName[1:]))
	}
}

func (m *Model) formatToolComplete(toolName string, args map[string]interface{}, result string) string {
	// Get the colored completion indicator (green for success)
	completionDot := lipgloss.NewStyle().Foreground(SuccessColor).Render("⎿")
	indent := "" // No indentation to match format

	switch toolName {
	case "read_file":
		// Count lines in result for display
		lines := strings.Count(result, "\n") + 1
		if result == "" {
			lines = 0
		}
		return fmt.Sprintf("%s%s Read %d lines", indent, completionDot, lines)
	case "write_file":
		lines := strings.Count(result, "\n") + 1
		if result == "" {
			lines = 1 // Written file has at least 1 line
		}
		return fmt.Sprintf("%s%s Wrote %d lines", indent, completionDot, lines)
	case "create_file":
		lines := strings.Count(result, "\n") + 1
		if result == "" {
			lines = 1 // Created file has at least 1 line
		}
		return fmt.Sprintf("%s%s Created %d lines", indent, completionDot, lines)
	case "edit_file", "multi_edit_file":
		if filePath, ok := args["file_path"].(string); ok {
			filename := m.getDisplayPath(filePath)
			return fmt.Sprintf("%s%s Updated %s", indent, completionDot, filename)
		}
		return fmt.Sprintf("%s%s Edit completed", indent, completionDot)
	case "bash":
		if strings.TrimSpace(result) == "" {
			return fmt.Sprintf("%s%s Run completed", indent, completionDot)
		}
		lines := strings.Count(result, "\n") + 1
		return fmt.Sprintf("%s%s Run output (%d lines)", indent, completionDot, lines)
	case "grep":
		lines := strings.Count(result, "\n")
		if lines == 0 && strings.TrimSpace(result) == "" {
			return fmt.Sprintf("%s%s No matches found", indent, completionDot)
		}
		return fmt.Sprintf("%s%s Found %d matches", indent, completionDot, lines+1)
	case "list_files":
		lines := strings.Count(result, "\n")
		if lines == 0 && strings.TrimSpace(result) == "" {
			return fmt.Sprintf("%s%s No files found", indent, completionDot)
		}
		return fmt.Sprintf("%s%s Listed %d items", indent, completionDot, lines+1)
	case "find", "fuzzy_search":
		lines := strings.Count(result, "\n")
		if lines == 0 && strings.TrimSpace(result) == "" {
			return fmt.Sprintf("%s%s No files found", indent, completionDot)
		}
		return fmt.Sprintf("%s%s Found %d files", indent, completionDot, lines+1)
	case "git_status", "git_diff", "git_add", "git_commit", "git_log", "git_branch":
		return fmt.Sprintf("%s%s Git operation completed", indent, completionDot)
	case "todo_read":
		// Special handling for todo_read - show formatted todo list instead of raw JSON
		if m.session != nil {
			todos := m.session.ParseTodoResult(result)
			if len(todos) > 0 {
				formattedList := m.session.FormatTodoList(todos)
				// Return formatted list as the content instead of generic message
				return formattedList
			}
		}
		return fmt.Sprintf("%s%s No todos found", indent, completionDot)
	case "todo_write":
		// Special handling for todo_write - show progress update
		if m.session != nil {
			todos := m.session.ParseTodoResult(result)
			if len(todos) > 0 {
				progressUpdate := m.session.ShowTodoProgress(todos)
				return progressUpdate
			}
		}
		return fmt.Sprintf("%s%s Todo list updated", indent, completionDot)
	case "web_fetch":
		lines := strings.Count(result, "\n")
		return fmt.Sprintf("%s%s Fetched content (%d lines)", indent, completionDot, lines+1)
	case "move_file":
		return fmt.Sprintf("%s%s File moved", indent, completionDot)
	case "copy_file":
		return fmt.Sprintf("%s%s File copied", indent, completionDot)
	case "delete_file":
		return fmt.Sprintf("%s%s File deleted", indent, completionDot)
	case "create_dir":
		return fmt.Sprintf("%s%s Directory created", indent, completionDot)
	case "delete_dir":
		return fmt.Sprintf("%s%s Directory deleted", indent, completionDot)
	default:
		return fmt.Sprintf("%s%s %s completed", indent, completionDot, strings.ToUpper(string(toolName[0]))+strings.ToLower(toolName[1:]))
	}
}

// formatToolError formats tool error message
func (m *Model) formatToolError(toolName string, args map[string]interface{}, errorMsg string) string {
	// Get the colored error indicator (red)
	errorDot := lipgloss.NewStyle().Foreground(ErrorColor).Render("⎿")
	indent := "" // No indentation to match format
	return fmt.Sprintf("%s%s %s failed: %s", indent, errorDot, strings.ToUpper(string(toolName[0]))+strings.ToLower(toolName[1:]), errorMsg)
}

// getDisplayPath returns a display-friendly path (relative to project root if possible)
func (m *Model) getDisplayPath(fullPath string) string {
	if m.session != nil && m.session.RootPath != "" {
		if relPath, err := filepath.Rel(m.session.RootPath, fullPath); err == nil {
			if !strings.HasPrefix(relPath, "..") {
				return relPath
			}
		}
	}
	return filepath.Base(fullPath)
}

// getMapKeys returns the keys of a map for debugging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// updateDimensions updates component dimensions based on window size
func (m *Model) updateDimensions() {
	chatWidth, chatHeight := GetChatDimensions(m.width, m.height)

	m.viewport.Width = chatWidth
	m.viewport.Height = chatHeight
	m.textarea.SetWidth(chatWidth - 4) // Leave space for border padding

	if m.glamourRenderer != nil {
		if newRenderer, err := glamour.NewTermRenderer(
			glamour.WithStylePath("dracula"),
			glamour.WithWordWrap(chatWidth-4), // Match content width
		); err == nil {
			m.glamourRenderer = newRenderer
		}
	}
}

// loadFiles loads files (placeholder for compatibility)
func (m *Model) loadFiles() {
	// Files are managed by session, no UI file list in simple model
}

// isMouseInInputArea determines if mouse coordinates are in the input area
func (m *Model) isMouseInInputArea(mouseY int) bool {
	// Calculate approximate input area position
	// Header + viewport + tool status + status bar + spacing = input area starts around here
	headerHeight := 1
	viewportHeight := m.viewport.Height
	// Tool status is now handled via chat messages (no separate area needed)
	statusBarHeight := 1
	spacingLines := 2 // Empty lines for spacing

	inputAreaStart := headerHeight + viewportHeight + statusBarHeight + spacingLines

	// Input area is roughly 5 lines (textarea height + border + help)
	inputAreaEnd := inputAreaStart + 5

	return mouseY >= inputAreaStart && mouseY <= inputAreaEnd
}

// showHelp displays help message with available shortcuts
// renderShortcutsOverlay renders the minimal shortcuts overlay
func (m *Model) renderShortcutsOverlay() string {
	if !m.showShortcuts {
		return ""
	}

	shortcuts := []string{
		"/ for commands",
		"↑↓ navigate history",
		"Shift+Enter new line",
		"Esc close overlay",
	}

	var items []string
	for _, shortcut := range shortcuts {
		items = append(items, lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a9b1d6")). // Subtle blue-gray
			Padding(0, 1).
			Render(shortcut))
	}

	content := strings.Join(items, "\n")

	// Box style similar to autocomplete
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#928374")). // Gruvbox gray
		Background(lipgloss.Color("#1d2021")).       // Gruvbox dark background
		Padding(0, 1).
		MaxWidth(m.width - 4)

	return boxStyle.Render(content)
}
