package tools

import (
	"context"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"os"
	"path/filepath"
	"testing"
)

func TestToolExecutor(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "bazinga-tools-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	executor := NewToolExecutor(tmpDir)

	// Test GetAvailableTools
	tools := executor.GetAvailableTools()
	if len(tools) != 24 {
		t.Errorf("Expected 24 tools, got %d", len(tools))
	}

	expectedTools := []string{
		"read_file", "write_file", "edit_file", "create_file", "multi_edit_file",
		"move_file", "copy_file", "delete_file", "create_dir", "delete_dir", "list_files",
		"bash", "grep", "find", "fuzzy_search", "todo_read", "todo_write",
		"git_status", "git_diff", "git_add", "git_commit", "git_log", "git_branch",
		"web_fetch",
	}

	// Check that all expected tools are present
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	for _, expectedTool := range expectedTools {
		if !toolNames[expectedTool] {
			t.Errorf("Expected tool %s not found", expectedTool)
		}
	}

	// Test create_file
	createCall := &llm.ToolCall{
		Name: "create_file",
		Input: map[string]interface{}{
			"file_path": "test.txt",
			"content":   "Hello, World!",
		},
	}

	result, err := executor.ExecuteTool(context.Background(), createCall)
	if err != nil {
		t.Errorf("create_file failed: %v", err)
	}

	if !contains(result, "created successfully") {
		t.Errorf("Unexpected create result: %s", result)
	}

	// Test read_file
	readCall := &llm.ToolCall{
		Name: "read_file",
		Input: map[string]interface{}{
			"file_path": "test.txt",
		},
	}

	result, err = executor.ExecuteTool(context.Background(), readCall)
	if err != nil {
		t.Errorf("read_file failed: %v", err)
	}

	if !contains(result, "Hello, World!") {
		t.Errorf("Unexpected read result: %s", result)
	}

	// Test edit_file
	editCall := &llm.ToolCall{
		Name: "edit_file",
		Input: map[string]interface{}{
			"file_path": "test.txt",
			"old_text":  "Hello, World!",
			"new_text":  "Hello, Bazinga!",
		},
	}

	result, err = executor.ExecuteTool(context.Background(), editCall)
	if err != nil {
		t.Errorf("edit_file failed: %v", err)
	}

	if !contains(result, "edited successfully") {
		t.Errorf("Unexpected edit result: %s", result)
	}

	// Verify the edit worked
	content, err := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
	if err != nil {
		t.Errorf("Failed to read edited file: %v", err)
	}

	if string(content) != "Hello, Bazinga!" {
		t.Errorf("Edit didn't work. Expected 'Hello, Bazinga!', got '%s'", string(content))
	}

	// Test list_files
	listCall := &llm.ToolCall{
		Name:  "list_files",
		Input: map[string]interface{}{},
	}

	result, err = executor.ExecuteTool(context.Background(), listCall)
	if err != nil {
		t.Errorf("list_files failed: %v", err)
	}

	if !contains(result, "test.txt") {
		t.Errorf("list_files should show test.txt: %s", result)
	}

	// Test todo_read (should return empty list initially)
	todoReadCall := &llm.ToolCall{
		Name:  "todo_read",
		Input: map[string]interface{}{},
	}

	result, err = executor.ExecuteTool(context.Background(), todoReadCall)
	if err != nil {
		t.Errorf("todo_read failed: %v", err)
	}

	if !contains(result, "No todos found") {
		t.Errorf("todo_read should return empty message initially: %s", result)
	}

	// Test todo_write
	todoWriteCall := &llm.ToolCall{
		Name: "todo_write",
		Input: map[string]interface{}{
			"todos": `[{"id":"test1","content":"Test todo item","status":"pending","priority":"high"}]`,
		},
	}

	result, err = executor.ExecuteTool(context.Background(), todoWriteCall)
	if err != nil {
		t.Errorf("todo_write failed: %v", err)
	}

	if !contains(result, "Todo list updated successfully") {
		t.Errorf("Unexpected todo_write result: %s", result)
	}

	// Test todo_read again (should now have the item)
	result, err = executor.ExecuteTool(context.Background(), todoReadCall)
	if err != nil {
		t.Errorf("todo_read failed after write: %v", err)
	}

	if !contains(result, "Test todo item") {
		t.Errorf("todo_read should show written item: %s", result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
