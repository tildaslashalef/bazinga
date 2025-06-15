package ui

import (
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ToolExecutionStatus represents a tool execution in progress or completed
type ToolExecutionStatus struct {
	ID             string
	Name           string
	Status         string // "starting", "reading", "writing", "running", "completed", "error"
	File           string
	Args           map[string]interface{}
	StartTime      time.Time
	EndTime        time.Time
	Error          error
	Result         string
	HideCompletion bool // If true, don't show this tool in completed status
}

// ToolDisplayState manages tool execution display state
type ToolDisplayState struct {
	ActiveTools    map[string]*ToolExecutionStatus
	CompletedTools []*ToolExecutionStatus
	ShowCompleted  bool
}

// NewToolDisplayState creates a new tool display state
func NewToolDisplayState() *ToolDisplayState {
	return &ToolDisplayState{
		ActiveTools:   make(map[string]*ToolExecutionStatus),
		ShowCompleted: true,
	}
}

// StartToolExecution adds a new tool execution to active status
func (tds *ToolDisplayState) StartToolExecution(toolCall *llm.ToolCall) {
	tds.StartToolExecutionWithOptions(toolCall, false)
}

// StartToolExecutionWithOptions adds a new tool execution with visibility options
func (tds *ToolDisplayState) StartToolExecutionWithOptions(toolCall *llm.ToolCall, hideCompletion bool) {
	status := &ToolExecutionStatus{
		ID:             toolCall.ID,
		Name:           toolCall.Name,
		Status:         "starting",
		Args:           toolCall.Input,
		StartTime:      time.Now(),
		HideCompletion: hideCompletion,
	}

	if filePath, ok := toolCall.Input["file_path"].(string); ok {
		// Try to make it relative to current working directory
		if cwd, err := os.Getwd(); err == nil {
			if relPath, err := filepath.Rel(cwd, filePath); err == nil && !strings.HasPrefix(relPath, "..") {
				status.File = relPath
			} else {
				status.File = filepath.Base(filePath)
			}
		} else {
			status.File = filepath.Base(filePath)
		}
	} else if command, ok := toolCall.Input["command"].(string); ok {
		// For bash commands, show the command
		status.File = command
	} else if pattern, ok := toolCall.Input["pattern"].(string); ok {
		// For grep/search operations
		status.File = pattern
	} else if url, ok := toolCall.Input["url"].(string); ok {
		// For web fetch operations
		status.File = url
	}

	// Update status based on tool type
	switch toolCall.Name {
	case "read_file":
		status.Status = "reading"
		// Hide read_file completions by default since they're usually automatic
		if !hideCompletion {
			status.HideCompletion = true
		}
	case "write_file", "create_file":
		status.Status = "writing"
	case "edit_file":
		status.Status = "editing"
	case "bash":
		status.Status = "running"
	case "grep", "find":
		status.Status = "searching"
	case "web_fetch":
		status.Status = "fetching"
	case "list_files":
		status.Status = "listing"
	default:
		status.Status = "executing"
	}

	tds.ActiveTools[toolCall.ID] = status
}

// CompleteToolExecution moves a tool from active to completed
func (tds *ToolDisplayState) CompleteToolExecution(toolID, result string, err error) {
	if status, exists := tds.ActiveTools[toolID]; exists {
		status.EndTime = time.Now()
		status.Result = result
		status.Error = err

		if err != nil {
			status.Status = "error"
		} else {
			status.Status = "completed"
		}

		// Move to completed and remove from active
		tds.CompletedTools = append(tds.CompletedTools, status)
		delete(tds.ActiveTools, toolID)

		// Keep only recent completed tools
		if len(tds.CompletedTools) > 10 {
			tds.CompletedTools = tds.CompletedTools[1:]
		}
	}
}

// Helper methods for status bar compatibility

// HasActiveTools returns true if there are any active tools
func (tds *ToolDisplayState) HasActiveTools() bool {
	return len(tds.ActiveTools) > 0
}

// GetActiveToolsCount returns the number of active tools
func (tds *ToolDisplayState) GetActiveToolsCount() int {
	return len(tds.ActiveTools)
}

// GetFirstActiveToolForStatusBar returns the first active tool for status bar display
func (tds *ToolDisplayState) GetFirstActiveToolForStatusBar() *ToolExecutionStatus {
	if len(tds.ActiveTools) == 0 {
		return nil
	}

	// Since maps are unordered, we need to find the earliest started tool
	var earliest *ToolExecutionStatus
	for _, tool := range tds.ActiveTools {
		if earliest == nil || tool.StartTime.Before(earliest.StartTime) {
			earliest = tool
		}
	}
	return earliest
}

// RenderToolStatus renders the current tool execution status
func (tds *ToolDisplayState) RenderToolStatus() string {
	if len(tds.ActiveTools) == 0 && (!tds.ShowCompleted || len(tds.CompletedTools) == 0) {
		return ""
	}

	var statusLines []string

	// Render active tools
	for _, tool := range tds.ActiveTools {
		line := tds.renderToolLine(tool, true)
		if line != "" {
			statusLines = append(statusLines, line)
		}
	}

	// Render recently completed tools if showing them
	if tds.ShowCompleted {
		for i := len(tds.CompletedTools) - 3; i < len(tds.CompletedTools); i++ {
			if i >= 0 && !tds.CompletedTools[i].HideCompletion {
				line := tds.renderToolLine(tds.CompletedTools[i], false)
				if line != "" {
					statusLines = append(statusLines, line)
				}
			}
		}
	}

	if len(statusLines) == 0 {
		return ""
	}

	// Wrap in a subtle container
	content := strings.Join(statusLines, "\n")
	return lipgloss.NewStyle().
		Foreground(TextSecondary).
		PaddingLeft(1).
		Render(content)
}

// renderToolLine renders a single tool execution line
func (tds *ToolDisplayState) renderToolLine(tool *ToolExecutionStatus, isActive bool) string {
	if tool == nil {
		return ""
	}

	var icon, actionText string
	var style lipgloss.Style

	// Choose icon and action text based on tool status
	switch tool.Status {
	case "starting":
		icon = "🚀"
		actionText = "Starting"
		style = lipgloss.NewStyle().Foreground(WarningColor)
	case "reading":
		icon = "📖"
		actionText = "Read"
		style = lipgloss.NewStyle().Foreground(AccentColor)
	case "writing":
		icon = "✏️"
		actionText = "Write"
		style = lipgloss.NewStyle().Foreground(AccentColor)
	case "editing":
		icon = "📝"
		actionText = "Edit"
		style = lipgloss.NewStyle().Foreground(AccentColor)
	case "running":
		icon = "⚡"
		actionText = "Run"
		style = lipgloss.NewStyle().Foreground(AccentColor)
	case "searching":
		icon = "🔍"
		actionText = "Search"
		style = lipgloss.NewStyle().Foreground(AccentColor)
	case "fetching":
		icon = "🌐"
		actionText = "Fetching"
		style = lipgloss.NewStyle().Foreground(AccentColor)
	case "listing":
		icon = "📋"
		actionText = "Listing"
		style = lipgloss.NewStyle().Foreground(AccentColor)
	case "executing":
		icon = "🔧"
		actionText = "Executing"
		style = lipgloss.NewStyle().Foreground(AccentColor)
	case "completed":
		icon = "✅"
		actionText = GetToolActionName(tool.Name) // Show specific action instead of generic "Completed"
		style = lipgloss.NewStyle().Foreground(SuccessColor)
	case "error":
		icon = "❌"
		actionText = "Failed"
		style = lipgloss.NewStyle().Foreground(ErrorColor)
	default:
		icon = "•"
		actionText = tool.Status
		style = lipgloss.NewStyle().Foreground(TextSecondary)
	}

	var displayText string
	if tool.File != "" {
		displayText = fmt.Sprintf("%s(%s)", actionText, tool.File)
	} else {
		// Fallback to just the action
		displayText = actionText
	}

	// Add timing for completed tools
	if !isActive && !tool.EndTime.IsZero() {
		duration := tool.EndTime.Sub(tool.StartTime)
		if duration > 0 {
			displayText += fmt.Sprintf(" (%s)", formatDuration(duration))
		}
	}

	// Render with icon and styling
	return style.Render(fmt.Sprintf("%s %s", icon, displayText))
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// GetToolActionName returns a user-friendly action name for a tool
func GetToolActionName(toolName string) string {
	switch toolName {
	case "read_file":
		return "Read"
	case "write_file":
		return "Write"
	case "create_file":
		return "Create"
	case "edit_file":
		return "Edit"
	case "move_file":
		return "Move"
	case "copy_file":
		return "Copy"
	case "delete_file":
		return "Delete"
	case "create_dir":
		return "Create directory"
	case "delete_dir":
		return "Delete directory"
	case "bash":
		return "Run"
	case "grep":
		return "Search"
	case "find":
		return "Find"
	case "list_files":
		return "Listing"
	case "web_fetch":
		return "Fetching"
	case "todo_read":
		return "Reading todos"
	case "todo_write":
		return "Updating todos"
	case "git_status":
		return "Git status"
	case "git_diff":
		return "Git diff"
	case "git_add":
		return "Git add"
	case "git_commit":
		return "Git commit"
	case "git_log":
		return "Git log"
	case "git_branch":
		return "Git branch"
	default:
		return "Executing"
	}
}

// GetToolDisplayFile returns the display name for the file/target of a tool operation
func GetToolDisplayFile(toolName string, args map[string]interface{}) string {
	switch toolName {
	case "read_file", "write_file", "create_file", "edit_file", "delete_file":
		if filePath, ok := args["file_path"].(string); ok {
			return filepath.Base(filePath)
		}
	case "move_file", "copy_file":
		if sourcePath, ok := args["source_path"].(string); ok {
			if destPath, ok2 := args["dest_path"].(string); ok2 {
				return fmt.Sprintf("%s → %s", filepath.Base(sourcePath), filepath.Base(destPath))
			}
			return filepath.Base(sourcePath)
		}
	case "create_dir", "delete_dir":
		if dirPath, ok := args["dir_path"].(string); ok {
			return filepath.Base(dirPath)
		}
	case "bash":
		if command, ok := args["command"].(string); ok {
			// Show first part of command
			words := strings.Fields(command)
			if len(words) > 0 {
				return words[0]
			}
			return "command"
		}
	case "grep":
		if pattern, ok := args["pattern"].(string); ok {
			return fmt.Sprintf("'%s'", pattern)
		}
	case "find":
		if name, ok := args["name"].(string); ok {
			return name
		}
	case "list_files":
		if directory, ok := args["directory"].(string); ok {
			return filepath.Base(directory)
		}
		return "current directory"
	case "web_fetch":
		if url, ok := args["url"].(string); ok {
			return url
		}
	}
	return ""
}
