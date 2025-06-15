package tools

import (
	"context"
	"encoding/json"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Mock the config.GetConfigDir function during tests
func setupTodoTest(t *testing.T) (string, string, func()) {
	// Create temporary directories
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".bazinga") // Uses .bazinga, not .config/bazinga
	todosDir := filepath.Join(configDir, "todos")

	err := os.MkdirAll(todosDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create todos directory: %v", err)
	}

	// Save original environment variables to restore later
	origXdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	origHome := os.Getenv("HOME")

	// Override HOME to point to our temp directory
	os.Setenv("HOME", tempDir)

	// Cleanup function to restore environment
	cleanup := func() {
		os.Setenv("XDG_CONFIG_HOME", origXdgConfigHome)
		os.Setenv("HOME", origHome)
	}

	return tempDir, todosDir, cleanup
}

func TestToolExecutor_TodoRead(t *testing.T) {
	tempDir, todosDir, cleanup := setupTodoTest(t)
	defer cleanup()

	// Create ToolExecutor with the temp directory
	te := NewToolExecutor(tempDir)

	// Test reading non-existent todos
	result, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "todo_read",
		Input: map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("todo_read failed: %v", err)
	}

	if !strings.Contains(result, "No todos found") {
		t.Errorf("Expected empty todo message, got: %s", result)
	}

	// Create a project-specific todos file
	projectName := filepath.Base(tempDir)
	todoFilePath := filepath.Join(todosDir, projectName+"_todos.json")

	// Create some todo items
	todos := []TodoItem{
		{
			ID:        "1",
			Content:   "Fix critical bug in authentication",
			Status:    "pending",
			Priority:  "high",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "2",
			Content:     "Complete user registration flow",
			Status:      "completed",
			Priority:    "high",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			CompletedAt: timePtr(time.Now()),
		},
	}

	// Write the todos file directly
	data, err := json.MarshalIndent(todos, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal todos: %v", err)
	}

	err = os.WriteFile(todoFilePath, data, 0o644)
	if err != nil {
		t.Fatalf("Failed to write todo file: %v", err)
	}

	// Test reading existing todos
	result, err = te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "todo_read",
		Input: map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("todo_read failed: %v", err)
	}

	// Should contain the todo content
	if !strings.Contains(result, "Fix critical bug") {
		t.Errorf("Expected todo content, got: %s", result)
	}

	if !strings.Contains(result, "Complete user registration") {
		t.Errorf("Expected second todo, got: %s", result)
	}

	// Should show completed and pending items
	if !strings.Contains(result, "Pending") && !strings.Contains(result, "Completed") {
		t.Errorf("Expected status sections, got: %s", result)
	}
}

func TestToolExecutor_TodoWrite(t *testing.T) {
	tempDir, _, cleanup := setupTodoTest(t)
	defer cleanup()

	// Create ToolExecutor with the temp directory
	te := NewToolExecutor(tempDir)

	// Test creating new todos with correct JSON format
	input := map[string]interface{}{
		"todos": `[
			{"id": "1", "content": "Fix security vulnerability", "status": "pending", "priority": "high"},
			{"id": "2", "content": "Deploy hotfix", "status": "pending", "priority": "high"},
			{"id": "3", "content": "Code review for PR #123", "status": "pending", "priority": "medium"}
		]`,
	}

	result, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "todo_write",
		Input: input,
	})
	if err != nil {
		t.Fatalf("todo_write failed: %v", err)
	}

	if !strings.Contains(result, "successfully") {
		t.Errorf("Expected success message, got: %s", result)
	}

	// Read the todos to verify they were written
	readResult, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "todo_read",
		Input: map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("todo_read failed after write: %v", err)
	}

	if !strings.Contains(readResult, "Fix security vulnerability") {
		t.Errorf("Expected to read written todo, got: %s", readResult)
	}

	if !strings.Contains(readResult, "Deploy hotfix") {
		t.Errorf("Expected to read written todo, got: %s", readResult)
	}

	// Test missing todos parameter
	invalidInput := map[string]interface{}{}

	_, err = te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "todo_write",
		Input: invalidInput,
	})
	if err == nil {
		t.Error("Expected error for missing todos")
	}

	if !strings.Contains(err.Error(), "todos field is required") {
		t.Errorf("Expected todos required error, got: %v", err)
	}
}

func TestToolExecutor_TodoWorkflow(t *testing.T) {
	tempDir, _, cleanup := setupTodoTest(t)
	defer cleanup()

	// Create ToolExecutor with the temp directory
	te := NewToolExecutor(tempDir)

	// Step 1: Write initial todos
	writeInput := map[string]interface{}{
		"todos": `[
			{"id": "1", "content": "Implement user authentication", "status": "pending", "priority": "high"},
			{"id": "2", "content": "Design database schema", "status": "pending", "priority": "medium"}
		]`,
	}

	_, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "todo_write",
		Input: writeInput,
	})
	if err != nil {
		t.Fatalf("Initial todo_write failed: %v", err)
	}

	// Step 2: Read the todos
	readResult, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "todo_read",
		Input: map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("todo_read failed: %v", err)
	}

	if !strings.Contains(readResult, "Implement user authentication") {
		t.Errorf("Expected initial content in read result, got: %s", readResult)
	}

	// Step 3: Update with completed tasks
	updateInput := map[string]interface{}{
		"todos": `[
			{"id": "1", "content": "Implement user authentication", "status": "completed", "priority": "high"},
			{"id": "2", "content": "Design database schema", "status": "pending", "priority": "medium"},
			{"id": "3", "content": "Add error logging", "status": "pending", "priority": "low"}
		]`,
	}

	_, err = te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "todo_write",
		Input: updateInput,
	})
	if err != nil {
		t.Fatalf("Update todo_write failed: %v", err)
	}

	// Step 4: Read updated todos
	finalResult, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "todo_read",
		Input: map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("Final todo_read failed: %v", err)
	}

	if !strings.Contains(finalResult, "Implement user authentication") {
		t.Errorf("Expected completed tasks in final result, got: %s", finalResult)
	}

	if !strings.Contains(finalResult, "Add error logging") {
		t.Errorf("Expected new tasks in final result, got: %s", finalResult)
	}
}

func TestToolExecutor_TodoEdgeCases(t *testing.T) {
	tempDir, _, cleanup := setupTodoTest(t)
	defer cleanup()

	// Create ToolExecutor with the temp directory
	te := NewToolExecutor(tempDir)

	// Test with empty todos array
	emptyInput := map[string]interface{}{
		"todos": "[]",
	}

	_, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "todo_write",
		Input: emptyInput,
	})
	if err != nil {
		t.Fatalf("todo_write with empty array failed: %v", err)
	}

	// Read should show no todos
	readResult, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "todo_read",
		Input: map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("todo_read after empty write failed: %v", err)
	}

	if !strings.Contains(readResult, "No todos found") {
		t.Errorf("Expected no todos message after empty write, got: %s", readResult)
	}

	// Test with invalid JSON
	invalidInput := map[string]interface{}{
		"todos": "invalid json",
	}

	_, err = te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "todo_write",
		Input: invalidInput,
	})
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}

	// Test with missing required fields
	missingFieldInput := map[string]interface{}{
		"todos": `[{"content": "Missing ID", "status": "pending"}]`,
	}

	_, err = te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "todo_write",
		Input: missingFieldInput,
	})
	if err == nil {
		t.Error("Expected error for missing ID field")
	}

	// Test with special characters
	specialInput := map[string]interface{}{
		"todos": `[
			{"id": "special", "content": "Handle UTF-8: café, naïve, résumé 🚀", "status": "pending", "priority": "medium"}
		]`,
	}

	_, err = te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "todo_write",
		Input: specialInput,
	})
	if err != nil {
		t.Fatalf("todo_write with special characters failed: %v", err)
	}

	// Read back and verify special characters are preserved
	specialResult, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "todo_read",
		Input: map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("todo_read with special characters failed: %v", err)
	}

	if !strings.Contains(specialResult, "café, naïve, résumé 🚀") {
		t.Errorf("Expected UTF-8 characters preserved, got: %s", specialResult)
	}
}

// Helper function to create a pointer to a time.Time
func timePtr(t time.Time) *time.Time {
	return &t
}
