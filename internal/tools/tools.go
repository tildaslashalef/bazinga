package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"github.com/tildaslashalef/bazinga/internal/loggy"
)

// ToolExecutor handles execution of tools
type ToolExecutor struct {
	rootPath           string
	todoManager        *TodoManager
	webFetcher         *WebFetcher
	fileChangeCallback func(FileChange)
}

// NewToolExecutor creates a new tool executor
func NewToolExecutor(rootPath string) *ToolExecutor {
	return &ToolExecutor{
		rootPath:    rootPath,
		todoManager: NewTodoManager(rootPath),
		webFetcher:  NewWebFetcher(),
	}
}

// SetFileChangeCallback sets the callback for file changes
func (te *ToolExecutor) SetFileChangeCallback(callback func(FileChange)) {
	te.fileChangeCallback = callback
}

// GetAvailableTools returns all available tools for the session
func (te *ToolExecutor) GetAvailableTools() []llm.Tool {
	return []llm.Tool{
		// File operations
		{
			Name:        "read_file",
			Description: "Read the contents of a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to read",
					},
				},
				"required": []string{"file_path"},
			},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file (creates or overwrites)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to write",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The content to write to the file",
					},
				},
				"required": []string{"file_path", "content"},
			},
		},
		{
			Name:        "edit_file",
			Description: "Edit a file by replacing specific text",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to edit",
					},
					"old_text": map[string]interface{}{
						"type":        "string",
						"description": "The text to find and replace",
					},
					"new_text": map[string]interface{}{
						"type":        "string",
						"description": "The new text to replace with",
					},
				},
				"required": []string{"file_path", "old_text", "new_text"},
			},
		},
		{
			Name:        "create_file",
			Description: "Create a new file with content",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "The path for the new file",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The initial content for the file",
					},
				},
				"required": []string{"file_path", "content"},
			},
		},
		{
			Name:        "multi_edit_file",
			Description: "Perform multiple edits on a file in sequence",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to edit",
					},
					"edits": map[string]interface{}{
						"type":        "array",
						"description": "Array of edit operations to perform",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"old_text": map[string]interface{}{
									"type":        "string",
									"description": "The text to find and replace",
								},
								"new_text": map[string]interface{}{
									"type":        "string",
									"description": "The new text to replace with",
								},
							},
							"required": []string{"old_text", "new_text"},
						},
					},
				},
				"required": []string{"file_path", "edits"},
			},
		},
		{
			Name:        "move_file",
			Description: "Move or rename a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source_path": map[string]interface{}{
						"type":        "string",
						"description": "The current path of the file to move",
					},
					"dest_path": map[string]interface{}{
						"type":        "string",
						"description": "The new path for the file",
					},
				},
				"required": []string{"source_path", "dest_path"},
			},
		},
		{
			Name:        "copy_file",
			Description: "Copy a file to a new location",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source_path": map[string]interface{}{
						"type":        "string",
						"description": "The path of the file to copy",
					},
					"dest_path": map[string]interface{}{
						"type":        "string",
						"description": "The destination path for the copy",
					},
				},
				"required": []string{"source_path", "dest_path"},
			},
		},
		{
			Name:        "delete_file",
			Description: "Delete a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "The path of the file to delete",
					},
				},
				"required": []string{"file_path"},
			},
		},
		{
			Name:        "create_dir",
			Description: "Create a directory",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"dir_path": map[string]interface{}{
						"type":        "string",
						"description": "The path of the directory to create",
					},
				},
				"required": []string{"dir_path"},
			},
		},
		{
			Name:        "delete_dir",
			Description: "Delete a directory",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"dir_path": map[string]interface{}{
						"type":        "string",
						"description": "The path of the directory to delete",
					},
					"recursive": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to delete non-empty directories recursively (default: false)",
					},
				},
				"required": []string{"dir_path"},
			},
		},
		{
			Name:        "list_files",
			Description: "List files in a directory",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"directory": map[string]interface{}{
						"type":        "string",
						"description": "The directory to list files from (optional, defaults to current directory)",
					},
				},
			},
		},
		// System operations
		{
			Name:        "bash",
			Description: "Execute shell commands, run build scripts, install dependencies, start services, or run any terminal command. Use this tool whenever you need to run, build, test, install, or execute anything via command line.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The bash command to execute",
					},
				},
				"required": []string{"command"},
			},
		},
		// Search operations
		{
			Name:        "grep",
			Description: "Search for text patterns, function names, variables, imports, or any code content across files. Use this tool when you need to find where something is defined or used in the codebase.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{
						"type":        "string",
						"description": "The pattern to search for (supports regex)",
					},
					"files": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Files to search (optional, defaults to all files)",
					},
					"recursive": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to search recursively (default: true)",
					},
					"context": map[string]interface{}{
						"type":        "number",
						"description": "Number of context lines to show around matches (default: 0)",
					},
					"ignore_case": map[string]interface{}{
						"type":        "boolean",
						"description": "Case insensitive search (default: false)",
					},
					"extensions": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "File extensions to search (e.g. ['.go', '.js']) - defaults to common code files",
					},
				},
				"required": []string{"pattern"},
			},
		},
		{
			Name:        "find",
			Description: "Find files and directories by name, extension, or path patterns. Use this tool when you need to locate specific files or discover the project structure.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "File name pattern to search for",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "File type (file, dir)",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Directory to search in (optional)",
					},
				},
			},
		},
		{
			Name:        "fuzzy_search",
			Description: "Fuzzy search for files when you only remember part of the filename. Use this tool when you're looking for files but aren't sure of the exact name.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The fuzzy search query (partial filename)",
					},
				},
				"required": []string{"query"},
			},
		},
		// Todo management
		{
			Name:        "todo_read",
			Description: "Read the current todo list",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "todo_write",
			Description: "Update the todo list with new items",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"todos": map[string]interface{}{
						"type":        "string",
						"description": "JSON array of todo items with id, content, status, and priority fields",
					},
				},
				"required": []string{"todos"},
			},
		},
		// Git operations
		{
			Name:        "git_status",
			Description: "Show the current git status",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "git_diff",
			Description: "Show git diff (changes)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"staged": map[string]interface{}{
						"type":        "boolean",
						"description": "Show staged changes instead of unstaged (default: false)",
					},
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Show diff for specific file (optional)",
					},
				},
			},
		},
		{
			Name:        "git_add",
			Description: "Add files to git staging area",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"paths": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "File paths to add to staging area",
					},
				},
				"required": []string{"paths"},
			},
		},
		{
			Name:        "git_commit",
			Description: "Create a git commit",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": map[string]interface{}{
						"type":        "string",
						"description": "Commit message",
					},
				},
				"required": []string{"message"},
			},
		},
		{
			Name:        "git_log",
			Description: "Show recent commit history",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Number of commits to show (default: 10)",
					},
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Show history for specific file (optional)",
					},
				},
			},
		},
		{
			Name:        "git_branch",
			Description: "List, create, or switch git branches",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"branch_name": map[string]interface{}{
						"type":        "string",
						"description": "Branch name to create or switch to (optional)",
					},
					"create": map[string]interface{}{
						"type":        "boolean",
						"description": "Create new branch if it doesn't exist (default: false)",
					},
				},
			},
		},
		// Web operations
		{
			Name:        "web_fetch",
			Description: "Fetch content from a URL",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The URL to fetch content from",
					},
				},
				"required": []string{"url"},
			},
		},
	}
}

// ExecuteTool executes a tool call
func (te *ToolExecutor) ExecuteTool(ctx context.Context, toolCall *llm.ToolCall) (string, error) {
	loggy.Debug("ToolExecutor ExecuteTool", "tool_name", toolCall.Name, "input", toolCall.Input, "id", toolCall.ID)

	switch toolCall.Name {
	// File operations
	case "read_file":
		return te.readFile(toolCall.Input)
	case "write_file":
		return te.writeFile(toolCall.Input)
	case "edit_file":
		return te.editFile(toolCall.Input)
	case "multi_edit_file":
		return te.multiEditFile(toolCall.Input)
	case "list_files":
		return te.listFiles(toolCall.Input)
	case "create_file":
		return te.createFile(toolCall.Input)
	case "move_file":
		return te.moveFile(toolCall.Input)
	case "copy_file":
		return te.copyFile(toolCall.Input)
	case "delete_file":
		return te.deleteFile(toolCall.Input)
	case "create_dir":
		return te.createDir(toolCall.Input)
	case "delete_dir":
		return te.deleteDir(toolCall.Input)

	// System operations
	case "bash":
		return te.executeBash(toolCall.Input)

	// Search operations
	case "grep":
		return te.grepFiles(toolCall.Input)
	case "find":
		return te.findFiles(toolCall.Input)
	case "fuzzy_search":
		return te.fuzzySearch(toolCall.Input)

	// Todo management
	case "todo_read":
		return te.todoRead(toolCall.Input)
	case "todo_write":
		return te.todoWrite(toolCall.Input)

	// Git operations
	case "git_status":
		return te.gitStatus(toolCall.Input)
	case "git_diff":
		return te.gitDiff(toolCall.Input)
	case "git_add":
		return te.gitAdd(toolCall.Input)
	case "git_commit":
		return te.gitCommit(toolCall.Input)
	case "git_log":
		return te.gitLog(toolCall.Input)
	case "git_branch":
		return te.gitBranch(toolCall.Input)

	// Web operations
	case "web_fetch":
		return te.webFetch(ctx, toolCall.Input)

	default:
		return "", fmt.Errorf("unknown tool: %s", toolCall.Name)
	}
}

// todoRead reads the current todo list
func (te *ToolExecutor) todoRead(input map[string]interface{}) (string, error) {
	return te.todoManager.Read()
}

// todoWrite updates the todo list with new items
func (te *ToolExecutor) todoWrite(input map[string]interface{}) (string, error) {
	todosData, ok := input["todos"].(string)
	if !ok {
		return "", fmt.Errorf("todos field is required and must be a JSON string")
	}

	if err := te.todoManager.Write(todosData); err != nil {
		return "", fmt.Errorf("failed to update todos: %w", err)
	}

	return "Todo list updated successfully", nil
}

// webFetch fetches content from a URL
func (te *ToolExecutor) webFetch(ctx context.Context, input map[string]interface{}) (string, error) {
	url, ok := input["url"].(string)
	if !ok {
		return "", fmt.Errorf("url field is required")
	}

	content, err := te.webFetcher.Fetch(ctx, url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}

	return fmt.Sprintf("Content from %s:\n\n%s", url, content), nil
}

// ToolCallFromJSON parses a tool call from JSON
func ToolCallFromJSON(data []byte) (*llm.ToolCall, error) {
	var toolCall llm.ToolCall
	if err := json.Unmarshal(data, &toolCall); err != nil {
		return nil, fmt.Errorf("failed to parse tool call: %w", err)
	}
	return &toolCall, nil
}
