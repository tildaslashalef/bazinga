package session

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TodoItem represents a todo item for display purposes
type TodoItem struct {
	ID       string
	Content  string
	Status   string // "pending", "in_progress", "completed", "canceled"
	Priority string // "high", "medium", "low"
}

// FormatTodoList creates a visual todo list with checkboxes and progress
func (s *Session) FormatTodoList(todos []TodoItem) string {
	if len(todos) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "📋 **Task Breakdown:**")
	lines = append(lines, "")

	completed := 0
	for _, todo := range todos {
		checkbox, icon := getStatusDisplay(todo.Status)
		priorityIndicator := getPriorityIndicator(todo.Priority)

		line := fmt.Sprintf("- [%s] %s %s%s", checkbox, icon, priorityIndicator, todo.Content)
		lines = append(lines, line)

		if todo.Status == "completed" {
			completed++
		}
	}

	// Add progress summary
	percentage := int((float64(completed) / float64(len(todos))) * 100)
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("**Progress:** %d/%d tasks completed (%d%%)", completed, len(todos), percentage))

	return strings.Join(lines, "\n")
}

// ShowTodoProgress displays completion summary and next steps
func (s *Session) ShowTodoProgress(todos []TodoItem) string {
	if len(todos) == 0 {
		return ""
	}

	completed := 0
	inProgress := 0
	pending := 0
	var nextTask *TodoItem

	for i := range todos {
		todo := &todos[i]
		switch todo.Status {
		case "completed":
			completed++
		case "in_progress":
			inProgress++
		case "pending":
			pending++
			if nextTask == nil {
				nextTask = todo
			}
		}
	}

	var lines []string

	// Progress header
	if completed == len(todos) {
		lines = append(lines, "✨ **All Tasks Complete!**")
	} else {
		lines = append(lines, "📊 **Progress Update:**")
	}
	lines = append(lines, "")

	// Status summary
	percentage := int((float64(completed) / float64(len(todos))) * 100)
	lines = append(lines, fmt.Sprintf("- ✅ Completed: %d", completed))
	if inProgress > 0 {
		lines = append(lines, fmt.Sprintf("- ⏳ In Progress: %d", inProgress))
	}
	if pending > 0 {
		lines = append(lines, fmt.Sprintf("- ⭕ Pending: %d", pending))
	}
	lines = append(lines, fmt.Sprintf("- 📈 Overall: %d%% complete", percentage))

	// Next steps
	if nextTask != nil {
		lines = append(lines, "")
		priorityIndicator := getPriorityIndicator(nextTask.Priority)
		lines = append(lines, fmt.Sprintf("**Next:** %s%s", priorityIndicator, nextTask.Content))
	}

	return strings.Join(lines, "\n")
}

// FormatTodoUpdate shows a brief status update for a single todo change
func (s *Session) FormatTodoUpdate(todo TodoItem, previousStatus string) string {
	if todo.Status == previousStatus {
		return ""
	}

	var icon, message string
	switch todo.Status {
	case "in_progress":
		icon = "🔄"
		message = "Started"
	case "completed":
		icon = "✅"
		message = "Completed"
	case "canceled":
		icon = "❌"
		message = "Canceled"
	default:
		return ""
	}

	priorityIndicator := getPriorityIndicator(todo.Priority)
	return fmt.Sprintf("%s **%s:** %s%s", icon, message, priorityIndicator, todo.Content)
}

// CreateQuickTodoSummary provides a one-line summary of todo status
func (s *Session) CreateQuickTodoSummary(todos []TodoItem) string {
	if len(todos) == 0 {
		return ""
	}

	completed := 0
	for _, todo := range todos {
		if todo.Status == "completed" {
			completed++
		}
	}

	if completed == len(todos) {
		return "✨ All tasks completed!"
	}

	percentage := int((float64(completed) / float64(len(todos))) * 100)
	return fmt.Sprintf("📋 Progress: %d/%d tasks (%d%%)", completed, len(todos), percentage)
}

// getStatusDisplay returns checkbox and icon for todo status
func getStatusDisplay(status string) (checkbox, icon string) {
	switch status {
	case "completed":
		return "x", "✅"
	case "in_progress":
		return " ", "⏳"
	case "canceled":
		return " ", "❌"
	default: // pending
		return " ", "⭕"
	}
}

// getPriorityIndicator returns priority indicator
func getPriorityIndicator(priority string) string {
	switch priority {
	case "high":
		return "🔥 "
	case "low":
		return "💫 "
	default: // medium
		return ""
	}
}

// ParseTodoResult parses JSON string result from todo tool into TodoItem slice
func (s *Session) ParseTodoResult(result string) []TodoItem {
	// Try to parse the JSON result
	var toolTodos []interface{}
	if err := json.Unmarshal([]byte(result), &toolTodos); err != nil {
		// If parsing fails, return empty slice
		return []TodoItem{}
	}

	return ConvertFromToolTodos(toolTodos)
}

// ConvertFromToolTodos converts tool todo format to display format
func ConvertFromToolTodos(toolTodos interface{}) []TodoItem {
	var todos []TodoItem

	// Handle the actual todo tool response format
	// The tool returns a slice of todo items with id, content, status, priority
	if todoSlice, ok := toolTodos.([]interface{}); ok {
		for _, item := range todoSlice {
			if todoMap, ok := item.(map[string]interface{}); ok {
				todo := TodoItem{}

				if id, ok := todoMap["id"].(string); ok {
					todo.ID = id
				}
				if content, ok := todoMap["content"].(string); ok {
					todo.Content = content
				}
				if status, ok := todoMap["status"].(string); ok {
					todo.Status = status
				}
				if priority, ok := todoMap["priority"].(string); ok {
					todo.Priority = priority
				}

				todos = append(todos, todo)
			}
		}
	}

	return todos
}

// Example usage functions for testing and documentation

// ExampleTodoDisplay shows how the formatting looks
func ExampleTodoDisplay() string {
	_ = []TodoItem{
		{ID: "1", Content: "Analyze current theme structure", Status: "completed", Priority: "high"},
		{ID: "2", Content: "Create dark theme configuration", Status: "in_progress", Priority: "high"},
		{ID: "3", Content: "Add theme toggle component", Status: "pending", Priority: "medium"},
		{ID: "4", Content: "Update existing components", Status: "pending", Priority: "medium"},
		{ID: "5", Content: "Test theme switching", Status: "pending", Priority: "low"},
	}

	var lines []string
	lines = append(lines, "📋 **Task Breakdown:**")
	lines = append(lines, "")
	lines = append(lines, "- [x] ✅ 🔥 Analyze current theme structure")
	lines = append(lines, "- [ ] ⏳ 🔥 Create dark theme configuration")
	lines = append(lines, "- [ ] ⭕ Add theme toggle component")
	lines = append(lines, "- [ ] ⭕ Update existing components")
	lines = append(lines, "- [ ] ⭕ 💫 Test theme switching")
	lines = append(lines, "")
	lines = append(lines, "**Progress:** 1/5 tasks completed (20%)")

	return strings.Join(lines, "\n")
}
