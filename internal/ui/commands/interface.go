package commands

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Command represents a slash command interface
type Command interface {
	Execute(ctx context.Context, args []string, model CommandModel) tea.Msg
	GetName() string
	GetUsage() string
	GetDescription() string
}

// CommandModel provides access to the UI model for commands
type CommandModel interface {
	GetSession() Session
	GetSessionManager() SessionManager
	LoadFiles()
	AddMessage(role, content string, streaming bool)
}

// Session interface for command access
type Session interface {
	GetFiles() []string
	GetProject() Project
	GetRootPath() string
	AddFile(ctx context.Context, path string) error
	GetDiffOutput() (string, error)
	CommitChanges(ctx context.Context, message string) error
	CommitWithAI(ctx context.Context) (string, error)
	SetModel(model string) error
	GetModel() string
	SetProvider(provider string) error
	GetProvider() string
	GetAvailableProviders() []string
	GetAvailableModels() map[string][]ModelInfo
	GetProjectSummary() string
	GetBranchInfo() (string, error)
	GetCommitHistory(limit int) (string, error)
	GetMemoryContent() *MemoryContent
	GetMemoryFilePaths() (string, string)
	CreateMemoryFile(ctx context.Context, isUserMemory bool) error
	ReloadMemory(ctx context.Context) error
	AddQuickMemory(ctx context.Context, note string, isUserMemory bool) error
	GetPermissionManager() PermissionManager
	ID() string
}

// SessionManager interface for command access
type SessionManager interface {
	SaveSession(session Session) error
	ListSavedSessions() ([]SavedSessionInfo, error)
}

// Project interface for command access
type Project interface {
	GetRelevantFiles(limit int) []string
	Root() string
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role      string
	Content   string
	Timestamp time.Time
	Streaming bool
}

// ModelInfo represents model information
type ModelInfo struct {
	ID   string
	Name string
}

// MemoryContent represents memory content
type MemoryContent struct {
	UserMemory    string
	ProjectMemory string
	ImportedFiles []string
}

// SavedSessionInfo represents saved session information
type SavedSessionInfo struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// PermissionManager interface for command access
type PermissionManager interface {
	GetToolRisk(toolCall interface{}) string
}

// ResponseMsg represents a command response message
type ResponseMsg struct {
	Content string
}

// StatusUpdateMsg represents a status update message
type StatusUpdateMsg struct {
	Response  string
	ModelName string
}

// LLMRequestMsg represents a request to send a message to the LLM
type LLMRequestMsg struct {
	Message string
}
