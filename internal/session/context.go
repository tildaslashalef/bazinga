package session

import (
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"path/filepath"
	"strings"
	"time"
)

// ContextManager handles intelligent context window management for LLM requests
type ContextManager struct {
	maxTokens      int
	targetTokens   int // Use 80% for safety margin
	estimateTokens func(string) int
}

// NewContextManager creates a new context manager
func NewContextManager(maxTokens int, estimateTokens func(string) int) *ContextManager {
	return &ContextManager{
		maxTokens:      maxTokens,
		targetTokens:   int(float64(maxTokens) * 0.8),
		estimateTokens: estimateTokens,
	}
}

// FileContent represents a file with metadata for context inclusion
type FileContent struct {
	Path         string
	RelativePath string
	Content      string
	Size         int
	LastModified time.Time
	Relevance    float64 // 0-1 score for how relevant this file is
	Sections     []FileSection
}

// FileSection represents a relevant section of a file
type FileSection struct {
	StartLine int
	EndLine   int
	Content   string
	Type      string // "function", "class", "imports", "relevant"
	Relevance float64
}

// ConversationMessage extends llm.Message with metadata
type ConversationMessage struct {
	llm.Message
	Timestamp   time.Time
	TokenCount  int
	Importance  float64 // 0-1 score for how important this message is
	Summary     string  // Compressed version for long sessions
	HasToolCall bool
}

// BuildOptimizedContext builds an optimized context for the LLM request
func (cm *ContextManager) BuildOptimizedContext(
	session *Session,
	history []llm.Message,
	currentMessage string,
) ([]llm.Message, error) {
	if session == nil {
		return nil, fmt.Errorf("cannot build context with nil session")
	}

	// Calculate token budget
	currentTokens := cm.estimateTokens(currentMessage)
	systemMsg := cm.buildEnhancedSystemMessage(session)
	messages := []llm.Message{systemMsg}

	// Extract system content for token calculation
	systemContent, ok := systemMsg.Content.(string)
	if !ok {
		systemContent = ""
		loggy.Warn("BuildOptimizedContext: Failed to extract system message content")
	}
	currentTokens += cm.estimateTokens(systemContent)

	// Add conversation history with intelligent pruning
	historyMessages := cm.pruneConversationHistory(history, cm.targetTokens-currentTokens)
	messages = append(messages, historyMessages...)

	loggy.Debug("BuildOptimizedContext completed",
		"total_messages", len(messages),
		"system_tokens", cm.estimateTokens(systemContent),
		"history_messages", len(historyMessages),
		"target_tokens", cm.targetTokens,

		"current_tokens", currentTokens)

	return messages, nil
}

func (cm *ContextManager) buildEnhancedSystemMessage(session *Session) llm.Message {
	var content strings.Builder

	// Use natural Bazinga prompt from prompts.go if available
	if session != nil {
		content.WriteString(session.buildBazingaPrompt())
		content.WriteString("\n\n")
	} else {
		// Fallback system prompt
		content.WriteString(`You are bazinga, an AI-powered coding assistant with powerful file editing capabilities.

CRITICAL: When users ask you to add, modify, create, or change code, you MUST use the appropriate tools to make those changes.
- NEVER just describe what changes to make - always execute the tools to make actual changes
- Always read files first with read_file before making changes
- Use edit_file, write_file, or create_file to implement the requested changes
- Take action with tools rather than just providing suggestions

Available Tools:
- read_file: Read file contents to understand current state
- edit_file: Replace specific text in a file (use this for modifications)
- write_file: Create or completely overwrite a file
- create_file: Create a new file (fails if file already exists)
- bash: Execute shell commands
- grep: Search for patterns in files
- find: Find files matching criteria
- list_files: List directory contents

`)
	}

	// Add session context if available
	if session != nil {
		cm.addSessionContext(&content, session)
	}

	return llm.Message{
		Role:    "system",
		Content: content.String(),
	}
}

// addSessionContext adds session-specific context to the system message
func (cm *ContextManager) addSessionContext(content *strings.Builder, session *Session) {
	// Add project context
	if session.project != nil {
		fmt.Fprintf(content, `Project Context:
- Type: %s
- Name: %s
- Total Files: %d
- Directories: %d

`, session.project.Type, session.project.Name,
			len(session.project.Files), len(session.project.Directories))
	}

	// Add session info
	fileCount := 0
	if session.Files != nil {
		fileCount = len(session.Files)
	}

	provider := session.Provider
	if provider == "" {
		provider = "default"
	}

	model := session.Model
	if model == "" {
		model = "default"
	}

	rootPath := session.RootPath
	if rootPath == "" {
		rootPath = "current directory"
	}

	fmt.Fprintf(content, `Session Info:
- Files in session: %d
- Provider: %s
- Model: %s
- Root path: %s

`, fileCount, provider, model, rootPath)

	// Add memory content if available
	if session.memoryContent != nil && session.memoryContent.FullContent != "" {
		content.WriteString("Memory System:\n")
		content.WriteString(session.memoryContent.FullContent)
		content.WriteString("\n\n")
	}

	if len(session.Files) > 0 {
		content.WriteString("Current files in session:\n")
		for _, file := range session.Files {
			relPath, err := filepath.Rel(session.RootPath, file)
			if err != nil {
				relPath = filepath.Base(file)
			}
			fmt.Fprintf(content, "- %s\n", relPath)
		}
		content.WriteString("\n")
	}

	content.WriteString("Remember to use tools to read files before making changes, and always maintain the existing code structure and style.")
}

// pruneConversationHistory intelligently prunes conversation history to fit token budget
func (cm *ContextManager) pruneConversationHistory(history []llm.Message, tokenBudget int) []llm.Message {
	if len(history) == 0 {
		return history
	}

	loggy.Debug("pruneConversationHistory starting",
		"input_messages", len(history),
		"token_budget", tokenBudget)

	// Convert to conversation messages with metadata
	convMessages := make([]ConversationMessage, len(history))
	for i, msg := range history {
		content, _ := msg.Content.(string)
		convMessages[i] = ConversationMessage{
			Message:     msg,
			Timestamp:   time.Now().Add(-time.Duration(len(history)-i) * time.Minute),
			TokenCount:  cm.estimateTokens(content),
			Importance:  cm.calculateMessageImportance(msg, i, len(history)),
			HasToolCall: cm.hasToolCall(content),
		}
	}

	// Keep most recent messages first
	var result []llm.Message
	usedTokens := 0

	// Add from end (most recent) first
	for i := len(convMessages) - 1; i >= 0; i-- {
		msg := convMessages[i]
		if usedTokens+msg.TokenCount <= tokenBudget {
			result = append([]llm.Message{msg.Message}, result...)
			usedTokens += msg.TokenCount
			loggy.Debug("pruneConversationHistory kept message",
				"index", i,
				"role", msg.Role,
				"tokens", msg.TokenCount,
				"total_used", usedTokens)
		} else {
			loggy.Debug("pruneConversationHistory excluded message",
				"index", i,
				"role", msg.Role,
				"tokens", msg.TokenCount,
				"would_exceed_budget", usedTokens+msg.TokenCount)
			break
		}
	}

	// Add summary if we have room and skipped important messages
	if len(result) < len(history) && usedTokens < tokenBudget {
		summary := cm.createConversationSummary(convMessages[:len(history)-len(result)])
		if summary != "" {
			summaryTokens := cm.estimateTokens(summary)
			if usedTokens+summaryTokens <= tokenBudget {
				summaryMsg := llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("Previous conversation summary: %s", summary),
				}
				result = append([]llm.Message{summaryMsg}, result...)
			}
		}
	}

	loggy.Debug("pruneConversationHistory completed",
		"input_messages", len(history),
		"output_messages", len(result),
		"used_tokens", usedTokens,
		"budget", tokenBudget)

	return result
}

// calculateMessageImportance scores how important a message is to keep
func (cm *ContextManager) calculateMessageImportance(msg llm.Message, index, total int) float64 {
	importance := 0.0
	content, _ := msg.Content.(string)

	// Recent messages are more important
	recencyScore := float64(index) / float64(total)
	importance += recencyScore * 0.4

	// Assistant messages with tool calls are important
	if msg.Role == "assistant" && cm.hasToolCall(content) {
		importance += 0.3
	}

	// Error messages are important
	if strings.Contains(strings.ToLower(content), "error") || strings.Contains(content, "❌") {
		importance += 0.2
	}

	// Tool result messages are important
	if strings.Contains(content, "<tool_result") {
		importance += 0.3
	}

	// Longer messages might be more important
	if len(content) > 200 {
		importance += 0.1
	}

	return importance
}

// hasToolCall checks if a message contains tool call information
func (cm *ContextManager) hasToolCall(content string) bool {
	return strings.Contains(content, "✅ Tool") ||
		strings.Contains(content, "❌ Tool") ||
		strings.Contains(content, "<tool_result") ||
		strings.Contains(content, "⚡")
}

// createConversationSummary creates a summary of conversation messages
func (cm *ContextManager) createConversationSummary(messages []ConversationMessage) string {
	if len(messages) == 0 {
		return ""
	}

	var keyPoints []string

	for _, msg := range messages {
		content, _ := msg.Content.(string)

		// Extract key information
		if msg.HasToolCall && msg.Role == "assistant" {
			if strings.Contains(content, "✅") {
				keyPoints = append(keyPoints, "Successful file operation")
			}
		}

		// Extract file mentions
		if strings.Contains(content, ".go") || strings.Contains(content, ".js") ||
			strings.Contains(content, ".py") || strings.Contains(content, ".java") {
			keyPoints = append(keyPoints, "Discussed specific files")
		}

		// Extract error mentions
		if strings.Contains(strings.ToLower(content), "error") {
			keyPoints = append(keyPoints, "Encountered and resolved errors")
		}

		// Extract tool results
		if strings.Contains(content, "<tool_result") {
			keyPoints = append(keyPoints, "Tool execution results")
		}
	}

	if len(keyPoints) == 0 {
		return fmt.Sprintf("Previous %d messages covered general coding discussion", len(messages))
	}

	return fmt.Sprintf("Previous %d messages: %s", len(messages), strings.Join(keyPoints, ", "))
}
