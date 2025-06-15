package session

import (
	"errors"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Error definitions
var (
	ErrToolNotFound = errors.New("tool not found in queue")
)

// ToolExecutionState represents the current state of a tool execution
type ToolExecutionState int

const (
	ToolPending ToolExecutionState = iota
	ToolAwaitingPermission
	ToolExecuting
	ToolCompleted
	ToolDenied
	ToolCancelled
)

// String returns a string representation of the tool execution state
func (t ToolExecutionState) String() string {
	switch t {
	case ToolPending:
		return "pending"
	case ToolAwaitingPermission:
		return "awaiting_permission"
	case ToolExecuting:
		return "executing"
	case ToolCompleted:
		return "completed"
	case ToolDenied:
		return "denied"
	case ToolCancelled:
		return "canceled"
	default:
		return "unknown"
	}
}

// PendingToolCall represents a tool call awaiting permission or execution
type PendingToolCall struct {
	ID                string
	ToolCall          *llm.ToolCall
	State             ToolExecutionState
	RiskLevel         string
	RiskReasons       []string
	AffectedResources []string
	CreatedAt         time.Time
	ResponseChan      chan bool // Channel to receive user's permission decision
	PermissionManager *PermissionManager
}

// Note: PermissionDecision is defined in permissions.go

// ToolQueue manages asynchronous tool execution with permission handling
type ToolQueue struct {
	pending   map[string]*PendingToolCall
	completed []string
	mutex     sync.RWMutex
	uiChan    chan<- tea.Msg // Channel to send UI messages
}

// NewToolQueue creates a new tool queue with UI communication channel
func NewToolQueue(uiChan chan<- tea.Msg) *ToolQueue {
	return &ToolQueue{
		pending:   make(map[string]*PendingToolCall),
		completed: make([]string, 0),
		uiChan:    uiChan,
	}
}

// QueueTool adds a tool call to the queue and returns its ID
func (tq *ToolQueue) QueueTool(toolCall *llm.ToolCall, permissionManager *PermissionManager) string {
	tq.mutex.Lock()
	defer tq.mutex.Unlock()

	// Generate unique ID for this tool execution
	id := generateToolExecutionID()

	// Assess risk level
	riskLevel := permissionManager.GetToolRisk(toolCall)

	// Create pending tool call
	pendingTool := &PendingToolCall{
		ID:                id,
		ToolCall:          toolCall,
		State:             ToolPending,
		RiskLevel:         riskLevel,
		RiskReasons:       []string{}, // TODO: Extract detailed risk reasons
		AffectedResources: extractAffectedResources(toolCall),
		CreatedAt:         time.Now(),
		ResponseChan:      make(chan bool, 1), // Buffered channel for non-blocking send
		PermissionManager: permissionManager,
	}

	// Add to pending queue
	tq.pending[id] = pendingTool

	return id
}

// ApproveTool approves a tool for execution
func (tq *ToolQueue) ApproveTool(id string, remember bool) error {
	tq.mutex.Lock()
	defer tq.mutex.Unlock()

	pendingTool, exists := tq.pending[id]
	if !exists {
		return ErrToolNotFound
	}

	// Update state and send approval
	pendingTool.State = ToolExecuting

	// Send approval through channel (non-blocking)
	select {
	case pendingTool.ResponseChan <- true:
		// Successfully sent approval
	default:
		// Channel full or closed, tool might have timed out
	}

	return nil
}

// DenyTool denies a tool execution
func (tq *ToolQueue) DenyTool(id string, remember bool) error {
	tq.mutex.Lock()
	defer tq.mutex.Unlock()

	pendingTool, exists := tq.pending[id]
	if !exists {
		return ErrToolNotFound
	}

	// Update state and send denial
	pendingTool.State = ToolDenied

	// Send denial through channel (non-blocking)
	select {
	case pendingTool.ResponseChan <- false:
		// Successfully sent denial
	default:
		// Channel full or closed, tool might have timed out
	}

	// Move to completed
	tq.completed = append(tq.completed, id)
	delete(tq.pending, id)

	return nil
}

// CompleteTool marks a tool as completed and removes it from the queue
func (tq *ToolQueue) CompleteTool(id string) error {
	tq.mutex.Lock()
	defer tq.mutex.Unlock()

	pendingTool, exists := tq.pending[id]
	if !exists {
		return ErrToolNotFound
	}

	// Update state
	pendingTool.State = ToolCompleted

	// Move to completed
	tq.completed = append(tq.completed, id)
	delete(tq.pending, id)

	return nil
}

// GetPendingTools returns all pending tool calls
func (tq *ToolQueue) GetPendingTools() []*PendingToolCall {
	tq.mutex.RLock()
	defer tq.mutex.RUnlock()

	tools := make([]*PendingToolCall, 0, len(tq.pending))
	for _, tool := range tq.pending {
		tools = append(tools, tool)
	}

	return tools
}

// GetTool returns a specific tool call by ID
func (tq *ToolQueue) GetTool(id string) (*PendingToolCall, bool) {
	tq.mutex.RLock()
	defer tq.mutex.RUnlock()

	tool, exists := tq.pending[id]
	return tool, exists
}

// GetDecisionChannel returns the response channel for a specific tool
func (tq *ToolQueue) GetDecisionChannel(id string) (chan bool, bool) {
	tq.mutex.RLock()
	defer tq.mutex.RUnlock()

	tool, exists := tq.pending[id]
	if !exists {
		return nil, false
	}

	return tool.ResponseChan, true
}

// SendPermissionRequest sends a permission request to the UI
func (tq *ToolQueue) SendPermissionRequest(id string) error {
	tq.mutex.RLock()
	tool, exists := tq.pending[id]
	tq.mutex.RUnlock()

	if !exists {
		return ErrToolNotFound
	}

	// Update state to awaiting permission
	tq.mutex.Lock()
	tool.State = ToolAwaitingPermission
	tq.mutex.Unlock()

	// Send permission request to UI if channel is available
	if tq.uiChan != nil {
		promptText := tool.PermissionManager.FormatPermissionPrompt(tool.ToolCall)

		// Note: We'll need to create this message type that UI can handle
		permissionMsg := struct {
			ToolID        string
			ToolCall      *llm.ToolCall
			RiskLevel     string
			RiskReasons   []string
			AffectedFiles []string
			QueuePosition int
			TotalQueued   int
			PromptText    string
			ResponseChan  chan bool
		}{
			ToolID:        id,
			ToolCall:      tool.ToolCall,
			RiskLevel:     tool.RiskLevel,
			RiskReasons:   tool.RiskReasons,
			AffectedFiles: tool.AffectedResources,
			QueuePosition: tq.getQueuePosition(id),
			TotalQueued:   len(tq.pending),
			PromptText:    promptText,
			ResponseChan:  tool.ResponseChan,
		}

		// Send to UI (non-blocking)
		select {
		case tq.uiChan <- permissionMsg:
			// Successfully sent to UI
		default:
			// UI channel full, permission request might be lost
			// TODO: Handle this case better
		}
	}

	return nil
}

// getQueuePosition returns the position of a tool in the queue (1-based)
func (tq *ToolQueue) getQueuePosition(id string) int {
	position := 1
	for _, tool := range tq.pending {
		if tool.ID == id {
			return position
		}
		if tool.State == ToolAwaitingPermission {
			position++
		}
	}
	return position
}

// extractAffectedResources extracts list of resources that will be affected by the tool
func extractAffectedResources(toolCall *llm.ToolCall) []string {
	resources := []string{}

	// Extract file paths
	if filePath, ok := toolCall.Input["file_path"].(string); ok {
		resources = append(resources, filePath)
	}

	// Extract command details for bash
	if toolCall.Name == "bash" {
		if command, ok := toolCall.Input["command"].(string); ok {
			resources = append(resources, "command: "+command)
		}
	}

	// Extract URLs for web fetch
	if toolCall.Name == "web_fetch" {
		if url, ok := toolCall.Input["url"].(string); ok {
			resources = append(resources, "url: "+url)
		}
	}

	return resources
}

// generateToolExecutionID generates a unique ID for tool execution tracking
func generateToolExecutionID() string {
	// Use timestamp-based ID for now
	return time.Now().Format("20060102-150405.000000")
}

// getAvailableTools returns the available tools for the session with enhanced context
func (s *Session) getAvailableTools() []llm.Tool {
	if s.toolExecutor == nil {
		return []llm.Tool{}
	}

	tools := s.toolExecutor.GetAvailableTools()

	// Enhance tool descriptions with memory and project context
	if s.memoryContent != nil || s.project != nil {
		for i := range tools {
			switch tools[i].Name {
			case "read_file", "write_file", "edit_file", "create_file":
				// Add project context to file operations
				if s.project != nil {
					projectInfo := fmt.Sprintf("\n\nProject Context: %s project with %d files",
						s.project.Type, len(s.project.Files))
					tools[i].Description += projectInfo
				}

				// Add memory context for file operations
				if s.memoryContent != nil && s.memoryContent.ProjectMemory != "" {
					memoryInfo := fmt.Sprintf("\n\nProject Memory: %s", s.memoryContent.ProjectMemory)
					tools[i].Description += memoryInfo
				}

			case "bash":
				// Add project-specific context
				if s.project != nil {
					commandInfo := fmt.Sprintf("\n\nProject Type: %s", s.project.Type)
					tools[i].Description += commandInfo
				}
			}
		}
	}

	return tools
}
