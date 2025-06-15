package tools

import (
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"os/exec"
	"strings"
	"time"
)

// For testing - allows us to mock exec.Command
var execCommand = exec.Command

// gitStatus shows the current git status
func (te *ToolExecutor) gitStatus(input map[string]interface{}) (string, error) {
	loggy.Debug("ToolExecutor gitStatus")

	cmd := execCommand("git", "status", "--porcelain", "-b")
	cmd.Dir = te.rootPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git status failed: %w\nOutput: %s", err, string(output))
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "Working tree clean", nil
	}

	// Parse the output for better formatting
	lines := strings.Split(result, "\n")
	var formattedLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "##") {
			// Branch information
			branch := strings.TrimPrefix(line, "## ")
			formattedLines = append(formattedLines, "Branch: "+branch)
		} else if len(line) >= 3 {
			// File status
			status := line[:2]
			file := strings.TrimSpace(line[3:])

			var statusDesc string
			switch status {
			case "M ":
				statusDesc = "Modified"
			case " M":
				statusDesc = "Modified (unstaged)"
			case "A ":
				statusDesc = "Added"
			case " A":
				statusDesc = "Added (unstaged)"
			case "D ":
				statusDesc = "Deleted"
			case " D":
				statusDesc = "Deleted (unstaged)"
			case "??":
				statusDesc = "Untracked"
			case "R ":
				statusDesc = "Renamed"
			case "C ":
				statusDesc = "Copied"
			case "MM":
				statusDesc = "Modified (staged and unstaged)"
			default:
				statusDesc = status
			}

			formattedLines = append(formattedLines, fmt.Sprintf("  %s: %s", statusDesc, file))
		}
	}

	return strings.Join(formattedLines, "\n"), nil
}

// gitDiff shows the git diff
func (te *ToolExecutor) gitDiff(input map[string]interface{}) (string, error) {
	loggy.Debug("ToolExecutor gitDiff")

	var args []string

	// Check if we want staged or unstaged diff
	staged := false
	if stagedInput, ok := input["staged"].(bool); ok {
		staged = stagedInput
	}

	if staged {
		args = []string{"diff", "--cached"}
	} else {
		args = []string{"diff"}
	}

	// Optional file path
	if filePath, ok := input["file_path"].(string); ok && filePath != "" {
		args = append(args, filePath)
	}

	cmd := execCommand("git", args...)
	cmd.Dir = te.rootPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w\nOutput: %s", err, string(output))
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		if staged {
			return "No staged changes to show", nil
		}
		return "No changes to show", nil
	}

	return result, nil
}

// gitAdd adds files to the staging area
func (te *ToolExecutor) gitAdd(input map[string]interface{}) (string, error) {
	loggy.Debug("ToolExecutor gitAdd")

	pathsInterface, ok := input["paths"]
	if !ok {
		return "", fmt.Errorf("paths array is required")
	}

	var paths []string
	switch v := pathsInterface.(type) {
	case string:
		paths = []string{v}
	case []interface{}:
		for _, pathInterface := range v {
			if path, ok := pathInterface.(string); ok {
				paths = append(paths, path)
			}
		}
	default:
		return "", fmt.Errorf("paths must be a string or array of strings")
	}

	if len(paths) == 0 {
		return "", fmt.Errorf("at least one path is required")
	}

	// Build git add command
	args := []string{"add"}
	args = append(args, paths...)

	cmd := execCommand("git", args...)
	cmd.Dir = te.rootPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git add failed: %w\nOutput: %s", err, string(output))
	}

	return fmt.Sprintf("Added %d file(s) to staging area", len(paths)), nil
}

// gitCommit creates a git commit
func (te *ToolExecutor) gitCommit(input map[string]interface{}) (string, error) {
	loggy.Debug("ToolExecutor gitCommit")

	message, ok := input["message"].(string)
	if !ok {
		return "", fmt.Errorf("commit message is required")
	}

	if strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("commit message cannot be empty")
	}

	cmd := execCommand("git", "commit", "-m", message)
	cmd.Dir = te.rootPath

	// Set timeout
	timeout := 30 * time.Second
	done := make(chan error, 1)

	var output []byte
	var err error

	go func() {
		output, err = cmd.CombinedOutput()
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return "", fmt.Errorf("git commit failed: %w\nOutput: %s", err, string(output))
		}

		result := strings.TrimSpace(string(output))
		return result, nil

	case <-time.After(timeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return "", fmt.Errorf("git commit timed out after %v", timeout)
	}
}

// gitLog shows recent commit history
func (te *ToolExecutor) gitLog(input map[string]interface{}) (string, error) {
	loggy.Debug("ToolExecutor gitLog")

	// Default to 10 commits
	limit := 10
	if limitInput, ok := input["limit"].(float64); ok {
		limit = int(limitInput)
	}

	// Optional file path
	args := []string{"log", fmt.Sprintf("-%d", limit), "--oneline", "--decorate"}
	if filePath, ok := input["file_path"].(string); ok && filePath != "" {
		args = append(args, "--", filePath)
	}

	cmd := execCommand("git", args...)
	cmd.Dir = te.rootPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git log failed: %w\nOutput: %s", err, string(output))
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "No commits found", nil
	}

	return result, nil
}

// gitBranch shows or creates branches
func (te *ToolExecutor) gitBranch(input map[string]interface{}) (string, error) {
	loggy.Debug("ToolExecutor gitBranch")

	var args []string

	if branchName, ok := input["branch_name"].(string); ok && branchName != "" {
		// Create or switch to branch
		if create, ok := input["create"].(bool); ok && create {
			args = []string{"checkout", "-b", branchName}
		} else {
			args = []string{"checkout", branchName}
		}
	} else {
		// List branches
		args = []string{"branch", "-v"}
	}

	cmd := execCommand("git", args...)
	cmd.Dir = te.rootPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git branch operation failed: %w\nOutput: %s", err, string(output))
	}

	result := strings.TrimSpace(string(output))
	return result, nil
}
