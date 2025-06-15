package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// MockSession implements SessionInterface for testing
type MockSession struct {
	id           string
	name         string
	rootPath     string
	provider     string
	model        string
	files        []string
	tags         []string
	dryRun       bool
	noAutoCommit bool
	createdAt    time.Time
	updatedAt    time.Time
	history      []map[string]interface{}
}

func (m *MockSession) GetID() string                        { return m.id }
func (m *MockSession) GetName() string                      { return m.name }
func (m *MockSession) GetRootPath() string                  { return m.rootPath }
func (m *MockSession) GetProvider() string                  { return m.provider }
func (m *MockSession) GetModel() string                     { return m.model }
func (m *MockSession) GetFiles() []string                   { return m.files }
func (m *MockSession) GetTags() []string                    { return m.tags }
func (m *MockSession) GetDryRun() bool                      { return m.dryRun }
func (m *MockSession) GetNoAutoCommit() bool                { return m.noAutoCommit }
func (m *MockSession) GetCreatedAt() time.Time              { return m.createdAt }
func (m *MockSession) GetUpdatedAt() time.Time              { return m.updatedAt }
func (m *MockSession) GetHistory() []map[string]interface{} { return m.history }

// setupTestStorage creates a test storage with temporary directory
func setupTestStorage(t *testing.T) (*Storage, string) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "bazinga-storage-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create sessions directory
	sessionsDir := filepath.Join(tempDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("Failed to create sessions dir: %v", err)
	}

	// Create storage with temp directory
	storage := &Storage{
		dataDir: tempDir,
	}

	return storage, tempDir
}

// TestSaveLoadSession tests saving and loading a session
func TestSaveLoadSession(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	// Create mock session
	now := time.Now()
	mockSession := &MockSession{
		id:           "test-session-123",
		name:         "Test Session",
		rootPath:     "/test/path",
		provider:     "bedrock",
		model:        "anthropic.claude-3-sonnet",
		files:        []string{"file1.go", "file2.go"},
		tags:         []string{"go", "test"},
		dryRun:       false,
		noAutoCommit: true,
		createdAt:    now.Add(-1 * time.Hour),
		updatedAt:    now,
		history: []map[string]interface{}{
			{"role": "user", "content": "Hello"},
			{"role": "assistant", "content": "Hi there!"},
		},
	}

	// Test saving
	err := storage.SaveSession(mockSession)
	if err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Test loading
	loaded, err := storage.LoadSession("test-session-123")
	if err != nil {
		t.Fatalf("Failed to load session: %v", err)
	}

	// Verify session data
	if loaded.ID != mockSession.id {
		t.Errorf("Expected session ID %s, got %s", mockSession.id, loaded.ID)
	}
	if loaded.Name != mockSession.name {
		t.Errorf("Expected session name %s, got %s", mockSession.name, loaded.Name)
	}
	if loaded.Provider != mockSession.provider {
		t.Errorf("Expected provider %s, got %s", mockSession.provider, loaded.Provider)
	}
	if loaded.Model != mockSession.model {
		t.Errorf("Expected model %s, got %s", mockSession.model, loaded.Model)
	}
	if len(loaded.Files) != len(mockSession.files) {
		t.Errorf("Expected %d files, got %d", len(mockSession.files), len(loaded.Files))
	}
	if len(loaded.Tags) != len(mockSession.tags) {
		t.Errorf("Expected %d tags, got %d", len(mockSession.tags), len(loaded.Tags))
	}
}

// TestListSessions tests listing saved sessions
func TestListSessions(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	// Create multiple mock sessions
	sessions := []*MockSession{
		{
			id:        "session-1",
			name:      "Session 1",
			rootPath:  "/test/path1",
			provider:  "bedrock",
			model:     "anthropic.claude-3-opus",
			createdAt: time.Now(),
			updatedAt: time.Now(),
			history:   []map[string]interface{}{{"role": "user", "content": "test1"}},
		},
		{
			id:        "session-2",
			name:      "Session 2",
			rootPath:  "/test/path2",
			provider:  "openai",
			model:     "gpt-4",
			createdAt: time.Now(),
			updatedAt: time.Now(),
			history:   []map[string]interface{}{{"role": "user", "content": "test2"}},
		},
	}

	// Save all sessions
	for _, session := range sessions {
		err := storage.SaveSession(session)
		if err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}
	}

	// Test listing
	listed, err := storage.ListSessions()
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	// Verify list count
	if len(listed) != len(sessions) {
		t.Errorf("Expected %d sessions, got %d", len(sessions), len(listed))
	}

	// Verify session IDs
	foundIDs := make(map[string]bool)
	for _, sess := range listed {
		foundIDs[sess.ID] = true
	}

	for _, expected := range sessions {
		if !foundIDs[expected.id] {
			t.Errorf("Session %s not found in listed sessions", expected.id)
		}
	}
}

// TestDeleteSession tests deleting a session
func TestDeleteSession(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	// Create mock session
	mockSession := &MockSession{
		id:        "delete-test",
		name:      "Delete Test",
		rootPath:  "/test/path",
		provider:  "bedrock",
		model:     "anthropic.claude-3-sonnet",
		createdAt: time.Now(),
		updatedAt: time.Now(),
		history:   []map[string]interface{}{{"role": "user", "content": "delete test"}},
	}

	// Save the session
	err := storage.SaveSession(mockSession)
	if err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Verify it exists
	_, err = storage.LoadSession(mockSession.id)
	if err != nil {
		t.Fatalf("Session should exist before deletion: %v", err)
	}

	// Delete session
	err = storage.DeleteSession(mockSession.id)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify it's deleted
	_, err = storage.LoadSession(mockSession.id)
	if err == nil {
		t.Error("Expected error when loading deleted session, got nil")
	}
}

// TestCleanupOldSessions tests cleaning up old sessions
func TestCleanupOldSessions(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	now := time.Now()

	// Create sessions with different ages
	sessions := []*MockSession{
		{
			id:        "new-session",
			name:      "New Session",
			updatedAt: now, // Current time, should be kept
			history:   []map[string]interface{}{{"role": "user", "content": "new"}},
		},
		{
			id:        "old-session",
			name:      "Old Session",
			updatedAt: now.Add(-48 * time.Hour), // 48 hours old, should be removed
			history:   []map[string]interface{}{{"role": "user", "content": "old"}},
		},
		{
			id:        "medium-session",
			name:      "Medium Session",
			updatedAt: now.Add(-12 * time.Hour), // 12 hours old, should be kept
			history:   []map[string]interface{}{{"role": "user", "content": "medium"}},
		},
	}

	// Save all sessions
	for _, session := range sessions {
		err := storage.SaveSession(session)
		if err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}
	}

	// Clean up sessions older than 24 hours
	err := storage.CleanupOldSessions(24 * time.Hour)
	if err != nil {
		t.Fatalf("Failed to cleanup sessions: %v", err)
	}

	// Check which sessions remain
	remaining, err := storage.ListSessions()
	if err != nil {
		t.Fatalf("Failed to list sessions after cleanup: %v", err)
	}

	// Should have 2 sessions left
	if len(remaining) != 2 {
		t.Errorf("Expected 2 sessions after cleanup, got %d", len(remaining))
	}

	// Old session should be gone
	for _, sess := range remaining {
		if sess.ID == "old-session" {
			t.Error("Old session should have been removed")
		}
	}
}

// TestHistoryTruncation tests the history truncation functionality
func TestHistoryTruncation(t *testing.T) {
	storage, tempDir := setupTestStorage(t)
	defer os.RemoveAll(tempDir)

	// Create a mock session with large history
	largeHistory := make([]map[string]interface{}, 100) // More than maxMessages (50)
	for i := 0; i < 100; i++ {
		largeHistory[i] = map[string]interface{}{
			"role":    "user",
			"content": fmt.Sprintf("Message %d", i),
		}
	}

	mockSession := &MockSession{
		id:        "truncate-test",
		name:      "Truncate Test",
		rootPath:  "/test/path",
		provider:  "bedrock",
		model:     "anthropic.claude-3-sonnet",
		createdAt: time.Now(),
		updatedAt: time.Now(),
		history:   largeHistory,
	}

	// Test saving with truncation
	err := storage.SaveSession(mockSession)
	if err != nil {
		t.Fatalf("Failed to save session with large history: %v", err)
	}

	// Load and verify truncation
	loaded, err := storage.LoadSession("truncate-test")
	if err != nil {
		t.Fatalf("Failed to load session: %v", err)
	}

	// Should be truncated to maxMessages (50) or less
	if len(loaded.History) > 50 {
		t.Errorf("History not truncated properly: got %d messages, expected <= 50", len(loaded.History))
	}

	// Should keep the most recent messages
	if len(loaded.History) > 0 {
		lastMsg, ok := loaded.History[len(loaded.History)-1]["content"].(string)
		if !ok || lastMsg != "Message 99" {
			t.Errorf("Last message not preserved correctly: got %v, expected 'Message 99'", lastMsg)
		}
	}
}
