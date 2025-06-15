package project

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectType represents different types of projects
type ProjectType string

const (
	ProjectTypeGo         ProjectType = "go"
	ProjectTypeJavaScript ProjectType = "javascript"
	ProjectTypeTypeScript ProjectType = "typescript"
	ProjectTypePython     ProjectType = "python"
	ProjectTypeRust       ProjectType = "rust"
	ProjectTypeJava       ProjectType = "java"
	ProjectTypeGeneric    ProjectType = "generic"
)

// Project represents a detected project
type Project struct {
	Type        ProjectType       `json:"type"`
	Root        string            `json:"root"`
	Name        string            `json:"name"`
	Files       []string          `json:"files"`
	Directories []string          `json:"directories"`
	Metadata    map[string]string `json:"metadata"`
	GitIgnore   []string          `json:"gitignore_patterns"`
}

// ProjectDetector handles project detection and scanning
type ProjectDetector struct {
	maxFiles      int
	maxDepth      int
	includeHidden bool
}

// NewDetector creates a new project detector
func NewDetector() *ProjectDetector {
	return &ProjectDetector{
		maxFiles:      500,   // Reasonable limit for context
		maxDepth:      5,     // Avoid deep recursion
		includeHidden: false, // Skip hidden files by default
	}
}

// DetectProject analyzes the given directory and detects project type
func (d *ProjectDetector) DetectProject(rootPath string) (*Project, error) {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if directory exists
	if _, err := os.Stat(absRoot); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", absRoot)
	}

	project := &Project{
		Root:        absRoot,
		Name:        filepath.Base(absRoot),
		Files:       make([]string, 0),
		Directories: make([]string, 0),
		Metadata:    make(map[string]string),
		GitIgnore:   make([]string, 0),
	}

	// Load .gitignore patterns first
	project.GitIgnore = d.loadGitIgnore(absRoot)

	// Detect project type
	project.Type = d.detectProjectType(absRoot)

	// Scan files based on project type
	err = d.scanProject(project)
	if err != nil {
		return nil, fmt.Errorf("failed to scan project: %w", err)
	}

	return project, nil
}

// detectProjectType determines the project type based on key files
func (d *ProjectDetector) detectProjectType(rootPath string) ProjectType {
	// Check for specific project types in order of priority

	// TypeScript (must have tsconfig.json)
	if d.fileExists(rootPath, "tsconfig.json") {
		return ProjectTypeTypeScript
	}

	// Go (go.mod is definitive)
	if d.fileExists(rootPath, "go.mod") {
		return ProjectTypeGo
	}

	// Rust (Cargo.toml is definitive)
	if d.fileExists(rootPath, "Cargo.toml") {
		return ProjectTypeRust
	}

	// Python (check multiple indicators)
	pythonFiles := []string{"requirements.txt", "pyproject.toml", "setup.py", "Pipfile"}
	for _, file := range pythonFiles {
		if d.fileExists(rootPath, file) {
			return ProjectTypePython
		}
	}

	// Java (check multiple indicators)
	javaFiles := []string{"pom.xml", "build.gradle", "gradle.properties"}
	for _, file := range javaFiles {
		if d.fileExists(rootPath, file) {
			return ProjectTypeJava
		}
	}

	// JavaScript (package.json without tsconfig.json)
	if d.fileExists(rootPath, "package.json") {
		return ProjectTypeJavaScript
	}

	return ProjectTypeGeneric
}

// fileExists checks if a file exists in the given directory
func (d *ProjectDetector) fileExists(rootPath, filename string) bool {
	filePath := filepath.Join(rootPath, filename)
	_, err := os.Stat(filePath)
	return err == nil
}

// scanProject scans the project directory for relevant files
func (d *ProjectDetector) scanProject(project *Project) error {
	extensions := d.getRelevantExtensions(project.Type)

	return filepath.Walk(project.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Skip files with errors
		}

		// Check depth limit
		relPath, _ := filepath.Rel(project.Root, path)
		depth := len(strings.Split(relPath, string(filepath.Separator)))
		if depth > d.maxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files/directories unless enabled
		if !d.includeHidden && strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check gitignore patterns
		if d.shouldIgnore(relPath, project.GitIgnore) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			project.Directories = append(project.Directories, relPath)
		} else {
			// Check if file is relevant
			if d.isRelevantFile(path, extensions) {
				project.Files = append(project.Files, relPath)

				// Stop if we've hit the file limit
				if len(project.Files) >= d.maxFiles {
					return fmt.Errorf("file limit reached")
				}
			}
		}

		return nil
	})
}

// getRelevantExtensions returns file extensions relevant to the project type
func (d *ProjectDetector) getRelevantExtensions(projectType ProjectType) []string {
	switch projectType {
	case ProjectTypeGo:
		return []string{".go", ".mod", ".sum", ".md"}
	case ProjectTypeJavaScript:
		return []string{".js", ".jsx", ".json", ".md", ".ts", ".tsx"}
	case ProjectTypeTypeScript:
		return []string{".ts", ".tsx", ".js", ".jsx", ".json", ".md"}
	case ProjectTypePython:
		return []string{".py", ".pyx", ".pyi", ".txt", ".toml", ".cfg", ".ini", ".md"}
	case ProjectTypeRust:
		return []string{".rs", ".toml", ".md"}
	case ProjectTypeJava:
		return []string{".java", ".xml", ".properties", ".gradle", ".md"}
	default:
		return []string{".md", ".txt", ".json", ".yaml", ".yml", ".toml"}
	}
}

// isRelevantFile checks if a file should be included based on extension and other criteria
func (d *ProjectDetector) isRelevantFile(filePath string, extensions []string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))

	// Check extensions
	for _, validExt := range extensions {
		if ext == validExt {
			return true
		}
	}

	// Check special files without extensions
	fileName := strings.ToLower(filepath.Base(filePath))
	specialFiles := []string{
		"readme", "license", "changelog", "makefile", "dockerfile",
		"gitignore", "gitattributes", "editorconfig",
	}

	for _, special := range specialFiles {
		if fileName == special || strings.HasPrefix(fileName, special+".") || fileName == "."+special {
			return true
		}
	}

	return false
}

// loadGitIgnore loads .gitignore patterns from the project root
func (d *ProjectDetector) loadGitIgnore(rootPath string) []string {
	gitignorePath := filepath.Join(rootPath, ".gitignore")

	file, err := os.Open(gitignorePath)
	if err != nil {
		return []string{} // No .gitignore file
	}
	defer func() { _ = file.Close() }()

	var patterns []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}

	return patterns
}

// shouldIgnore checks if a path should be ignored based on gitignore patterns
func (d *ProjectDetector) shouldIgnore(relPath string, patterns []string) bool {
	// Always ignore common directories
	commonIgnores := []string{
		"node_modules", ".git", ".svn", ".hg",
		"vendor", "target", "build", "dist",
		".vscode", ".idea", "__pycache__", ".pytest_cache",
		".DS_Store", "Thumbs.db",
	}

	pathParts := strings.Split(relPath, string(filepath.Separator))

	// Check common ignores
	for _, part := range pathParts {
		for _, ignore := range commonIgnores {
			if part == ignore {
				return true
			}
		}
	}

	// Check gitignore patterns (simplified matching)
	for _, pattern := range patterns {
		if d.matchesPattern(relPath, pattern) {
			return true
		}
	}

	return false
}

// matchesPattern provides basic gitignore pattern matching
func (d *ProjectDetector) matchesPattern(path, pattern string) bool {
	// Handle simple patterns (not full gitignore spec)
	pattern = strings.TrimSpace(pattern)

	// Directory patterns
	if strings.HasSuffix(pattern, "/") {
		pattern = strings.TrimSuffix(pattern, "/")
		return strings.Contains(path, pattern)
	}

	// Wildcard patterns (basic)
	if strings.Contains(pattern, "*") {
		// Convert to simple regex-like matching
		if strings.HasPrefix(pattern, "*.") {
			ext := strings.TrimPrefix(pattern, "*")
			return strings.HasSuffix(path, ext)
		}
	}

	// Exact match or contains
	return strings.Contains(path, pattern) || filepath.Base(path) == pattern
}

// GetProjectSummary returns a human-readable summary of the project
func (p *Project) GetProjectSummary() string {
	summary := fmt.Sprintf("Project: %s (%s)\n", p.Name, p.Type)
	summary += fmt.Sprintf("Root: %s\n", p.Root)
	summary += fmt.Sprintf("Files: %d relevant files found\n", len(p.Files))
	summary += fmt.Sprintf("Directories: %d\n", len(p.Directories))

	if len(p.GitIgnore) > 0 {
		summary += fmt.Sprintf("GitIgnore: %d patterns loaded\n", len(p.GitIgnore))
	}

	return summary
}

// GetMainFiles returns the most important files for this project type
func (p *Project) GetMainFiles() []string {
	var mainFiles []string

	// Add project-specific important files first
	switch p.Type {
	case ProjectTypeGo:
		priorities := []string{"main.go", "go.mod", "README.md", "Makefile"}
		for _, priority := range priorities {
			for _, file := range p.Files {
				if strings.HasSuffix(file, priority) {
					mainFiles = append(mainFiles, file)
				}
			}
		}
	case ProjectTypeJavaScript, ProjectTypeTypeScript:
		priorities := []string{"package.json", "index.js", "index.ts", "src/index.js", "src/index.ts", "README.md"}
		for _, priority := range priorities {
			for _, file := range p.Files {
				if strings.HasSuffix(file, priority) {
					mainFiles = append(mainFiles, file)
				}
			}
		}
	case ProjectTypePython:
		priorities := []string{"main.py", "__init__.py", "requirements.txt", "README.md"}
		for _, priority := range priorities {
			for _, file := range p.Files {
				if strings.HasSuffix(file, priority) {
					mainFiles = append(mainFiles, file)
				}
			}
		}
	}

	// If we don't have enough main files, add the first few files
	if len(mainFiles) < 5 && len(p.Files) > 0 {
		remaining := 5 - len(mainFiles)
		for i, file := range p.Files {
			if i >= remaining {
				break
			}
			// Don't add duplicates
			found := false
			for _, existing := range mainFiles {
				if existing == file {
					found = true
					break
				}
			}
			if !found {
				mainFiles = append(mainFiles, file)
			}
		}
	}

	return mainFiles
}

// GetRelevantFiles returns a broader set of relevant files for the project
// This includes main files plus important source files, up to a reasonable limit
func (p *Project) GetRelevantFiles(maxFiles int) []string {
	if maxFiles <= 0 {
		maxFiles = 15 // Default reasonable limit
	}

	var relevantFiles []string

	// Start with main files
	mainFiles := p.GetMainFiles()
	relevantFiles = append(relevantFiles, mainFiles...)

	// Add source files by priority
	switch p.Type {
	case ProjectTypeGo:
		// Add Go source files, prioritizing certain patterns
		priorities := []string{
			"cmd/", "internal/", "pkg/", // Common Go structure
			".go", // All Go files
		}
		for _, priority := range priorities {
			if len(relevantFiles) >= maxFiles {
				break
			}
			for _, file := range p.Files {
				if len(relevantFiles) >= maxFiles {
					break
				}
				// Skip if already added
				if contains(relevantFiles, file) {
					continue
				}

				if strings.Contains(file, priority) {
					relevantFiles = append(relevantFiles, file)
				}
			}
		}

	case ProjectTypeJavaScript, ProjectTypeTypeScript:
		// Add JS/TS source files
		priorities := []string{
			"src/", "lib/", "app/", // Common JS/TS structure
			"index.", "main.", "app.", // Entry points
			".js", ".ts", ".jsx", ".tsx", // Source files
		}
		for _, priority := range priorities {
			if len(relevantFiles) >= maxFiles {
				break
			}
			for _, file := range p.Files {
				if len(relevantFiles) >= maxFiles {
					break
				}
				if contains(relevantFiles, file) {
					continue
				}
				if strings.Contains(file, priority) {
					relevantFiles = append(relevantFiles, file)
				}
			}
		}

	case ProjectTypePython:
		// Add Python source files
		priorities := []string{
			"src/", "lib/", "app/", // Common Python structure
			"__init__.py", "main.py", // Entry points
			".py", // Python files
		}
		for _, priority := range priorities {
			if len(relevantFiles) >= maxFiles {
				break
			}
			for _, file := range p.Files {
				if len(relevantFiles) >= maxFiles {
					break
				}
				if contains(relevantFiles, file) {
					continue
				}
				if strings.Contains(file, priority) {
					relevantFiles = append(relevantFiles, file)
				}
			}
		}

	default:
		// For generic projects, add files by extension priority
		priorities := []string{".md", ".txt", ".json", ".yaml", ".yml"}
		for _, priority := range priorities {
			if len(relevantFiles) >= maxFiles {
				break
			}
			for _, file := range p.Files {
				if len(relevantFiles) >= maxFiles {
					break
				}
				if contains(relevantFiles, file) {
					continue
				}
				if strings.HasSuffix(file, priority) {
					relevantFiles = append(relevantFiles, file)
				}
			}
		}
	}

	return relevantFiles
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
