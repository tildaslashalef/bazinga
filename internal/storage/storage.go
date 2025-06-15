package storage

import (
	"encoding/json"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/config"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	// Default directory names
	sessionsDir = "sessions"
)

// SessionInterface defines the interface for sessions that can be saved
type SessionInterface interface {
	GetID() string
	GetName() string
	GetRootPath() string
	GetProvider() string
	GetModel() string
	GetFiles() []string
	GetTags() []string
	GetDryRun() bool
	GetNoAutoCommit() bool
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
	GetHistory() []map[string]interface{}
}

// Storage manages session persistence
type Storage struct {
	dataDir string
	config  *config.Config
}

// NewStorage creates a new storage manager with default settings
func NewStorage() (*Storage, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return NewStorageWithConfig(cfg)
}

// NewStorageWithConfig creates a new storage manager with the provided config
func NewStorageWithConfig(cfg *config.Config) (*Storage, error) {
	// Get config directory
	configDir, err := config.GetConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config directory: %w", err)
	}

	dataDir := configDir

	// Ensure sessions directory exists
	if err := os.MkdirAll(filepath.Join(dataDir, sessionsDir), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &Storage{
		dataDir: dataDir,
		config:  cfg,
	}, nil
}

// SerializableSession represents a session that can be saved/loaded
type SerializableSession struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	RootPath     string    `json:"root_path"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	Files        []string  `json:"files"`
	Tags         []string  `json:"tags"`
	DryRun       bool      `json:"dry_run"`
	NoAutoCommit bool      `json:"no_auto_commit"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// History with smart truncation to prevent large files
	History []map[string]interface{} `json:"history,omitempty"`
}

// SaveSession saves a session to disk
func (s *Storage) SaveSession(sess SessionInterface) error {
	// Convert to serializable format
	serializable := &SerializableSession{
		ID:           sess.GetID(),
		Name:         sess.GetName(),
		RootPath:     sess.GetRootPath(),
		Provider:     sess.GetProvider(),
		Model:        sess.GetModel(),
		Files:        sess.GetFiles(),
		Tags:         sess.GetTags(),
		DryRun:       sess.GetDryRun(),
		NoAutoCommit: sess.GetNoAutoCommit(),
		CreatedAt:    sess.GetCreatedAt(),
		UpdatedAt:    sess.GetUpdatedAt(),
		History:      s.truncateHistory(sess.GetHistory()),
	}

	// Create session file path
	sessionPath := filepath.Join(s.GetSessionsDir(), sess.GetID()+".json")

	// Marshal to JSON
	data, err := json.MarshalIndent(serializable, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Write to file
	if err := os.WriteFile(sessionPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// LoadSession loads a session from disk
func (s *Storage) LoadSession(sessionID string) (*SerializableSession, error) {
	sessionPath := filepath.Join(s.GetSessionsDir(), sessionID+".json")

	// Read file
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	// Unmarshal from JSON
	var serializable SerializableSession
	if err := json.Unmarshal(data, &serializable); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &serializable, nil
}

// ListSessions returns all saved sessions
func (s *Storage) ListSessions() ([]*SerializableSession, error) {
	sessionsPath := s.GetSessionsDir()

	var sessions []*SerializableSession

	err := filepath.WalkDir(sessionsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && filepath.Ext(path) == ".json" {
			sessionID := filepath.Base(path[:len(path)-5]) // Remove .json
			session, err := s.LoadSession(sessionID)
			if err != nil {
				// Skip corrupted sessions
				return nil //nolint:nilerr
			}
			sessions = append(sessions, session)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	return sessions, nil
}

// FindSessionsByRootPath returns sessions for a specific project directory
func (s *Storage) FindSessionsByRootPath(rootPath string) ([]*SerializableSession, error) {
	allSessions, err := s.ListSessions()
	if err != nil {
		return nil, err
	}

	var matchingSessions []*SerializableSession
	for _, session := range allSessions {
		if session.RootPath == rootPath {
			matchingSessions = append(matchingSessions, session)
		}
	}

	// Sort by UpdatedAt (most recent first)
	sort.Slice(matchingSessions, func(i, j int) bool {
		return matchingSessions[i].UpdatedAt.After(matchingSessions[j].UpdatedAt)
	})

	return matchingSessions, nil
}

// DeleteSession removes a session from disk
func (s *Storage) DeleteSession(sessionID string) error {
	sessionPath := filepath.Join(s.GetSessionsDir(), sessionID+".json")

	if err := os.Remove(sessionPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// GetSessionsDir returns the sessions directory path
func (s *Storage) GetSessionsDir() string {
	return filepath.Join(s.dataDir, sessionsDir)
}

// CleanupOldSessions removes sessions older than the specified duration or config-defined retention period
func (s *Storage) CleanupOldSessions(maxAge time.Duration) error {
	// If maxAge is 0 and config is available, use retention period from config
	if maxAge == 0 && s.config != nil && s.config.Logging.MaxAge > 0 {
		maxAge = time.Duration(s.config.Logging.MaxAge) * 24 * time.Hour
	}

	// Use default if still not set
	if maxAge == 0 {
		maxAge = 30 * 24 * time.Hour // Default: 30 days
	}
	sessions, err := s.ListSessions()
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-maxAge)

	for _, session := range sessions {
		if session.UpdatedAt.Before(cutoff) {
			if err := s.DeleteSession(session.ID); err != nil {
				// Log error but continue cleanup
				fmt.Printf("Warning: failed to delete old session %s: %v\n", session.ID, err)
			}
		}
	}

	return nil
}

// truncateHistory implements smart history truncation to prevent large files
func (s *Storage) truncateHistory(history []map[string]interface{}) []map[string]interface{} {
	if len(history) == 0 {
		return history
	}

	const (
		maxMessages  = 50     // Keep last 50 messages
		maxSizeBytes = 100000 // 100KB limit per session
	)

	// First truncate by message count
	startIdx := 0
	if len(history) > maxMessages {
		startIdx = len(history) - maxMessages
	}

	truncated := history[startIdx:]

	// Then check size and truncate further if needed
	for len(truncated) > 10 { // Keep at least 10 messages
		// Estimate JSON size
		if data, err := json.Marshal(truncated); err == nil {
			if len(data) <= maxSizeBytes {
				break
			}
		}

		// Remove oldest message and try again
		truncated = truncated[1:]
	}

	return truncated
}
