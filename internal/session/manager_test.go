package session

import (
	"context"
	"github.com/tildaslashalef/bazinga/internal/config"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewManager tests the creation of a new manager
func TestNewManager(t *testing.T) {
	llmManager := newMockLLMManager()
	cfg := &config.Config{}
	manager := NewManager(llmManager, cfg)

	assert.NotNil(t, manager, "Manager should be created")
	assert.NotNil(t, manager.storage, "Storage should be initialized")
}

// TestCreateAndSaveSession tests creating and saving a session
func TestCreateAndSaveSession(t *testing.T) {
	manager, _ := setupTestSessionManager()
	ctx := context.Background()

	// Test session creation
	opts := &CreateOptions{
		Name:  "Test Manager Session",
		Tags:  []string{"test", "manager"},
		Files: []string{},
	}

	session, err := manager.CreateSession(ctx, opts)
	require.NoError(t, err)
	require.NotNil(t, session)

	assert.Equal(t, "Test Manager Session", session.Name)
	assert.Equal(t, "openai", session.Provider)
	assert.ElementsMatch(t, []string{"test", "manager"}, session.Tags)
	assert.NotNil(t, session.toolExecutor)
	assert.NotNil(t, session.contextManager)

	// Test saving
	err = manager.SaveSession(session)
	assert.NoError(t, err)
}

// TestListSavedSessions tests listing saved sessions
func TestListSavedSessions(t *testing.T) {
	manager, _ := setupTestSessionManager()
	ctx := context.Background()

	// Create a test session
	session, err := manager.CreateSession(ctx, &CreateOptions{
		Name: "Test List Session",
	})
	require.NoError(t, err)

	// Save the session
	err = manager.SaveSession(session)
	require.NoError(t, err)

	// List saved sessions
	sessions, err := manager.ListSavedSessions()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(sessions), 1, "Should have at least one session")

	// Check if our session is in the list
	found := false
	for _, s := range sessions {
		if s.ID == session.ID {
			found = true
			assert.Equal(t, "Test List Session", s.Name)
			break
		}
	}
	assert.True(t, found, "Created session should be in the list")
}

// TestLoadSession tests loading a session
func TestLoadSession(t *testing.T) {
	manager, _ := setupTestSessionManager()
	ctx := context.Background()

	// Create a test session with unique characteristics
	originalSession, err := manager.CreateSession(ctx, &CreateOptions{
		Name:  "Test Load Session",
		Tags:  []string{"load", "test"},
		Files: []string{},
	})
	require.NoError(t, err)

	// Add a system message to the session
	err = originalSession.AddSystemMessage("Test system message")
	require.NoError(t, err)

	// Modify and save the session
	originalSession.Model = "custom-model"
	err = manager.SaveSession(originalSession)
	require.NoError(t, err)

	// Load the session
	loadedSession, err := manager.LoadSession(ctx, originalSession.ID)
	assert.NoError(t, err)
	assert.NotNil(t, loadedSession)

	// Verify session properties
	assert.Equal(t, originalSession.ID, loadedSession.ID)
	assert.Equal(t, "Test Load Session", loadedSession.Name)
	assert.ElementsMatch(t, []string{"load", "test"}, loadedSession.Tags)
	assert.Equal(t, "custom-model", loadedSession.Model)

	// Verify dependencies are initialized
	assert.NotNil(t, loadedSession.toolExecutor)
	assert.NotNil(t, loadedSession.contextManager)
	assert.NotNil(t, loadedSession.fileWatcher)
}

// TestFindSessionsByRootPath tests finding sessions by root path
func TestFindSessionsByRootPath(t *testing.T) {
	manager, _ := setupTestSessionManager()
	ctx := context.Background()

	// Get current working directory
	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Create a test session
	session, err := manager.CreateSession(ctx, &CreateOptions{
		Name: "Test Find Session",
	})
	require.NoError(t, err)

	// Save the session
	err = manager.SaveSession(session)
	require.NoError(t, err)

	// Find sessions for current path
	sessions, err := manager.FindSessionsByRootPath(cwd)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(sessions), 1, "Should find at least one session")

	// Check if our session is found
	found := false
	for _, s := range sessions {
		if s.ID == session.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "Created session should be found by root path")
}
