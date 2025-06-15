package session

import (
	"context"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/config"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"github.com/tildaslashalef/bazinga/internal/memory"
	"github.com/tildaslashalef/bazinga/internal/project"
	"github.com/tildaslashalef/bazinga/internal/storage"
	"github.com/tildaslashalef/bazinga/internal/tools"
	"github.com/tildaslashalef/bazinga/internal/watcher"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
)

// Manager manages coding sessions
type Manager struct {
	llmManager *llm.Manager
	config     *config.Config
	storage    *storage.Storage
}

// NewManager creates a new session manager
func NewManager(llmManager *llm.Manager, cfg *config.Config) *Manager {
	// Initialize storage
	storage, err := storage.NewStorage()
	if err != nil {
		loggy.Warn("Could not initialize session storage", "error", err)
	}

	return &Manager{
		llmManager: llmManager,
		config:     cfg,
		storage:    storage,
	}
}

// CreateSession creates a new coding session
func (m *Manager) CreateSession(ctx context.Context, opts *CreateOptions) (*Session, error) {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Generate session ID
	sessionID := generateSessionID()

	// Try to open git repository
	var gitRepo *git.Repository
	if repo, err := git.PlainOpen(cwd); err == nil {
		gitRepo = repo
	}

	// Initialize file watcher
	fileWatcher, err := watcher.NewFileWatcher()
	if err != nil {
		loggy.Warn("Could not initialize file watcher", "error", err)
	}

	// Initialize tool executor
	toolExecutor := tools.NewToolExecutor(cwd)

	// Initialize context manager
	contextManager := NewContextManager(m.config.LLM.MaxTokens, func(text string) int {
		// Simple token estimation: ~4 characters per token for English
		return len(text) / 4
	})

	// Initialize memory system
	logger := loggy.WithSource()
	memorySystem := memory.NewMemorySystem(logger)

	// Initialize permission manager and tool queue
	permissionManager := NewPermissionManager()

	// Create tool queue for async permission handling
	// Note: UI channel will be set later when UI is initialized
	toolQueue := NewToolQueue(nil)
	permissionManager.SetToolQueue(toolQueue)

	// Set provider from config, ensuring it has a valid value
	provider := m.config.LLM.DefaultProvider
	if provider == "" {
		// Fallback to first available provider if no default is set
		providers := m.llmManager.ListProviders()
		if len(providers) > 0 {
			provider = providers[0]
			loggy.Info("No default provider set, using first available provider", "provider", provider)
		} else {
			loggy.Warn("No providers available in session creation")
		}
	}

	session := &Session{
		ID:                sessionID,
		Name:              opts.Name,
		RootPath:          cwd,
		Provider:          provider,
		Model:             m.config.LLM.DefaultModel,
		Files:             make([]string, 0),
		History:           make([]llm.Message, 0),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		Tags:              opts.Tags,
		DryRun:            opts.DryRun,
		NoAutoCommit:      opts.NoAutoCommit,
		manager:           m,
		llmManager:        m.llmManager,
		config:            m.config,
		gitRepo:           gitRepo,
		fileWatcher:       fileWatcher,
		toolExecutor:      toolExecutor,
		contextManager:    contextManager,
		memorySystem:      memorySystem,
		permissionManager: permissionManager,
		toolQueue:         toolQueue,
	}

	// Load memory content
	if memContent, err := memorySystem.LoadMemory(ctx, cwd); err == nil {
		session.memoryContent = memContent
		logger.Info("Memory system loaded for session", "session_id", sessionID,
			"has_user_memory", memContent.UserMemory != "",
			"has_project_memory", memContent.ProjectMemory != "")
	} else {
		logger.Debug("No memory content loaded for session", "session_id", sessionID, "error", err)
	}

	// Detect project structure
	detector := project.NewDetector()
	detectedProject, err := detector.DetectProject(cwd)
	if err != nil {
		// Continue without project detection on error
		loggy.Warn("Could not detect project structure", "error", err)
	} else {
		session.project = detectedProject
		session.promptBuilder = project.NewPromptBuilder(detectedProject)

		// Enhanced auto-detection - always load key files
		if opts.AutoDetectFiles || len(opts.Files) == 0 {
			relevantFiles := session.getBazingaStyleFiles(detectedProject)
			loggy.Info("Auto-detecting relevant files", "detected_count", len(relevantFiles))

			for _, file := range relevantFiles {
				fullPath := filepath.Join(cwd, file)
				if err := session.AddFile(ctx, fullPath); err != nil {
					loggy.Warn("Could not add auto-detected file", "file", file, "error", err)
				} else {
					loggy.Debug("Auto-added file", "file", file)
				}
			}
		}
	}

	// Add initial files if provided
	for _, file := range opts.Files {
		if err := session.AddFile(ctx, file); err != nil {
			return nil, fmt.Errorf("failed to add file %s: %w", file, err)
		}
	}

	return session, nil
}

// LoadSession loads an existing session from storage
func (m *Manager) LoadSession(ctx context.Context, sessionID string) (*Session, error) {
	if m.storage == nil {
		return nil, fmt.Errorf("session storage not available")
	}

	// Load serializable session
	serializable, err := m.storage.LoadSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	// Set provider from serialized session, ensuring it has a valid value
	provider := serializable.Provider
	if provider == "" {
		// Fallback to config's default provider
		provider = m.config.LLM.DefaultProvider

		// If still empty, try to get first available provider
		if provider == "" {
			providers := m.llmManager.ListProviders()
			if len(providers) > 0 {
				provider = providers[0]
				loggy.Info("No provider set in saved session, using first available provider", "provider", provider)
			} else {
				loggy.Warn("No providers available when loading session")
			}
		}
	}

	// Reconstruct session
	session := &Session{
		ID:           serializable.ID,
		Name:         serializable.Name,
		RootPath:     serializable.RootPath,
		Provider:     provider,
		Model:        serializable.Model,
		Files:        serializable.Files,
		History:      m.restoreHistory(serializable.History),
		CreatedAt:    serializable.CreatedAt,
		UpdatedAt:    serializable.UpdatedAt,
		Tags:         serializable.Tags,
		DryRun:       serializable.DryRun,
		NoAutoCommit: serializable.NoAutoCommit,
		manager:      m,
		llmManager:   m.llmManager,
		config:       m.config,
	}

	// Try to open git repository
	if repo, err := git.PlainOpen(session.RootPath); err == nil {
		session.gitRepo = repo
	}

	// Initialize file watcher
	if fileWatcher, err := watcher.NewFileWatcher(); err == nil {
		session.fileWatcher = fileWatcher
		// Add existing files to watcher
		for _, file := range session.Files {
			_ = fileWatcher.AddFile(file)
		}
	}

	// Initialize tool executor
	session.toolExecutor = tools.NewToolExecutor(session.RootPath)

	// Initialize context manager
	session.contextManager = NewContextManager(m.config.LLM.MaxTokens, func(text string) int {
		return len(text) / 4
	})

	// Initialize memory system
	logger := loggy.WithSource()
	session.memorySystem = memory.NewMemorySystem(logger)

	// Load memory content
	if memContent, err := session.memorySystem.LoadMemory(ctx, session.RootPath); err == nil {
		session.memoryContent = memContent
		logger.Info("Memory system loaded for session", "session_id", session.ID,
			"has_user_memory", memContent.UserMemory != "",
			"has_project_memory", memContent.ProjectMemory != "")
	} else {
		logger.Debug("No memory content loaded for session", "session_id", session.ID, "error", err)
	}

	// Detect project structure
	detector := project.NewDetector()
	if detectedProject, err := detector.DetectProject(session.RootPath); err == nil {
		session.project = detectedProject
		session.promptBuilder = project.NewPromptBuilder(detectedProject)
	}

	return session, nil
}

// SaveSession saves a session to storage
func (m *Manager) SaveSession(session *Session) error {
	if m.storage == nil {
		return fmt.Errorf("session storage not available")
	}

	return m.storage.SaveSession(session)
}

// ListSavedSessions returns all saved sessions
func (m *Manager) ListSavedSessions() ([]*storage.SerializableSession, error) {
	if m.storage == nil {
		return nil, fmt.Errorf("session storage not available")
	}

	return m.storage.ListSessions()
}

// FindSessionsByRootPath finds existing sessions for a specific project directory
func (m *Manager) FindSessionsByRootPath(rootPath string) ([]*storage.SerializableSession, error) {
	if m.storage == nil {
		return nil, fmt.Errorf("session storage not available")
	}

	return m.storage.FindSessionsByRootPath(rootPath)
}

// restoreHistory converts stored history maps back to llm.Message slice
func (m *Manager) restoreHistory(historyMaps []map[string]interface{}) []llm.Message {
	var history []llm.Message

	for _, msgMap := range historyMaps {
		msg := llm.Message{}

		if role, ok := msgMap["role"].(string); ok {
			msg.Role = role
		}

		if content, ok := msgMap["content"].(string); ok {
			msg.Content = content
		}

		history = append(history, msg)
	}

	return history
}
