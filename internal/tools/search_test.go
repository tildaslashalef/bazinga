package tools

import (
	"context"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolExecutor_Grep(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "grep_test.txt")
	testContent := `Line 1: Hello World
Line 2: Testing grep
Line 3: Another line
Line 4: Hello again`

	err := os.WriteFile(testFile, []byte(testContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	te := NewToolExecutor(tempDir)

	// Test successful grep
	input := map[string]interface{}{
		"pattern": "Hello",
		"path":    "grep_test.txt",
	}

	result, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "grep",
		Input: input,
	})
	if err != nil {
		t.Fatalf("grep failed: %v", err)
	}

	if !strings.Contains(result, "Hello World") {
		t.Errorf("Expected to find 'Hello World', got: %s", result)
	}

	if !strings.Contains(result, "Hello again") {
		t.Errorf("Expected to find 'Hello again', got: %s", result)
	}

	if strings.Contains(result, "Testing grep") {
		t.Errorf("Should not contain non-matching line, got: %s", result)
	}

	// Test no matches
	input = map[string]interface{}{
		"pattern": "nonexistent",
		"path":    "grep_test.txt",
	}

	result, err = te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "grep",
		Input: input,
	})
	if err != nil {
		t.Fatalf("grep failed: %v", err)
	}

	if !strings.Contains(result, "No matches found") {
		t.Errorf("Expected 'No matches found' message, got: %s", result)
	}
}

func TestToolExecutor_Find(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	testFiles := []string{
		"test.txt",
		"example.go",
		"README.md",
		"subdir/nested.txt",
		"subdir/another.go",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tempDir, file)
		err := os.MkdirAll(filepath.Dir(fullPath), 0o755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		err = os.WriteFile(fullPath, []byte("content"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	te := NewToolExecutor(tempDir)

	// Test find by name pattern
	input := map[string]interface{}{
		"name": "*.txt",
	}

	result, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "find",
		Input: input,
	})
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}

	if !strings.Contains(result, "test.txt") {
		t.Errorf("Expected to find test.txt, got: %s", result)
	}

	if !strings.Contains(result, "nested.txt") {
		t.Errorf("Expected to find nested.txt, got: %s", result)
	}

	if strings.Contains(result, "example.go") {
		t.Errorf("Should not find .go files, got: %s", result)
	}

	// Test find in specific directory
	input = map[string]interface{}{
		"name": "*",
		"path": "subdir",
	}

	result, err = te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "find",
		Input: input,
	})
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}

	if !strings.Contains(result, "nested.txt") {
		t.Errorf("Expected to find nested.txt in subdir, got: %s", result)
	}

	if !strings.Contains(result, "another.go") {
		t.Errorf("Expected to find another.go in subdir, got: %s", result)
	}
}

func TestToolExecutor_ListFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files and directories
	testFiles := []string{"file1.txt", "file2.go", "README.md"}
	for _, file := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, file), []byte("content"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	subDir := filepath.Join(tempDir, "subdir")
	err := os.Mkdir(subDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	te := NewToolExecutor(tempDir)

	// Test listing current directory
	input := map[string]interface{}{}

	result, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "list_files",
		Input: input,
	})
	if err != nil {
		t.Fatalf("listFiles failed: %v", err)
	}

	for _, file := range testFiles {
		if !strings.Contains(result, file) {
			t.Errorf("Expected result to contain %s, got: %s", file, result)
		}
	}

	if !strings.Contains(result, "subdir/") {
		t.Errorf("Expected result to contain subdirectory, got: %s", result)
	}

	// Test listing specific directory
	input = map[string]interface{}{
		"directory": "subdir",
	}

	result, err = te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "list_files",
		Input: input,
	})
	if err != nil {
		t.Fatalf("listFiles failed for subdirectory: %v", err)
	}

	if !strings.Contains(result, "subdir") {
		t.Errorf("Expected result to mention subdirectory, got: %s", result)
	}
}
