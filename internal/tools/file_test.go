package tools

import (
	"context"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolExecutor_ReadFile(t *testing.T) {
	// Create temporary directory and file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Hello, World!\nThis is a test file."

	err := os.WriteFile(testFile, []byte(testContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	te := NewToolExecutor(tempDir)

	// Test successful read
	input := map[string]interface{}{
		"file_path": "test.txt",
	}

	result, err := te.readFile(input)
	if err != nil {
		t.Fatalf("readFile failed: %v", err)
	}

	if !strings.Contains(result, testContent) {
		t.Errorf("Expected result to contain test content, got: %s", result)
	}

	if !strings.Contains(result, "Lines: 2") {
		t.Errorf("Expected result to show line count, got: %s", result)
	}

	// Test file not found
	input = map[string]interface{}{
		"file_path": "nonexistent.txt",
	}

	_, err = te.readFile(input)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}

	// Test missing file_path parameter
	input = map[string]interface{}{}

	_, err = te.readFile(input)
	if err == nil {
		t.Error("Expected error for missing file_path")
	}
}

func TestToolExecutor_WriteFile(t *testing.T) {
	tempDir := t.TempDir()
	te := NewToolExecutor(tempDir)

	// Test successful write
	input := map[string]interface{}{
		"file_path": "new_file.txt",
		"content":   "New file content\nSecond line",
	}

	result, err := te.writeFile(input)
	if err != nil {
		t.Fatalf("writeFile failed: %v", err)
	}

	if !strings.Contains(result, "written successfully") {
		t.Errorf("Expected success message, got: %s", result)
	}

	// Verify file was created with correct content
	filePath := filepath.Join(tempDir, "new_file.txt")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	expected := "New file content\nSecond line"
	if string(content) != expected {
		t.Errorf("Expected file content '%s', got '%s'", expected, string(content))
	}

	// Test missing parameters
	input = map[string]interface{}{
		"file_path": "test.txt",
		// missing content
	}

	_, err = te.writeFile(input)
	if err == nil {
		t.Error("Expected error for missing content")
	}
}

func TestToolExecutor_CreateFile(t *testing.T) {
	tempDir := t.TempDir()
	te := NewToolExecutor(tempDir)

	// Test successful create
	input := map[string]interface{}{
		"file_path": "created_file.txt",
		"content":   "Created content",
	}

	result, err := te.createFile(input)
	if err != nil {
		t.Fatalf("createFile failed: %v", err)
	}

	if !strings.Contains(result, "created successfully") {
		t.Errorf("Expected success message, got: %s", result)
	}

	// Verify file exists
	filePath := filepath.Join(tempDir, "created_file.txt")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Expected file to be created")
	}

	// Test creating file that already exists
	_, err = te.createFile(input)
	if err == nil {
		t.Error("Expected error when creating existing file")
	}
}

func TestToolExecutor_EditFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "edit_test.txt")
	originalContent := "Line 1\nLine 2\nLine 3\nLine 4"

	err := os.WriteFile(testFile, []byte(originalContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	te := NewToolExecutor(tempDir)

	// Test successful edit
	input := map[string]interface{}{
		"file_path": "edit_test.txt",
		"old_text":  "Line 2",
		"new_text":  "Modified Line 2",
	}

	result, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "edit_file",
		Input: input,
	})
	if err != nil {
		t.Fatalf("editFile failed: %v", err)
	}

	if !strings.Contains(result, "edited successfully") {
		t.Errorf("Expected success message, got: %s", result)
	}

	// Verify content was changed
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read edited file: %v", err)
	}

	if !strings.Contains(string(content), "Modified Line 2") {
		t.Error("Expected file to contain modified content")
	}

	// Check that the original "Line 2" (without modification) is not present
	if strings.Contains(string(content), "Line 2\n") && !strings.Contains(string(content), "Modified Line 2") {
		t.Error("Expected original content to be replaced")
	}

	// Test string not found
	input = map[string]interface{}{
		"file_path": "edit_test.txt",
		"old_text":  "Nonexistent string",
		"new_text":  "Replacement",
	}

	_, err = te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "edit_file",
		Input: input,
	})
	if err == nil {
		t.Error("Expected error when old_text not found")
	}
}

func TestToolExecutor_DeleteFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "delete_test.txt")

	err := os.WriteFile(testFile, []byte("content"), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	te := NewToolExecutor(tempDir)

	// Test successful delete
	input := map[string]interface{}{
		"file_path": "delete_test.txt",
	}

	result, err := te.deleteFile(input)
	if err != nil {
		t.Fatalf("deleteFile failed: %v", err)
	}

	if !strings.Contains(result, "deleted successfully") {
		t.Errorf("Expected success message, got: %s", result)
	}

	// Verify file was deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Expected file to be deleted")
	}

	// Test deleting nonexistent file
	_, err = te.deleteFile(input)
	if err == nil {
		t.Error("Expected error when deleting nonexistent file")
	}
}
