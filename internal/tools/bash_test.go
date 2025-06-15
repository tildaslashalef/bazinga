package tools

import (
	"context"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"strings"
	"testing"
)

func TestToolExecutor_Bash(t *testing.T) {
	tempDir := t.TempDir()
	te := NewToolExecutor(tempDir)

	// Test simple command
	input := map[string]interface{}{
		"command": "echo 'Hello, World!'",
	}

	result, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "bash",
		Input: input,
	})
	if err != nil {
		t.Fatalf("bash failed: %v", err)
	}

	if !strings.Contains(result, "Hello, World!") {
		t.Errorf("Expected command output, got: %s", result)
	}

	// Should contain enhanced output format
	if !strings.Contains(result, "Command:") {
		t.Errorf("Expected enhanced output format, got: %s", result)
	}

	if !strings.Contains(result, "Exit Code: 0") {
		t.Errorf("Expected exit code in output, got: %s", result)
	}

	// Test command with error
	input = map[string]interface{}{
		"command": "ls /nonexistent/directory",
	}

	_, err = te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "bash",
		Input: input,
	})
	if err == nil {
		t.Error("Expected error for failed command")
	}

	if !strings.Contains(err.Error(), "Command failed with exit code") {
		t.Errorf("Expected enhanced error format, got: %v", err)
	}

	// Test missing command
	input = map[string]interface{}{}

	_, err = te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "bash",
		Input: input,
	})
	if err == nil {
		t.Error("Expected error for missing command")
	}

	// Test working directory
	input = map[string]interface{}{
		"command": "pwd",
	}

	result, err = te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "bash",
		Input: input,
	})
	if err != nil {
		t.Fatalf("bash pwd failed: %v", err)
	}

	if !strings.Contains(result, tempDir) {
		t.Errorf("Expected command to run in temp directory, got: %s", result)
	}
}

func TestToolExecutor_Bash_LongRunningCommand(t *testing.T) {
	tempDir := t.TempDir()
	te := NewToolExecutor(tempDir)

	// Test command that takes some time
	input := map[string]interface{}{
		"command": "sleep 0.1 && echo 'done'",
	}

	result, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "bash",
		Input: input,
	})
	if err != nil {
		t.Fatalf("bash sleep command failed: %v", err)
	}

	if !strings.Contains(result, "done") {
		t.Errorf("Expected 'done' in output, got: %s", result)
	}

	// Should contain duration information
	if !strings.Contains(result, "Duration:") {
		t.Errorf("Expected duration in output, got: %s", result)
	}
}

func TestToolExecutor_Bash_EnhancedFeatures(t *testing.T) {
	tempDir := t.TempDir()
	te := NewToolExecutor(tempDir)

	// Test custom timeout
	input := map[string]interface{}{
		"command": "echo 'test'",
		"timeout": 5.0, // 5 seconds
	}

	result, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "bash",
		Input: input,
	})
	if err != nil {
		t.Fatalf("bash with timeout failed: %v", err)
	}

	if !strings.Contains(result, "test") {
		t.Errorf("Expected 'test' in output, got: %s", result)
	}

	// Test environment variables
	input = map[string]interface{}{
		"command": "echo $TEST_VAR",
		"env": map[string]interface{}{
			"TEST_VAR": "hello_world",
		},
	}

	result, err = te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "bash",
		Input: input,
	})
	if err != nil {
		t.Fatalf("bash with env vars failed: %v", err)
	}

	if !strings.Contains(result, "hello_world") {
		t.Errorf("Expected environment variable output, got: %s", result)
	}
}

func TestToolExecutor_Bash_CommandValidation(t *testing.T) {
	tempDir := t.TempDir()
	te := NewToolExecutor(tempDir)

	// Test dangerous command blocking
	input := map[string]interface{}{
		"command": "rm -rf /",
	}

	_, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "bash",
		Input: input,
	})
	if err == nil {
		t.Error("Expected error for dangerous command")
	}

	if !strings.Contains(err.Error(), "potentially dangerous command blocked") {
		t.Errorf("Expected dangerous command error, got: %v", err)
	}

	// Test nonexistent command
	input = map[string]interface{}{
		"command": "nonexistentcommand123",
	}

	_, err = te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "bash",
		Input: input,
	})
	if err == nil {
		t.Error("Expected error for nonexistent command")
	}

	if !strings.Contains(err.Error(), "command not found") {
		t.Errorf("Expected command not found error, got: %v", err)
	}
}

func TestToolExecutor_Bash_Timeout(t *testing.T) {
	tempDir := t.TempDir()
	te := NewToolExecutor(tempDir)

	// Test timeout with very short duration
	input := map[string]interface{}{
		"command": "sleep 2",
		"timeout": 0.1, // 100ms
	}

	_, err := te.ExecuteTool(context.Background(), &llm.ToolCall{
		Name:  "bash",
		Input: input,
	})
	if err == nil {
		t.Error("Expected timeout error")
	}

	if !strings.Contains(err.Error(), "command timed out") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}
