package session

import (
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/config"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"github.com/tildaslashalef/bazinga/internal/memory"
	"github.com/tildaslashalef/bazinga/internal/project"
	"github.com/tildaslashalef/bazinga/internal/tools"
	"github.com/tildaslashalef/bazinga/internal/watcher"
	"math/rand"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
)

// Session represents an active coding session
type Session struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	RootPath     string        `json:"root_path"`
	Provider     string        `json:"provider"`
	Model        string        `json:"model"`
	Files        []string      `json:"files"`
	History      []llm.Message `json:"history"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	Tags         []string      `json:"tags"`
	DryRun       bool          `json:"dry_run"`
	NoAutoCommit bool          `json:"no_auto_commit"`

	// Runtime dependencies
	manager           *Manager
	llmManager        *llm.Manager
	config            *config.Config
	gitRepo           *git.Repository
	project           *project.Project
	promptBuilder     *project.PromptBuilder
	fileWatcher       *watcher.FileWatcher
	toolExecutor      *tools.ToolExecutor
	contextManager    *ContextManager
	memorySystem      *memory.MemorySystem
	memoryContent     *memory.MemoryContent
	permissionManager *PermissionManager
	toolQueue         *ToolQueue
}

// CreateOptions contains options for creating a new session
type CreateOptions struct {
	Name            string
	Tags            []string
	Files           []string
	DryRun          bool
	NoAutoCommit    bool
	AutoDetectFiles bool // Automatically detect and add project files
}

// GetFiles returns the list of files in the session
func (s *Session) GetFiles() []string {
	return s.Files
}

// GetProvider returns the current provider
func (s *Session) GetProvider() string {
	return s.Provider
}

// GetModel returns the current model
func (s *Session) GetModel() string {
	return s.Model
}

// SetModel sets the current model
func (s *Session) SetModel(model string) error {
	s.Model = model
	s.UpdatedAt = time.Now()
	loggy.Debug("Set model", "model", model)
	return nil
}

// SetProvider sets the current provider
func (s *Session) SetProvider(provider string) error {
	// Empty provider name is not allowed
	if provider == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	// Verify provider exists
	p, err := s.llmManager.GetProvider(provider)
	if err != nil {
		return fmt.Errorf("provider not available: %w", err)
	}

	// Ensure we actually got a provider
	if p == nil {
		return fmt.Errorf("provider '%s' resolved to nil", provider)
	}

	s.Provider = provider
	s.UpdatedAt = time.Now()
	loggy.Debug("Session provider changed", "provider", provider)
	return nil
}

// GetAvailableProviders returns list of available providers
func (s *Session) GetAvailableProviders() []string {
	return s.llmManager.ListProviders()
}

// GetAvailableModels returns available models by provider
func (s *Session) GetAvailableModels() map[string][]llm.Model {
	return s.llmManager.GetAvailableModels()
}

// AddSystemMessage adds a system message to the session history
func (s *Session) AddSystemMessage(message string) error {
	systemMsg := llm.Message{
		Role:    "system",
		Content: message,
	}
	s.History = append(s.History, systemMsg)
	return nil
}

// GetProject returns the detected project information
func (s *Session) GetProject() *project.Project {
	return s.project
}

// GetProjectSummary returns a summary of the detected project
func (s *Session) GetProjectSummary() string {
	if s.project == nil {
		return "No project detected"
	}
	return s.project.GetProjectSummary()
}

// GetFileWatcher returns the file watcher for this session
func (s *Session) GetFileWatcher() *watcher.FileWatcher {
	return s.fileWatcher
}

// GetToolExecutor returns the tool executor for this session
func (s *Session) GetToolExecutor() *tools.ToolExecutor {
	return s.toolExecutor
}

// GetPermissionManager returns the permission manager for this session
func (s *Session) GetPermissionManager() *PermissionManager {
	return s.permissionManager
}

// GetToolQueue returns the tool queue for this session
func (s *Session) GetToolQueue() *ToolQueue {
	return s.toolQueue
}

// IsTerminatorMode returns whether terminator mode is enabled (bypasses all permissions)
func (s *Session) IsTerminatorMode() bool {
	if s.config == nil {
		return false
	}
	return s.config.Security.Terminator
}

// Save saves the session to storage
func (s *Session) Save() error {
	if s.manager == nil {
		return fmt.Errorf("session manager not available")
	}
	s.UpdatedAt = time.Now()
	return s.manager.SaveSession(s)
}

// Close properly closes the session and cleans up resources
func (s *Session) Close() error {
	// Save session before closing
	if err := s.Save(); err != nil {
		loggy.Error("Failed to auto-save session on close", "session_id", s.ID, "error", err)
	}

	if s.fileWatcher != nil {
		return s.fileWatcher.Close()
	}
	return nil
}

// Getter methods for storage interface
func (s *Session) GetID() string           { return s.ID }
func (s *Session) GetName() string         { return s.Name }
func (s *Session) GetRootPath() string     { return s.RootPath }
func (s *Session) GetTags() []string       { return s.Tags }
func (s *Session) GetDryRun() bool         { return s.DryRun }
func (s *Session) GetNoAutoCommit() bool   { return s.NoAutoCommit }
func (s *Session) GetCreatedAt() time.Time { return s.CreatedAt }
func (s *Session) GetUpdatedAt() time.Time { return s.UpdatedAt }
func (s *Session) GetHistory() []map[string]interface{} {
	// Convert llm.Message slice to map slice for storage
	var history []map[string]interface{}
	for _, msg := range s.History {
		msgMap := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
		history = append(history, msgMap)
	}
	return history
}

// GetMemoryContent returns the current memory content
func (s *Session) GetMemoryContent() *memory.MemoryContent {
	return s.memoryContent
}

// generateSessionID generates a unique session ID using Unix nanoseconds and random component
func generateSessionID() string {
	return fmt.Sprintf("sess_%d_%04d", time.Now().UnixNano(), rand.Intn(10000))
}

// getBazingaStyleFiles intelligently selects the most relevant project files
func (s *Session) getBazingaStyleFiles(project *project.Project) []string {
	if project == nil {
		return []string{}
	}

	var files []string
	maxFiles := 12 // Load 10-15 most relevant files initially

	// Priority 1: Essential project files (always include)
	essentialFiles := []string{
		"README.md", "readme.md", "Readme.md",
		"package.json", "go.mod", "Cargo.toml", "pyproject.toml", "requirements.txt",
		"Makefile", "makefile", "CMakeLists.txt",
		"tsconfig.json", "webpack.config.js", "vite.config.js",
		".gitignore", "LICENSE", "CHANGELOG.md",
	}

	for _, essential := range essentialFiles {
		if len(files) >= maxFiles {
			break
		}
		for _, projectFile := range project.Files {
			if strings.HasSuffix(projectFile, essential) {
				if !s.contains(files, projectFile) {
					files = append(files, projectFile)
				}
				break
			}
		}
	}

	// Priority 2: Main entry points by project type
	entryPoints := s.getProjectEntryPoints(project)
	for _, entry := range entryPoints {
		if len(files) >= maxFiles {
			break
		}
		for _, projectFile := range project.Files {
			if strings.HasSuffix(projectFile, entry) || strings.Contains(projectFile, entry) {
				if !s.contains(files, projectFile) {
					files = append(files, projectFile)
				}
			}
		}
	}

	// Priority 3: Key architectural files
	architecturalFiles := s.getArchitecturalFiles(project)
	for _, arch := range architecturalFiles {
		if len(files) >= maxFiles {
			break
		}
		for _, projectFile := range project.Files {
			if strings.Contains(projectFile, arch) {
				if !s.contains(files, projectFile) {
					files = append(files, projectFile)
				}
			}
		}
	}

	// Priority 4: Fill remaining slots with relevant source files
	if len(files) < maxFiles {
		sourceFiles := s.getRelevantSourceFiles(project, maxFiles-len(files))
		for _, source := range sourceFiles {
			if !s.contains(files, source) {
				files = append(files, source)
			}
		}
	}

	return files
}

// getProjectEntryPoints returns entry point files based on project type
func (s *Session) getProjectEntryPoints(project *project.Project) []string {
	switch project.Type {
	case "go":
		return []string{"main.go", "cmd/", "app.go"}
	case "javascript", "typescript":
		return []string{"index.js", "index.ts", "app.js", "app.ts", "src/index", "src/app", "src/main"}
	case "python":
		return []string{"main.py", "app.py", "__init__.py", "src/", "app/"}
	case "rust":
		return []string{"main.rs", "lib.rs", "src/main.rs", "src/lib.rs"}
	case "java":
		return []string{"Main.java", "Application.java", "src/main/"}
	default:
		return []string{"main", "index", "app"}
	}
}

// getArchitecturalFiles returns important architectural files
func (s *Session) getArchitecturalFiles(project *project.Project) []string {
	switch project.Type {
	case "go":
		return []string{"internal/", "pkg/", "api/", "cmd/", "config/"}
	case "javascript", "typescript":
		return []string{"src/", "lib/", "components/", "pages/", "api/", "config/"}
	case "python":
		return []string{"src/", "lib/", "api/", "config/", "models/", "views/"}
	case "rust":
		return []string{"src/", "benches/", "examples/"}
	case "java":
		return []string{"src/main/java/", "src/main/resources/", "src/test/"}
	default:
		return []string{"src/", "lib/", "config/"}
	}
}

// getRelevantSourceFiles returns additional relevant source files
func (s *Session) getRelevantSourceFiles(project *project.Project, maxFiles int) []string {
	var sourceFiles []string
	extensions := s.getSourceExtensions(project.Type)

	for _, projectFile := range project.Files {
		if len(sourceFiles) >= maxFiles {
			break
		}

		// Check if file has relevant extension
		for _, ext := range extensions {
			if strings.HasSuffix(projectFile, ext) {
				// Skip test files and vendor directories
				if !s.shouldSkipFile(projectFile) {
					sourceFiles = append(sourceFiles, projectFile)
					break
				}
			}
		}
	}

	return sourceFiles
}

// getSourceExtensions returns relevant file extensions for a project type
func (s *Session) getSourceExtensions(projectType project.ProjectType) []string {
	switch projectType {
	case "go":
		return []string{".go"}
	case "javascript":
		return []string{".js", ".jsx", ".mjs"}
	case "typescript":
		return []string{".ts", ".tsx", ".js", ".jsx"}
	case "python":
		return []string{".py", ".pyi"}
	case "rust":
		return []string{".rs"}
	case "java":
		return []string{".java"}
	default:
		return []string{".js", ".ts", ".py", ".go", ".rs", ".java", ".cpp", ".c", ".h"}
	}
}

// shouldSkipFile determines if a file should be skipped during auto-detection
func (s *Session) shouldSkipFile(filePath string) bool {
	skipPatterns := []string{
		"_test.", "test_", ".test.",
		"node_modules/", "vendor/", ".git/",
		"build/", "dist/", "target/",
		".min.", ".bundle.",
		"__pycache__/", ".pyc",
	}

	for _, pattern := range skipPatterns {
		if strings.Contains(filePath, pattern) {
			return true
		}
	}

	return false
}

// contains checks if a slice contains a string
func (s *Session) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
