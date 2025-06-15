package tools

import (
	"context"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// BashResult contains detailed information about command execution
type BashResult struct {
	Output     string
	ExitCode   int
	Duration   time.Duration
	Command    string
	WorkingDir string
}

// executeBash executes a bash command with enhanced features
func (te *ToolExecutor) executeBash(input map[string]interface{}) (string, error) {
	command, ok := input["command"].(string)
	if !ok {
		return "", fmt.Errorf("command is required")
	}

	// Add validation
	if err := te.validateCommand(command); err != nil {
		return "", err
	}

	// Optional working directory override
	workingDir := te.rootPath
	if dir, ok := input["working_dir"].(string); ok && dir != "" {
		if filepath.IsAbs(dir) {
			workingDir = dir
		} else {
			workingDir = filepath.Join(te.rootPath, dir)
		}

		// Validate directory exists
		if _, err := os.Stat(workingDir); os.IsNotExist(err) {
			return "", fmt.Errorf("working directory does not exist: %s", workingDir)
		}
	}

	// Optional timeout override (default 30s)
	timeout := 30 * time.Second
	if timeoutSec, ok := input["timeout"].(float64); ok && timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
		if timeout > 300*time.Second { // Max 5 minutes
			timeout = 300 * time.Second
		}
	}

	// Optional environment variables
	env := os.Environ()
	if envVars, ok := input["env"].(map[string]interface{}); ok {
		for key, value := range envVars {
			if strValue, ok := value.(string); ok {
				env = append(env, fmt.Sprintf("%s=%s", key, strValue))
			}
		}
	}

	loggy.Debug("ToolExecutor executeBash",
		"command", command,
		"working_dir", workingDir,
		"timeout", timeout)

	startTime := time.Now()

	// Create command with context for better cancellation
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = workingDir
	cmd.Env = env

	// Capture both stdout and stderr separately for better debugging
	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		}
	}

	result := &BashResult{
		Output:     strings.TrimSpace(string(output)),
		ExitCode:   exitCode,
		Duration:   duration,
		Command:    command,
		WorkingDir: workingDir,
	}

	// Format response based on success/failure
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			loggy.Error("ToolExecutor executeBash timeout",
				"command", command,
				"timeout", timeout,
				"duration", duration)
			return "", fmt.Errorf("command timed out after %v\nCommand: %s\nOutput: %s",
				timeout, command, result.Output)
		}

		loggy.Error("ToolExecutor executeBash failed",
			"command", command,
			"exit_code", exitCode,
			"duration", duration,
			"error", err)

		// Enhanced error reporting
		errorMsg := fmt.Sprintf("Command failed with exit code %d\nCommand: %s\nWorking Directory: %s\nDuration: %v\nOutput:\n%s",
			exitCode, command, workingDir, duration, result.Output)

		return "", fmt.Errorf("%s", errorMsg)
	}

	// Success case - provide detailed output
	loggy.Info("ToolExecutor executeBash success",
		"command", command,
		"duration", duration,
		"output_size", len(result.Output))

	// Format successful response
	response := te.formatBashResponse(result)
	return response, nil
}

// formatBashResponse formats the bash execution result
func (te *ToolExecutor) formatBashResponse(result *BashResult) string {
	var response strings.Builder

	response.WriteString(fmt.Sprintf("Command: %s\n", result.Command))
	response.WriteString(fmt.Sprintf("Working Directory: %s\n", result.WorkingDir))
	response.WriteString(fmt.Sprintf("Exit Code: %d\n", result.ExitCode))
	response.WriteString(fmt.Sprintf("Duration: %v\n", result.Duration))

	if result.Output != "" {
		response.WriteString(fmt.Sprintf("Output:\n%s", result.Output))
	} else {
		response.WriteString("Output: (no output)")
	}

	return response.String()
}

// validateCommand performs basic security checks on commands
func (te *ToolExecutor) validateCommand(command string) error {
	// Extract first command word
	parts := strings.Fields(command)
	if len(parts) > 0 && !te.commandExists(parts[0]) {
		return fmt.Errorf("command not found: %s", parts[0])
	}

	// Block potentially dangerous commands
	dangerousCommands := []string{
		"rm -rf /",
		"rm -rf ~/",
		":(){ :|:& };:",   // Fork bomb
		"dd if=/dev/zero", // Disk fill
		"chmod -R 777 /",  // Dangerous permissions
	}

	commandLower := strings.ToLower(strings.TrimSpace(command))
	for _, dangerous := range dangerousCommands {
		if strings.Contains(commandLower, dangerous) {
			return fmt.Errorf("potentially dangerous command blocked: %s", command)
		}
	}

	return nil
}

// Helper function to check if command exists
func (te *ToolExecutor) commandExists(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}
