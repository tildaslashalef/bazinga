package session

import (
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"strings"
	"time"
)

// PermissionLevel represents the permission level for tool execution
type PermissionLevel int

const (
	// PermissionDeny - Always deny execution
	PermissionDeny PermissionLevel = iota
	// PermissionPrompt - Always prompt user for approval
	PermissionPrompt
	// PermissionAllow - Always allow execution
	PermissionAllow
)

// ToolPermissionRule defines permission rules for specific tools
type ToolPermissionRule struct {
	ToolName   string
	Permission PermissionLevel
	// Optional conditions for more granular control
	FilePatterns []string // File patterns that require different permissions
	Commands     []string // Specific commands that require different permissions
}

// PermissionDecision represents a user's decision on a tool execution request
type PermissionDecision struct {
	Approved       bool
	RememberChoice bool
	ApplyToSimilar bool
	Reason         string
	Timestamp      time.Time
}

// PermissionRule represents a session-specific permission rule
type PermissionRule struct {
	ToolPattern    string
	FilePattern    string
	CommandPattern string
	Decision       PermissionDecision
	CreatedAt      time.Time
}

// PermissionManager handles tool execution permissions
type PermissionManager struct {
	defaultPermission PermissionLevel
	toolRules         map[string]*ToolPermissionRule
	promptCallback    func(toolCall *llm.ToolCall) bool // Callback to prompt user

	// Async permission handling
	toolQueue    *ToolQueue
	patterns     map[string]PermissionDecision
	sessionRules []PermissionRule
}

// NewPermissionManager creates a new permission manager with defaults
func NewPermissionManager() *PermissionManager {
	pm := &PermissionManager{
		defaultPermission: PermissionPrompt,
		toolRules:         make(map[string]*ToolPermissionRule),
		patterns:          make(map[string]PermissionDecision),
		sessionRules:      make([]PermissionRule, 0),
	}
	pm.setDefaultRules()
	return pm
}

// SetToolQueue sets the tool queue for async permission handling
func (pm *PermissionManager) SetToolQueue(queue *ToolQueue) {
	pm.toolQueue = queue
}

// setDefaultRules configures default permission rules
func (pm *PermissionManager) setDefaultRules() {
	// Safe read-only operations - allow without prompting
	safeTools := []string{"read_file", "list_files", "grep", "find", "fuzzy_search", "git_status", "git_diff", "git_log", "todo_read"}
	for _, tool := range safeTools {
		pm.toolRules[tool] = &ToolPermissionRule{
			ToolName:   tool,
			Permission: PermissionAllow,
		}
	}

	// Write operations - always prompt
	writeTools := []string{"write_file", "create_file", "edit_file", "multi_edit_file", "move_file", "copy_file", "delete_file", "create_dir", "delete_dir"}
	for _, tool := range writeTools {
		pm.toolRules[tool] = &ToolPermissionRule{
			ToolName:   tool,
			Permission: PermissionPrompt,
		}
	}

	// Potentially dangerous operations - always prompt with extra caution
	dangerousTools := []string{"bash", "git_add", "git_commit", "git_branch"}
	for _, tool := range dangerousTools {
		pm.toolRules[tool] = &ToolPermissionRule{
			ToolName:   tool,
			Permission: PermissionPrompt,
		}
	}

	// Todo operations - allow (safe)
	pm.toolRules["todo_write"] = &ToolPermissionRule{
		ToolName:   "todo_write",
		Permission: PermissionAllow,
	}

	// Web operations - prompt for security
	pm.toolRules["web_fetch"] = &ToolPermissionRule{
		ToolName:   "web_fetch",
		Permission: PermissionPrompt,
	}
}

// SetPromptCallback sets the callback function for user prompts
func (pm *PermissionManager) SetPromptCallback(callback func(toolCall *llm.ToolCall) bool) {
	pm.promptCallback = callback
}

// CheckPermission checks if a tool execution should be allowed
func (pm *PermissionManager) CheckPermission(toolCall *llm.ToolCall) bool {
	if toolCall == nil {
		return false
	}

	// Get permission level for this tool
	permission := pm.getToolPermission(toolCall)

	switch permission {
	case PermissionAllow:
		return true
	case PermissionDeny:
		return false
	case PermissionPrompt:
		if pm.promptCallback != nil {
			return pm.promptCallback(toolCall)
		}
		// If no prompt callback, default to deny for safety
		return false
	default:
		return false
	}
}

// getToolPermission determines the permission level for a specific tool call
func (pm *PermissionManager) getToolPermission(toolCall *llm.ToolCall) PermissionLevel {
	// Check if we have a specific rule for this tool
	if rule, exists := pm.toolRules[toolCall.Name]; exists {
		// Check for special conditions
		if pm.hasSpecialConditions(toolCall, rule) {
			return PermissionPrompt // Escalate to prompt for special conditions
		}
		return rule.Permission
	}

	// Use default permission
	return pm.defaultPermission
}

// hasSpecialConditions checks if the tool call has conditions that require special handling
func (pm *PermissionManager) hasSpecialConditions(toolCall *llm.ToolCall, rule *ToolPermissionRule) bool {
	// Check for dangerous file patterns
	if filePath, ok := toolCall.Input["file_path"].(string); ok {
		dangerousPatterns := []string{
			"/etc/", "/bin/", "/sbin/", "/usr/bin/", "/usr/sbin/",
			".env", ".key", ".pem", ".p12", ".pfx",
			"passwd", "shadow", "sudoers",
		}

		for _, pattern := range dangerousPatterns {
			if strings.Contains(strings.ToLower(filePath), pattern) {
				return true
			}
		}
	}

	// Check for dangerous bash commands
	if toolCall.Name == "bash" {
		if command, ok := toolCall.Input["command"].(string); ok {
			dangerousCommands := []string{
				"rm -rf", "sudo", "su", "chmod +x", "curl", "wget",
				"npm install", "pip install", "go install",
				"docker", "systemctl", "service",
			}

			commandLower := strings.ToLower(command)
			for _, dangerous := range dangerousCommands {
				if strings.Contains(commandLower, dangerous) {
					return true
				}
			}
		}
	}

	// Check for git operations that modify history
	if strings.HasPrefix(toolCall.Name, "git_") {
		if command, ok := toolCall.Input["command"].(string); ok {
			historyCommands := []string{"rebase", "reset --hard", "push --force", "commit --amend"}
			commandLower := strings.ToLower(command)
			for _, historyCmd := range historyCommands {
				if strings.Contains(commandLower, historyCmd) {
					return true
				}
			}
		}
	}

	return false
}

// GetToolRisk returns a risk assessment for a tool call
func (pm *PermissionManager) GetToolRisk(toolCall *llm.ToolCall) string {
	if toolCall == nil {
		return "unknown"
	}

	// Check for high-risk conditions
	if pm.hasSpecialConditions(toolCall, &ToolPermissionRule{}) {
		return "high"
	}

	// Assess based on tool type
	switch toolCall.Name {
	case "read_file", "list_files", "grep", "find", "fuzzy_search", "git_status", "git_diff", "git_log", "todo_read":
		return "low"
	case "write_file", "create_file", "edit_file", "multi_edit_file", "todo_write":
		return "medium"
	case "move_file", "copy_file", "delete_file", "create_dir", "delete_dir", "git_add", "git_commit":
		return "medium"
	case "bash", "git_branch", "web_fetch":
		return "high"
	default:
		return "medium"
	}
}

// FormatPermissionPrompt creates a user-friendly permission prompt
func (pm *PermissionManager) FormatPermissionPrompt(toolCall *llm.ToolCall) string {
	if toolCall == nil {
		return ""
	}

	var prompt strings.Builder

	// Tool description with nerd font icon
	actionName := pm.getActionDescription(toolCall)
	prompt.WriteString(fmt.Sprintf(" Permission required: %s", actionName))

	// Risk level with nerd font icons
	risk := pm.GetToolRisk(toolCall)
	var riskIcon string
	switch risk {
	case "low":
		riskIcon = "" // Green circle
	case "medium":
		riskIcon = "" // Yellow circle
	case "high":
		riskIcon = "" // Red circle
	default:
		riskIcon = "" // White circle
	}
	prompt.WriteString(fmt.Sprintf("\n%s Risk: %s", riskIcon, strings.ToUpper(risk)))

	// Details
	if details := pm.getToolDetails(toolCall); details != "" {
		prompt.WriteString(fmt.Sprintf("\nDetails: %s", details))
	}

	// Special warnings with nerd font icon
	if warnings := pm.getToolWarnings(toolCall); warnings != "" {
		prompt.WriteString(fmt.Sprintf("\n %s", warnings))
	}

	return prompt.String()
}

// getActionDescription returns a human-readable description of what the tool will do
func (pm *PermissionManager) getActionDescription(toolCall *llm.ToolCall) string {
	switch toolCall.Name {
	case "read_file":
		if filePath, ok := toolCall.Input["file_path"].(string); ok {
			return fmt.Sprintf("Read file '%s'", filePath)
		}
		return "Read a file"
	case "write_file", "create_file":
		if filePath, ok := toolCall.Input["file_path"].(string); ok {
			return fmt.Sprintf("Write to file '%s'", filePath)
		}
		return "Write to a file"
	case "edit_file":
		if filePath, ok := toolCall.Input["file_path"].(string); ok {
			return fmt.Sprintf("Edit file '%s'", filePath)
		}
		return "Edit a file"
	case "delete_file":
		if filePath, ok := toolCall.Input["file_path"].(string); ok {
			return fmt.Sprintf("Delete file '%s'", filePath)
		}
		return "Delete a file"
	case "bash":
		if command, ok := toolCall.Input["command"].(string); ok {
			return fmt.Sprintf("Run command '%s'", command)
		}
		return "Execute a shell command"
	case "git_commit":
		if message, ok := toolCall.Input["message"].(string); ok {
			return fmt.Sprintf("Git commit with message '%s'", message)
		}
		return "Create a git commit"
	case "web_fetch":
		if url, ok := toolCall.Input["url"].(string); ok {
			return fmt.Sprintf("Fetch data from '%s'", url)
		}
		return "Fetch data from the web"
	default:
		return fmt.Sprintf("Execute %s tool", toolCall.Name)
	}
}

// getToolDetails returns additional details about what the tool will do
func (pm *PermissionManager) getToolDetails(toolCall *llm.ToolCall) string {
	switch toolCall.Name {
	case "edit_file":
		if oldStr, ok := toolCall.Input["old_string"].(string); ok {
			if newStr, ok2 := toolCall.Input["new_string"].(string); ok2 {
				// Truncate long strings
				if len(oldStr) > 50 {
					oldStr = oldStr[:47] + "..."
				}
				if len(newStr) > 50 {
					newStr = newStr[:47] + "..."
				}
				return fmt.Sprintf("Replace '%s' with '%s'", oldStr, newStr)
			}
		}
	case "bash":
		if command, ok := toolCall.Input["command"].(string); ok {
			if len(command) > 100 {
				return fmt.Sprintf("Command: %s...", command[:97])
			}
			return fmt.Sprintf("Command: %s", command)
		}
	}
	return ""
}

// getToolWarnings returns any warnings about the tool execution
func (pm *PermissionManager) getToolWarnings(toolCall *llm.ToolCall) string {
	warnings := []string{}

	// Check for dangerous file operations
	if filePath, ok := toolCall.Input["file_path"].(string); ok {
		if strings.Contains(filePath, "/etc/") || strings.Contains(filePath, "/bin/") {
			warnings = append(warnings, "Modifying system files")
		}
		if strings.Contains(strings.ToLower(filePath), ".env") || strings.Contains(strings.ToLower(filePath), ".key") {
			warnings = append(warnings, "Accessing sensitive files")
		}
	}

	// Check for dangerous bash commands
	if toolCall.Name == "bash" {
		if command, ok := toolCall.Input["command"].(string); ok {
			commandLower := strings.ToLower(command)
			if strings.Contains(commandLower, "rm -rf") {
				warnings = append(warnings, "Destructive file operation")
			}
			if strings.Contains(commandLower, "sudo") || strings.Contains(commandLower, "su ") {
				warnings = append(warnings, "Requires elevated privileges")
			}
			if strings.Contains(commandLower, "curl") || strings.Contains(commandLower, "wget") {
				warnings = append(warnings, "Network access required")
			}
		}
	}

	// Check for git history modifications
	if strings.HasPrefix(toolCall.Name, "git_") {
		if command, ok := toolCall.Input["command"].(string); ok {
			if strings.Contains(strings.ToLower(command), "rebase") || strings.Contains(strings.ToLower(command), "reset --hard") {
				warnings = append(warnings, "Modifies git history")
			}
		}
	}

	if len(warnings) > 0 {
		return strings.Join(warnings, ", ")
	}
	return ""
}

// RequestPermissionAsync queues a tool for permission approval and returns a decision channel
func (pm *PermissionManager) RequestPermissionAsync(toolCall *llm.ToolCall) <-chan PermissionDecision {
	if pm.toolQueue == nil {
		// Fallback to synchronous behavior if no queue is set
		responseChan := make(chan PermissionDecision, 1)

		approved := pm.CheckPermission(toolCall)
		decision := PermissionDecision{
			Approved:  approved,
			Reason:    "fallback to sync permission check",
			Timestamp: time.Now(),
		}

		responseChan <- decision
		close(responseChan)
		return responseChan
	}

	// Check if we have a cached decision for this tool pattern
	if decision, exists := pm.matchesPattern(toolCall); exists {
		responseChan := make(chan PermissionDecision, 1)
		responseChan <- decision
		close(responseChan)
		return responseChan
	}

	// Queue the tool for permission
	toolID := pm.toolQueue.QueueTool(toolCall, pm)

	// Send permission request to UI
	_ = pm.toolQueue.SendPermissionRequest(toolID)

	// Return channel that will receive the decision
	decisionChan := make(chan PermissionDecision, 1)

	// Convert the bool response channel to PermissionDecision channel
	go func() {
		if responseChan, exists := pm.toolQueue.GetDecisionChannel(toolID); exists {
			approved := <-responseChan
			decision := PermissionDecision{
				Approved:  approved,
				Reason:    "user decision",
				Timestamp: time.Now(),
			}

			// Cache the decision if it should be remembered
			if decision.RememberChoice {
				key := pm.generatePatternKey(toolCall)
				pm.patterns[key] = decision
			}

			decisionChan <- decision
			close(decisionChan)
		} else {
			// Tool not found, default to deny
			decision := PermissionDecision{
				Approved:  false,
				Reason:    "tool not found in queue",
				Timestamp: time.Now(),
			}
			decisionChan <- decision
			close(decisionChan)
		}
	}()

	return decisionChan
}

// matchesPattern checks if a tool call matches any cached permission patterns
func (pm *PermissionManager) matchesPattern(toolCall *llm.ToolCall) (PermissionDecision, bool) {
	key := pm.generatePatternKey(toolCall)
	decision, exists := pm.patterns[key]
	return decision, exists
}

// generatePatternKey generates a key for caching permission decisions
func (pm *PermissionManager) generatePatternKey(toolCall *llm.ToolCall) string {
	key := toolCall.Name

	// Add file path if present
	if filePath, ok := toolCall.Input["file_path"].(string); ok {
		key += ":" + filePath
	}

	// Add command if present (for bash tools)
	if command, ok := toolCall.Input["command"].(string); ok {
		// For commands, only use the first word to allow similar commands
		parts := strings.Fields(command)
		if len(parts) > 0 {
			key += ":" + parts[0]
		}
	}

	return key
}

// AddSessionRule adds a permission rule for the current session
func (pm *PermissionManager) AddSessionRule(rule PermissionRule) {
	pm.sessionRules = append(pm.sessionRules, rule)
}

// GetRiskReasons returns detailed reasons why a tool is considered risky
func (pm *PermissionManager) GetRiskReasons(toolCall *llm.ToolCall) []string {
	reasons := []string{}

	// Check for special conditions
	if pm.hasSpecialConditions(toolCall, &ToolPermissionRule{}) {
		reasons = append(reasons, "Contains dangerous patterns")
	}

	// Check tool-specific risks
	switch toolCall.Name {
	case "bash":
		if command, ok := toolCall.Input["command"].(string); ok {
			commandLower := strings.ToLower(command)
			if strings.Contains(commandLower, "rm -rf") {
				reasons = append(reasons, "Destructive file operation")
			}
			if strings.Contains(commandLower, "sudo") {
				reasons = append(reasons, "Requires elevated privileges")
			}
			if strings.Contains(commandLower, "curl") || strings.Contains(commandLower, "wget") {
				reasons = append(reasons, "Network access")
			}
		}
	case "delete_file":
		reasons = append(reasons, "File deletion")
	case "web_fetch":
		reasons = append(reasons, "External network request")
	}

	return reasons
}
