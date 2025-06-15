package tools

import (
	"encoding/json"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/config"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// TodoItem represents a single todo item
type TodoItem struct {
	ID          string     `json:"id"`
	Content     string     `json:"content"`
	Status      string     `json:"status"`   // "pending", "in_progress", "completed", "canceled"
	Priority    string     `json:"priority"` // "high", "medium", "low"
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// TodoManager manages todo items for a session
type TodoManager struct {
	todoFile string
	items    []TodoItem
}

// NewTodoManager creates a new todo manager
func NewTodoManager(rootPath string) *TodoManager {
	var todoFile string

	// Use config directory structure like sessions
	configDir, err := config.GetConfigDir()
	if err != nil {
		loggy.Warn("Could not get config directory, falling back to project directory", "error", err)
		todoFile = filepath.Join(rootPath, ".todos.json")
	} else {
		todosDir := filepath.Join(configDir, "todos")
		// Create todos directory if it doesn't exist
		if err := os.MkdirAll(todosDir, 0o755); err != nil {
			loggy.Warn("Could not create todos directory, falling back to project directory", "error", err)
			todoFile = filepath.Join(rootPath, ".todos.json")
		} else {
			// Use project-specific todo file based on root path
			projectName := filepath.Base(rootPath)
			if projectName == "" || projectName == "." {
				projectName = "default"
			}
			todoFile = filepath.Join(todosDir, fmt.Sprintf("%s_todos.json", projectName))
		}
	}

	manager := &TodoManager{
		todoFile: todoFile,
		items:    make([]TodoItem, 0),
	}

	// Load existing todos
	if err := manager.load(); err != nil {
		loggy.Debug("Could not load existing todos", "error", err, "todo_file", todoFile)
	}

	return manager
}

// load loads todos from the file
func (tm *TodoManager) load() error {
	if _, err := os.Stat(tm.todoFile); os.IsNotExist(err) {
		return nil // No file exists yet, start with empty list
	}

	data, err := os.ReadFile(tm.todoFile)
	if err != nil {
		return fmt.Errorf("failed to read todo file: %w", err)
	}

	if len(data) == 0 {
		return nil // Empty file
	}

	if err := json.Unmarshal(data, &tm.items); err != nil {
		return fmt.Errorf("failed to parse todo file: %w", err)
	}

	return nil
}

// save saves todos to the file
func (tm *TodoManager) save() error {
	data, err := json.MarshalIndent(tm.items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal todos: %w", err)
	}

	if err := os.WriteFile(tm.todoFile, data, 0o644); err != nil {
		return fmt.Errorf("failed to write todo file: %w", err)
	}

	return nil
}

// Read returns all todo items formatted for display
func (tm *TodoManager) Read() (string, error) {
	if err := tm.load(); err != nil {
		return "", err
	}

	if len(tm.items) == 0 {
		return "No todos found. Use todo_write to create some!", nil
	}

	// Sort by status and priority
	sorted := make([]TodoItem, len(tm.items))
	copy(sorted, tm.items)

	sort.Slice(sorted, func(i, j int) bool {
		// Sort by status first (in_progress, pending, completed, canceled)
		statusOrder := map[string]int{
			"in_progress": 0,
			"pending":     1,
			"completed":   2,
			"canceled":    3,
		}

		if statusOrder[sorted[i].Status] != statusOrder[sorted[j].Status] {
			return statusOrder[sorted[i].Status] < statusOrder[sorted[j].Status]
		}

		// Then by priority
		priorityOrder := map[string]int{
			"high":   0,
			"medium": 1,
			"low":    2,
		}

		return priorityOrder[sorted[i].Priority] < priorityOrder[sorted[j].Priority]
	})

	// Format for display
	var result []string
	result = append(result, "📋 Todo List:")
	result = append(result, "")

	statusGroups := map[string][]TodoItem{
		"in_progress": {},
		"pending":     {},
		"completed":   {},
		"canceled":    {},
	}

	for _, item := range sorted {
		statusGroups[item.Status] = append(statusGroups[item.Status], item)
	}

	// Display each group
	statusLabels := map[string]string{
		"in_progress": "◈ In Progress",
		"pending":     "◉ Pending",
		"completed":   "✓ Completed",
		"canceled":    "✗ Canceled",
	}

	statusIcons := map[string]string{
		"high":   "▸",
		"medium": "•",
		"low":    "▫",
	}

	for _, status := range []string{"in_progress", "pending", "completed", "canceled"} {
		items := statusGroups[status]
		if len(items) == 0 {
			continue
		}

		result = append(result, statusLabels[status]+":")
		for _, item := range items {
			icon := statusIcons[item.Priority]
			timeStr := item.CreatedAt.Format("01/02")
			if item.CompletedAt != nil {
				timeStr = item.CompletedAt.Format("01/02")
			}
			idStr := item.ID
			if len(idStr) > 8 {
				idStr = idStr[:8]
			}
			result = append(result, fmt.Sprintf("  %s %s [%s] (%s)", icon, item.Content, idStr, timeStr))
		}
		result = append(result, "")
	}

	// Add summary
	stats := tm.getStats()
	result = append(result, fmt.Sprintf("◉ Summary: %d total, %d pending, %d completed",
		stats["total"], stats["pending"], stats["completed"]))

	return fmt.Sprintf("%s\n", joinLines(result)), nil
}

// Write updates the todo list with new items
func (tm *TodoManager) Write(todosData string) error {
	// Parse the todos data as JSON
	var newItems []TodoItem
	if err := json.Unmarshal([]byte(todosData), &newItems); err != nil {
		return fmt.Errorf("failed to parse todos JSON: %w", err)
	}

	// Validate and update items
	now := time.Now()
	for i := range newItems {
		item := &newItems[i]

		// Validate required fields
		if item.ID == "" {
			return fmt.Errorf("todo item missing required field: id")
		}
		if item.Content == "" {
			return fmt.Errorf("todo item missing required field: content")
		}
		if item.Status == "" {
			item.Status = "pending"
		}
		if item.Priority == "" {
			item.Priority = "medium"
		}

		// Validate enum values
		if !isValidStatus(item.Status) {
			return fmt.Errorf("invalid status: %s (must be pending, in_progress, completed, or canceled)", item.Status)
		}
		if !isValidPriority(item.Priority) {
			return fmt.Errorf("invalid priority: %s (must be high, medium, or low)", item.Priority)
		}

		// Find existing item to preserve timestamps
		var existing *TodoItem
		for j := range tm.items {
			if tm.items[j].ID == item.ID {
				existing = &tm.items[j]
				break
			}
		}

		if existing != nil {
			// Update existing item
			item.CreatedAt = existing.CreatedAt
			item.UpdatedAt = now

			// Handle completion
			if item.Status == "completed" && existing.Status != "completed" {
				item.CompletedAt = &now
			} else if existing.CompletedAt != nil {
				item.CompletedAt = existing.CompletedAt
			}
		} else {
			// New item
			item.CreatedAt = now
			item.UpdatedAt = now

			if item.Status == "completed" {
				item.CompletedAt = &now
			}
		}
	}

	// Replace the entire list
	tm.items = newItems

	return tm.save()
}

// getStats returns statistics about the todos
func (tm *TodoManager) getStats() map[string]int {
	stats := map[string]int{
		"total":       len(tm.items),
		"pending":     0,
		"in_progress": 0,
		"completed":   0,
		"canceled":    0,
	}

	for _, item := range tm.items {
		stats[item.Status]++
	}

	return stats
}

// isValidStatus checks if a status is valid
func isValidStatus(status string) bool {
	valid := []string{"pending", "in_progress", "completed", "canceled"}
	for _, v := range valid {
		if v == status {
			return true
		}
	}
	return false
}

// isValidPriority checks if a priority is valid
func isValidPriority(priority string) bool {
	valid := []string{"high", "medium", "low"}
	for _, v := range valid {
		if v == priority {
			return true
		}
	}
	return false
}

// joinLines joins string lines with newlines
func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		result += line
		if i < len(lines)-1 {
			result += "\n"
		}
	}
	return result
}
