package session

import (
	"context"
	"github.com/tildaslashalef/bazinga/internal/git"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAddAndRemoveFile tests adding and removing files from a session
func TestAddAndRemoveFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "session-file-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a test file
	testFilePath := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFilePath, []byte("test content"), 0o644)
	require.NoError(t, err)

	// Create session for testing
	manager, _ := setupTestSessionManager()
	ctx := context.Background()
	session, err := manager.CreateSession(ctx, &CreateOptions{Name: "File Test"})
	require.NoError(t, err)

	// Test adding the file
	err = session.AddFile(ctx, testFilePath)
	assert.NoError(t, err, "Should add file without error")
	assert.Contains(t, session.Files, testFilePath, "File should be in session's Files slice")

	// Test adding the same file again (should fail)
	err = session.AddFile(ctx, testFilePath)
	assert.Error(t, err, "Should get error when adding same file twice")

	// Test adding a non-existent file
	err = session.AddFile(ctx, filepath.Join(tmpDir, "nonexistent.txt"))
	assert.Error(t, err, "Should get error when adding non-existent file")

	// Test removing the file
	err = session.RemoveFile(ctx, testFilePath)
	assert.NoError(t, err, "Should remove file without error")
	assert.NotContains(t, session.Files, testFilePath, "File should not be in session's Files slice after removal")

	// Test removing a file that's not in the session
	err = session.RemoveFile(ctx, testFilePath)
	assert.Error(t, err, "Should get error when removing file that's not in session")
}

// TestGetFileStatus tests the file status functionality
func TestGetFileStatus(t *testing.T) {
	// Create session for testing
	manager, _ := setupTestSessionManager()
	ctx := context.Background()
	session, err := manager.CreateSession(ctx, &CreateOptions{Name: "File Status Test"})
	require.NoError(t, err)

	// Since we don't have a real git repo, it should return unknown status
	status := session.GetFileStatus("some/file.go")
	assert.Equal(t, git.StatusUnknown, status, "File status should be unknown without git repo")

	// Test GetAllFileStatuses
	// First add some files to session
	tmpDir, err := os.MkdirTemp("", "session-status-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	err = os.WriteFile(file1, []byte("content1"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("content2"), 0o644)
	require.NoError(t, err)

	// Add files to session
	err = session.AddFile(ctx, file1)
	require.NoError(t, err)
	err = session.AddFile(ctx, file2)
	require.NoError(t, err)

	// Get all statuses
	statuses := session.GetAllFileStatuses()
	assert.Len(t, statuses, 2, "Should have statuses for both files")
	assert.Equal(t, git.StatusUnknown, statuses[file1], "Status should be unknown")
	assert.Equal(t, git.StatusUnknown, statuses[file2], "Status should be unknown")
}

// TestScanForMoreFiles tests the ScanForMoreFiles functionality
func TestScanForMoreFiles(t *testing.T) {
	// Create session for testing
	manager, _ := setupTestSessionManager()
	ctx := context.Background()
	session, err := manager.CreateSession(ctx, &CreateOptions{Name: "Scan Files Test"})
	require.NoError(t, err)

	// Explicitly set project to nil to ensure error condition
	session.project = nil

	// Now scanning should error because the project is nil
	err = session.ScanForMoreFiles(ctx)
	assert.Error(t, err, "ScanForMoreFiles should error without a project")
	assert.Contains(t, err.Error(), "no project detected", "Error message should indicate no project was detected")
}
