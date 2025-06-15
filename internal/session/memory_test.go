package session

import (
	"context"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"github.com/tildaslashalef/bazinga/internal/memory"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetMemoryFilePaths tests the GetMemoryFilePaths function
func TestGetMemoryFilePaths(t *testing.T) {
	// Create a test logger
	logger := loggy.NewTestLogger()
	memSys := memory.NewMemorySystem(logger)

	// Setup session with the memory system
	s := &Session{
		RootPath:     "/test/root",
		memorySystem: memSys,
	}

	// Verify the method doesn't panic when called
	assert.NotPanics(t, func() {
		s.GetMemoryFilePaths()
	})
}

// TestGetMemoryFilePathsNoMemorySystem tests GetMemoryFilePaths with nil memorySystem
func TestGetMemoryFilePathsNoMemorySystem(t *testing.T) {
	// Setup: Session with nil memorySystem
	s := &Session{}
	s.memorySystem = nil

	// Execute
	userPath, projectPath := s.GetMemoryFilePaths()

	// Verify
	assert.Equal(t, "", userPath)
	assert.Equal(t, "", projectPath)
}

// TestCreateMemoryFile tests the CreateMemoryFile function with a temporary directory
func TestCreateMemoryFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "bazinga_memory_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a test logger
	logger := loggy.NewTestLogger()
	memSys := memory.NewMemorySystem(logger)

	// Setup session with the memory system
	s := &Session{
		RootPath:     tmpDir,
		memorySystem: memSys,
	}

	// Test creating a memory file
	ctx := context.Background()

	// Test execution - should not panic
	assert.NotPanics(t, func() {
		_ = s.CreateMemoryFile(ctx, false)
	})
}

// TestCreateMemoryFileNoMemorySystem tests CreateMemoryFile with nil memorySystem
func TestCreateMemoryFileNoMemorySystem(t *testing.T) {
	// Setup: Session with nil memorySystem
	ctx := context.Background()
	s := &Session{}
	s.memorySystem = nil

	// Execute
	err := s.CreateMemoryFile(ctx, true)

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "memory system not available")
}

// TestAddQuickMemory tests the AddQuickMemory function with a temporary directory
func TestAddQuickMemory(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "bazinga_memory_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a test logger
	logger := loggy.NewTestLogger()
	memSys := memory.NewMemorySystem(logger)

	// Setup session with the memory system
	s := &Session{
		RootPath:     tmpDir,
		memorySystem: memSys,
	}

	// Test adding a quick memory note - just make sure it doesn't panic
	ctx := context.Background()
	assert.NotPanics(t, func() {
		_ = s.AddQuickMemory(ctx, "Test note", false)
	})
}

// TestAddQuickMemoryNoMemorySystem tests AddQuickMemory with nil memorySystem
func TestAddQuickMemoryNoMemorySystem(t *testing.T) {
	// Setup: Session with nil memorySystem
	ctx := context.Background()
	s := &Session{}
	s.memorySystem = nil

	// Execute
	err := s.AddQuickMemory(ctx, "Test note", true)

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "memory system not available")
}

// TestReloadMemory tests the ReloadMemory function with a temporary directory
func TestReloadMemory(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "bazinga_memory_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a memory file in the tmp directory
	memFilePath := filepath.Join(tmpDir, "MEMORY.md")
	content := "# Test Memory\nThis is a test memory file."
	err = os.WriteFile(memFilePath, []byte(content), 0o644)
	require.NoError(t, err)

	// Create a test logger
	logger := loggy.NewTestLogger()
	memSys := memory.NewMemorySystem(logger)

	// Setup session with the memory system
	s := &Session{
		RootPath:     tmpDir,
		memorySystem: memSys,
	}

	// Test reloading memory - just make sure it doesn't panic
	ctx := context.Background()
	assert.NotPanics(t, func() {
		_ = s.ReloadMemory(ctx)
	})

	// If the memory reload was successful, memoryContent should be non-nil
	if err := s.ReloadMemory(ctx); err == nil {
		assert.NotNil(t, s.memoryContent, "Memory content should be non-nil after successful reload")
	}
}

// TestReloadMemoryNoMemorySystem tests ReloadMemory with nil memorySystem
func TestReloadMemoryNoMemorySystem(t *testing.T) {
	// Setup: Session with nil memorySystem
	ctx := context.Background()
	s := &Session{}
	s.memorySystem = nil

	// Execute
	err := s.ReloadMemory(ctx)

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "memory system not available")
}
