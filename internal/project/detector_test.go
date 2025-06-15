package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectProjectType(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "bazinga-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	detector := NewDetector()

	tests := []struct {
		name     string
		files    []string
		expected ProjectType
	}{
		{
			name:     "Go project",
			files:    []string{"go.mod", "main.go"},
			expected: ProjectTypeGo,
		},
		{
			name:     "JavaScript project",
			files:    []string{"package.json", "index.js"},
			expected: ProjectTypeJavaScript,
		},
		{
			name:     "TypeScript project",
			files:    []string{"tsconfig.json", "package.json"},
			expected: ProjectTypeTypeScript,
		},
		{
			name:     "Python project",
			files:    []string{"requirements.txt", "main.py"},
			expected: ProjectTypePython,
		},
		{
			name:     "Rust project",
			files:    []string{"Cargo.toml", "src/main.rs"},
			expected: ProjectTypeRust,
		},
		{
			name:     "Generic project",
			files:    []string{"README.md", "some-file.txt"},
			expected: ProjectTypeGeneric,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test directory
			testDir := filepath.Join(tmpDir, tt.name)
			err := os.MkdirAll(testDir, 0o755)
			if err != nil {
				t.Fatalf("Failed to create test dir: %v", err)
			}

			// Create test files
			for _, file := range tt.files {
				filePath := filepath.Join(testDir, file)

				// Create directory if needed
				dir := filepath.Dir(filePath)
				if dir != testDir {
					err := os.MkdirAll(dir, 0o755)
					if err != nil {
						t.Fatalf("Failed to create dir %s: %v", dir, err)
					}
				}

				// Create file
				f, err := os.Create(filePath)
				if err != nil {
					t.Fatalf("Failed to create file %s: %v", filePath, err)
				}
				_ = f.Close()
			}

			// Test detection
			detected := detector.detectProjectType(testDir)
			if detected != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, detected)
			}
		})
	}
}

func TestDetectProject(t *testing.T) {
	// Create a temporary Go project for testing
	tmpDir, err := os.MkdirTemp("", "bazinga-go-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create Go project structure
	files := map[string]string{
		"go.mod":     "module test-project\n\ngo 1.21\n",
		"main.go":    "package main\n\nfunc main() {}\n",
		"utils.go":   "package main\n\nfunc helper() {}\n",
		"README.md":  "# Test Project\n",
		".gitignore": "*.log\nbuild/\n",
	}

	for filePath, content := range files {
		fullPath := filepath.Join(tmpDir, filePath)
		err := os.WriteFile(fullPath, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	// Test project detection
	detector := NewDetector()
	project, err := detector.DetectProject(tmpDir)
	if err != nil {
		t.Fatalf("Failed to detect project: %v", err)
	}

	// Verify results
	if project.Type != ProjectTypeGo {
		t.Errorf("Expected Go project, got %s", project.Type)
	}

	if project.Name != filepath.Base(tmpDir) {
		t.Errorf("Expected project name %s, got %s", filepath.Base(tmpDir), project.Name)
	}

	if len(project.Files) == 0 {
		t.Error("Expected some files to be detected")
	}

	// Check that Go files were detected
	foundGoFiles := false
	for _, file := range project.Files {
		if filepath.Ext(file) == ".go" {
			foundGoFiles = true
			break
		}
	}
	if !foundGoFiles {
		t.Error("Expected Go files to be detected")
	}

	// Check gitignore patterns
	if len(project.GitIgnore) == 0 {
		t.Error("Expected .gitignore patterns to be loaded")
	}
}

func TestGetRelevantExtensions(t *testing.T) {
	detector := NewDetector()

	tests := []struct {
		projectType ProjectType
		expectedExt string
	}{
		{ProjectTypeGo, ".go"},
		{ProjectTypeJavaScript, ".js"},
		{ProjectTypeTypeScript, ".ts"},
		{ProjectTypePython, ".py"},
		{ProjectTypeRust, ".rs"},
		{ProjectTypeJava, ".java"},
	}

	for _, tt := range tests {
		t.Run(string(tt.projectType), func(t *testing.T) {
			extensions := detector.getRelevantExtensions(tt.projectType)

			found := false
			for _, ext := range extensions {
				if ext == tt.expectedExt {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected extension %s for project type %s", tt.expectedExt, tt.projectType)
			}
		})
	}
}

func TestIsRelevantFile(t *testing.T) {
	detector := NewDetector()
	extensions := []string{".go", ".md", ".txt"}

	tests := []struct {
		filePath string
		expected bool
	}{
		{"main.go", true},
		{"README.md", true},
		{"notes.txt", true},
		{"Makefile", true},
		{"Dockerfile", true},
		{"script.sh", false},
		{"image.png", false},
		{".gitignore", true},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			result := detector.isRelevantFile(tt.filePath, extensions)
			if result != tt.expected {
				t.Errorf("Expected %t for file %s, got %t", tt.expected, tt.filePath, result)
			}
		})
	}
}

func TestShouldIgnore(t *testing.T) {
	detector := NewDetector()
	patterns := []string{"*.log", "build/", "temp"}

	tests := []struct {
		path     string
		expected bool
	}{
		{"main.go", false},
		{"src/utils.go", false},
		{"debug.log", true},
		{"build/output", true},
		{"node_modules/package", true},
		{".git/config", true},
		{"temp", true},
		{"temp/file.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := detector.shouldIgnore(tt.path, patterns)
			if result != tt.expected {
				t.Errorf("Expected %t for path %s, got %t", tt.expected, tt.path, result)
			}
		})
	}
}

func TestProjectGetMainFiles(t *testing.T) {
	// Test Go project main files
	goProject := &Project{
		Type:  ProjectTypeGo,
		Files: []string{"main.go", "utils.go", "go.mod", "README.md", "test.go"},
	}

	mainFiles := goProject.GetMainFiles()

	// Should prioritize main.go, go.mod, README.md
	expectedPriorities := []string{"main.go", "go.mod", "README.md"}

	for _, expected := range expectedPriorities {
		found := false
		for _, file := range mainFiles {
			if filepath.Base(file) == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected main file %s not found in %v", expected, mainFiles)
		}
	}
}
