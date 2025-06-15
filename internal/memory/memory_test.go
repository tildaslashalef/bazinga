package memory

import (
	"context"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemorySystem_LoadMemory(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "bazinga-memory-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logger := loggy.WithSource()
	ms := NewMemorySystem(logger)

	// Create test MEMORY.md file
	memoryContent := `# Test Project Memory
This is a test project memory file.

## Important Notes
- Test note 1
- Test note 2
`
	memoryPath := filepath.Join(tempDir, "MEMORY.md")
	err = os.WriteFile(memoryPath, []byte(memoryContent), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Test loading memory
	ctx := context.Background()
	memory, err := ms.LoadMemory(ctx, tempDir)
	if err != nil {
		t.Fatalf("Failed to load memory: %v", err)
	}

	if memory.ProjectMemory == "" {
		t.Error("Expected project memory to be loaded")
	}

	if !strings.Contains(memory.ProjectMemory, "Test Project Memory") {
		t.Error("Project memory content not loaded correctly")
	}

	if !strings.Contains(memory.FullContent, "Project Memory") {
		t.Error("Full content should contain project memory section")
	}
}

func TestMemorySystem_ProcessImports(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "bazinga-import-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logger := loggy.WithSource()
	ms := NewMemorySystem(logger)

	// Create imported file
	importedContent := "This is imported content from another file."
	importedPath := filepath.Join(tempDir, "imported.md")
	err = os.WriteFile(importedPath, []byte(importedContent), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Create main MEMORY.md with import
	mainContent := `# Main Memory File

Some initial content.

@imported.md

Some content after import.`
	memoryPath := filepath.Join(tempDir, "MEMORY.md")
	err = os.WriteFile(memoryPath, []byte(mainContent), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Test loading with imports
	ctx := context.Background()
	memory, err := ms.LoadMemory(ctx, tempDir)
	if err != nil {
		t.Fatalf("Failed to load memory with imports: %v", err)
	}

	if !strings.Contains(memory.ProjectMemory, importedContent) {
		t.Error("Imported content not found in project memory")
	}

	if len(memory.ImportedFiles) != 1 {
		t.Errorf("Expected 1 imported file, got %d", len(memory.ImportedFiles))
	}

	expectedPath := filepath.Join(tempDir, "imported.md")
	if memory.ImportedFiles[0] != expectedPath {
		t.Errorf("Expected imported file path %s, got %s", expectedPath, memory.ImportedFiles[0])
	}
}

func TestMemorySystem_RecursiveLookup(t *testing.T) {
	// Create nested directory structure
	tempDir, err := os.MkdirTemp("", "bazinga-recursive-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create nested directories
	subDir := filepath.Join(tempDir, "subdir", "deeper")
	err = os.MkdirAll(subDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}

	// Create MEMORY.md in parent directory
	memoryContent := "Parent directory memory content"
	memoryPath := filepath.Join(tempDir, "MEMORY.md")
	err = os.WriteFile(memoryPath, []byte(memoryContent), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	logger := loggy.WithSource()
	ms := NewMemorySystem(logger)

	// Test loading from deeper subdirectory
	ctx := context.Background()
	memory, err := ms.LoadMemory(ctx, subDir)
	if err != nil {
		t.Fatalf("Failed to load memory from subdirectory: %v", err)
	}

	if !strings.Contains(memory.ProjectMemory, "Parent directory memory content") {
		t.Error("Should have found MEMORY.md in parent directory")
	}
}

func TestMemorySystem_CreateMemoryFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bazinga-create-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logger := loggy.WithSource()
	ms := NewMemorySystem(logger)

	// Test creating project memory file
	ctx := context.Background()
	projectPath := filepath.Join(tempDir, "MEMORY.md")
	err = ms.CreateMemoryFile(ctx, projectPath, false)
	if err != nil {
		t.Fatalf("Failed to create project memory file: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		t.Error("Project memory file was not created")
	}

	// Check content
	content, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "Project Memory") {
		t.Error("Project memory template not written correctly")
	}

	// Test creating user memory file
	userDir := filepath.Join(tempDir, ".bazinga")
	userPath := filepath.Join(userDir, "MEMORY.md")
	err = ms.CreateMemoryFile(ctx, userPath, true)
	if err != nil {
		t.Fatalf("Failed to create user memory file: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(userPath); os.IsNotExist(err) {
		t.Error("User memory file was not created")
	}

	// Check content
	content, err = os.ReadFile(userPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "User Memory") {
		t.Error("User memory template not written correctly")
	}
}

func TestMemorySystem_AddQuickMemory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bazinga-quick-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logger := loggy.WithSource()
	ms := NewMemorySystem(logger)

	ctx := context.Background()
	testNote := "This is a quick test note"

	// Test adding to project memory (should create file if not exists)
	err = ms.AddQuickMemory(ctx, tempDir, testNote, false)
	if err != nil {
		t.Fatalf("Failed to add quick memory: %v", err)
	}

	// Verify file was created and contains the note
	memoryPath := filepath.Join(tempDir, "MEMORY.md")
	content, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), testNote) {
		t.Error("Quick note not found in memory file")
	}

	if !strings.Contains(string(content), "Quick Note") {
		t.Error("Quick note header not found in memory file")
	}
}

func TestMemorySystem_GetMemoryFilePaths(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bazinga-paths-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logger := loggy.WithSource()
	ms := NewMemorySystem(logger)

	// Create a project MEMORY.md file
	memoryPath := filepath.Join(tempDir, "MEMORY.md")
	err = os.WriteFile(memoryPath, []byte("test content"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	userPath, projectPath := ms.GetMemoryFilePaths(tempDir)

	// User path should always be set (even if file doesn't exist)
	if userPath == "" {
		t.Error("User path should not be empty")
	}

	if !strings.Contains(userPath, ".bazinga") {
		t.Error("User path should contain .bazinga directory")
	}

	// Project path should point to the created file
	if projectPath != memoryPath {
		t.Errorf("Expected project path %s, got %s", memoryPath, projectPath)
	}
}

func TestMemorySystem_CircularImports(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bazinga-circular-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logger := loggy.WithSource()
	ms := NewMemorySystem(logger)

	// Create file A that imports file B
	fileAContent := `File A content
@fileB.md
End of file A`
	fileAPath := filepath.Join(tempDir, "fileA.md")
	err = os.WriteFile(fileAPath, []byte(fileAContent), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Create file B that imports file A (circular)
	fileBContent := `File B content
@fileA.md
End of file B`
	fileBPath := filepath.Join(tempDir, "fileB.md")
	err = os.WriteFile(fileBPath, []byte(fileBContent), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Create main MEMORY.md that imports file A
	mainContent := `Main content
@fileA.md
End of main`
	memoryPath := filepath.Join(tempDir, "MEMORY.md")
	err = os.WriteFile(memoryPath, []byte(mainContent), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Test loading - should handle circular imports gracefully
	ctx := context.Background()
	memory, err := ms.LoadMemory(ctx, tempDir)
	if err != nil {
		t.Fatalf("Failed to load memory with circular imports: %v", err)
	}

	// Should have processed some imports but avoided infinite loops
	if len(memory.ImportedFiles) == 0 {
		t.Error("Expected some imports to be processed")
	}

	// Content should contain some imported content
	if !strings.Contains(memory.ProjectMemory, "File A content") &&
		!strings.Contains(memory.ProjectMemory, "File B content") {
		t.Error("Expected some imported content to be present")
	}
}
