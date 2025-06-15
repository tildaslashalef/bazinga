package memory

import (
	"context"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MemorySystem handles MEMORY.md file discovery and content management
type MemorySystem struct {
	logger *loggy.Logger
}

// MemoryContent represents the parsed content from MEMORY.md files
type MemoryContent struct {
	ProjectMemory string   // Content from ./MEMORY.md
	UserMemory    string   // Content from ~/.github.com/tildaslashalef/bazinga/MEMORY.md
	ImportedFiles []string // List of imported file paths
	FullContent   string   // Combined and processed content
}

// NewMemorySystem creates a new memory system instance
func NewMemorySystem(logger *loggy.Logger) *MemorySystem {
	return &MemorySystem{
		logger: logger,
	}
}

// LoadMemory discovers and loads all MEMORY.md files with imports
func (ms *MemorySystem) LoadMemory(ctx context.Context, workingDir string) (*MemoryContent, error) {
	content := &MemoryContent{
		ImportedFiles: make([]string, 0),
	}

	// Load user-level memory first (~/.github.com/tildaslashalef/bazinga/MEMORY.md)
	if userMemory, err := ms.loadUserMemory(ctx); err == nil {
		content.UserMemory = userMemory
		ms.logger.Debug("Loaded user memory", "path", "~/.github.com/tildaslashalef/bazinga/MEMORY.md")
	} else {
		ms.logger.Debug("No user memory found", "error", err)
	}

	// Load project memory with recursive lookup
	if projectMemory, err := ms.loadProjectMemory(ctx, workingDir); err == nil {
		content.ProjectMemory = projectMemory
		ms.logger.Debug("Loaded project memory", "dir", workingDir)
	} else {
		ms.logger.Debug("No project memory found", "error", err)
	}

	// Process imports and combine content
	if err := ms.processImports(ctx, content, workingDir); err != nil {
		return nil, fmt.Errorf("failed to process imports: %w", err)
	}

	// Combine all content
	content.FullContent = ms.combineContent(content)

	ms.logger.Info("Memory system loaded",
		"has_user_memory", content.UserMemory != "",
		"has_project_memory", content.ProjectMemory != "",
		"imported_files", len(content.ImportedFiles))

	return content, nil
}

// loadUserMemory loads ~/.github.com/tildaslashalef/bazinga/MEMORY.md if it exists
func (ms *MemorySystem) loadUserMemory(ctx context.Context) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	bazingaDir := filepath.Join(homeDir, ".bazinga")
	memoryPath := filepath.Join(bazingaDir, "MEMORY.md")

	return ms.readMemoryFile(memoryPath)
}

// loadProjectMemory recursively searches for MEMORY.md from workingDir to root
func (ms *MemorySystem) loadProjectMemory(ctx context.Context, workingDir string) (string, error) {
	currentDir := workingDir

	for {
		memoryPath := filepath.Join(currentDir, "MEMORY.md")

		if content, err := ms.readMemoryFile(memoryPath); err == nil {
			ms.logger.Debug("Found project MEMORY.md", "path", memoryPath)
			return content, nil
		}

		// Move up one directory
		parentDir := filepath.Dir(currentDir)

		// Stop if we've reached the root or can't go up further
		if parentDir == currentDir || parentDir == "/" {
			break
		}

		currentDir = parentDir
	}

	return "", fmt.Errorf("no MEMORY.md found in directory hierarchy")
}

// readMemoryFile reads and returns the content of a memory file
func (ms *MemorySystem) readMemoryFile(path string) (string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}

	return string(content), nil
}

// processImports processes @path/to/import syntax and loads imported files
func (ms *MemorySystem) processImports(ctx context.Context, content *MemoryContent, workingDir string) error {
	// Process user memory imports
	if content.UserMemory != "" {
		homeDir, _ := os.UserHomeDir()
		bazingaDir := filepath.Join(homeDir, ".bazinga")
		if err := ms.processImportsInText(ctx, &content.UserMemory, bazingaDir, content); err != nil {
			return fmt.Errorf("failed to process user memory imports: %w", err)
		}
	}

	// Process project memory imports
	if content.ProjectMemory != "" {
		if err := ms.processImportsInText(ctx, &content.ProjectMemory, workingDir, content); err != nil {
			return fmt.Errorf("failed to process project memory imports: %w", err)
		}
	}

	return nil
}

// processImportsInText processes imports within a specific text and base directory
func (ms *MemorySystem) processImportsInText(ctx context.Context, text *string, baseDir string, content *MemoryContent) error {
	importRegex := regexp.MustCompile(`@([^\s\n]+)`)

	// Find all import statements
	imports := importRegex.FindAllStringSubmatch(*text, -1)

	for _, match := range imports {
		if len(match) < 2 {
			continue
		}

		importPath := match[1]
		fullPath := filepath.Join(baseDir, importPath)

		// Avoid circular imports
		alreadyImported := false
		for _, imported := range content.ImportedFiles {
			if imported == fullPath {
				alreadyImported = true
				break
			}
		}

		if alreadyImported {
			ms.logger.Debug("Skipping circular import", "path", fullPath)
			continue
		}

		// Read the imported file
		importedContent, err := ms.readMemoryFile(fullPath)
		if err != nil {
			ms.logger.Debug("Failed to import file", "path", fullPath, "error", err)
			continue
		}

		// Track imported file
		content.ImportedFiles = append(content.ImportedFiles, fullPath)

		// Replace the import statement with the file content
		*text = strings.Replace(*text, match[0], importedContent, 1)

		ms.logger.Debug("Imported memory file", "path", fullPath)

		// Recursively process imports in the imported content
		if err := ms.processImportsInText(ctx, text, filepath.Dir(fullPath), content); err != nil {
			return err
		}
	}

	return nil
}

// combineContent combines user and project memory into a single string
// Note: This is mainly for backward compatibility - the session prompt builder
// now handles formatting in a more structured way
func (ms *MemorySystem) combineContent(content *MemoryContent) string {
	var parts []string

	// Simple combination without heavy formatting since prompts.go handles that now
	if content.UserMemory != "" {
		parts = append(parts, content.UserMemory)
		parts = append(parts, "")
	}

	if content.ProjectMemory != "" {
		parts = append(parts, content.ProjectMemory)
		parts = append(parts, "")
	}

	// Remove trailing empty line if present
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	return strings.Join(parts, "\n")
}

// CreateMemoryFile creates a new MEMORY.md file with template content
func (ms *MemorySystem) CreateMemoryFile(ctx context.Context, path string, isUserMemory bool) error {
	var template string

	if isUserMemory {
		template = `# User Memory

This file contains your personal preferences and instructions that apply across all projects.

## Coding Preferences
- Preferred programming style
- Common patterns to follow
- Libraries and frameworks to use

## Communication Style
- How you prefer the AI to respond
- Level of detail in explanations
- Preferred formats for output

## Custom Instructions
Add any global instructions here...
`
	} else {
		template = `# Project Memory

This file contains project-specific context and instructions for this codebase.

## Project Overview
Brief description of what this project does...

## Architecture
Key architectural decisions and patterns...

## Development Guidelines
- Coding standards specific to this project
- Testing requirements
- Deployment procedures

## Important Context
Any crucial information the AI should know about this project...

## File Structure
Key files and directories to be aware of...
`
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write the template
	if err := os.WriteFile(path, []byte(template), 0o644); err != nil {
		return fmt.Errorf("failed to write memory file %s: %w", path, err)
	}

	ms.logger.Info("Created memory file", "path", path, "is_user_memory", isUserMemory)
	return nil
}

// AddQuickMemory appends a quick note to the appropriate MEMORY.md file
func (ms *MemorySystem) AddQuickMemory(ctx context.Context, workingDir, note string, isUserMemory bool) error {
	var targetPath string

	if isUserMemory {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		bazingaDir := filepath.Join(homeDir, ".bazinga")
		targetPath = filepath.Join(bazingaDir, "MEMORY.md")

		// Create user memory file if it doesn't exist
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			if err := ms.CreateMemoryFile(ctx, targetPath, true); err != nil {
				return err
			}
		}
	} else {
		// Find or create project MEMORY.md
		targetPath = filepath.Join(workingDir, "MEMORY.md")

		// Create project memory file if it doesn't exist
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			if err := ms.CreateMemoryFile(ctx, targetPath, false); err != nil {
				return err
			}
		}
	}

	// Append the note with timestamp
	file, err := os.OpenFile(targetPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open memory file: %w", err)
	}
	defer file.Close()

	timestamp := fmt.Sprintf("\n\n## Quick Note (Added by bazinga)\n%s\n", note)
	if _, err := file.WriteString(timestamp); err != nil {
		return fmt.Errorf("failed to write to memory file: %w", err)
	}

	ms.logger.Info("Added quick memory", "path", targetPath, "note_length", len(note))
	return nil
}

// GetMemoryFilePaths returns the paths to relevant MEMORY.md files
func (ms *MemorySystem) GetMemoryFilePaths(workingDir string) (userPath, projectPath string) {
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		userPath = filepath.Join(homeDir, ".bazinga", "MEMORY.md")
	}

	// Find project memory file with recursive lookup
	currentDir := workingDir
	for {
		memoryPath := filepath.Join(currentDir, "MEMORY.md")
		if _, err := os.Stat(memoryPath); err == nil {
			projectPath = memoryPath
			break
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir || parentDir == "/" {
			break
		}
		currentDir = parentDir
	}

	// If no existing project memory found, use working directory
	if projectPath == "" {
		projectPath = filepath.Join(workingDir, "MEMORY.md")
	}

	return userPath, projectPath
}
