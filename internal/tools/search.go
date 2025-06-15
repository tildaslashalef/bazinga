package tools

import (
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Default file extensions for searching
var defaultSearchExtensions = []string{
	".go", ".py", ".js", ".ts", ".jsx", ".tsx", ".java", ".c", ".cpp", ".h", ".hpp",
	".cs", ".php", ".rb", ".rs", ".kt", ".scala", ".clj", ".hs", ".ml", ".elm",
	".txt", ".md", ".rst", ".org", ".tex", ".html", ".htm", ".xml", ".css", ".scss", ".sass",
	".json", ".yaml", ".yml", ".toml", ".ini", ".cfg", ".conf", ".properties",
	".sql", ".sh", ".bash", ".zsh", ".fish", ".ps1", ".bat", ".cmd",
	".dockerfile", ".makefile", ".cmake", ".ninja", ".gradle", ".maven",
	".R", ".m", ".swift", ".dart", ".lua", ".perl", ".pl", ".vim", ".emacs",
}

// grepFiles searches for patterns in files using ripgrep if available, fallback to native search
func (te *ToolExecutor) grepFiles(input map[string]interface{}) (string, error) {
	_, ok := input["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("pattern is required")
	}

	// Try ripgrep first for better performance
	if result, err := te.ripgrepSearch(input); err == nil {
		return result, nil
	}

	// Fallback to native search
	return te.nativeGrepSearch(input)
}

// ripgrepSearch uses ripgrep (rg) for fast searching
func (te *ToolExecutor) ripgrepSearch(input map[string]interface{}) (string, error) {
	pattern, _ := input["pattern"].(string)

	// Check if ripgrep is available
	if _, err := exec.LookPath("rg"); err != nil {
		return "", fmt.Errorf("ripgrep not available")
	}

	args := []string{"--line-number", "--no-heading", "--color=never"}

	// Handle context lines
	if contextLines, ok := input["context"].(float64); ok && contextLines > 0 {
		args = append(args, "-C", strconv.Itoa(int(contextLines)))
	}

	// Handle case sensitivity
	if ignoreCase, ok := input["ignore_case"].(bool); ok && ignoreCase {
		args = append(args, "--ignore-case")
	}

	// Handle file extensions
	if extensions, ok := input["extensions"].([]interface{}); ok && len(extensions) > 0 {
		for _, ext := range extensions {
			if extStr, ok := ext.(string); ok {
				args = append(args, "--type-add", fmt.Sprintf("custom:*%s", extStr))
				args = append(args, "--type", "custom")
			}
		}
	} else {
		// Use default extensions
		for _, ext := range defaultSearchExtensions {
			args = append(args, "--type-add", fmt.Sprintf("custom:*%s", ext))
		}
		args = append(args, "--type", "custom")
	}

	// Handle specific files
	if filesInterface, ok := input["files"]; ok {
		if filesList, ok := filesInterface.([]interface{}); ok {
			for _, fileInterface := range filesList {
				if file, ok := fileInterface.(string); ok {
					args = append(args, file)
				}
			}
		}
	}

	// Add pattern and search path
	args = append(args, pattern, ".")

	cmd := exec.Command("rg", args...)
	cmd.Dir = te.rootPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		// If ripgrep fails, return error to trigger fallback
		return "", fmt.Errorf("ripgrep failed: %w", err)
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "No matches found", nil
	}

	// Format ripgrep output
	lines := strings.Split(result, "\n")
	matchCount := len(lines)

	return fmt.Sprintf("Found %d matches:\n%s", matchCount, result), nil
}

// nativeGrepSearch provides fallback search functionality
func (te *ToolExecutor) nativeGrepSearch(input map[string]interface{}) (string, error) {
	pattern, _ := input["pattern"].(string)

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}

	var searchFiles []string

	// Get files to search
	if filesInterface, ok := input["files"]; ok {
		if filesList, ok := filesInterface.([]interface{}); ok {
			for _, fileInterface := range filesList {
				if file, ok := fileInterface.(string); ok {
					searchFiles = append(searchFiles, file)
				}
			}
		}
	}

	recursive := true // Default to recursive
	if rec, ok := input["recursive"].(bool); ok {
		recursive = rec
	}

	// Get context lines
	contextLines := 0
	if context, ok := input["context"].(float64); ok {
		contextLines = int(context)
	}

	// Get extensions filter
	var allowedExtensions map[string]bool
	if extensions, ok := input["extensions"].([]interface{}); ok && len(extensions) > 0 {
		allowedExtensions = make(map[string]bool)
		for _, ext := range extensions {
			if extStr, ok := ext.(string); ok {
				allowedExtensions[extStr] = true
			}
		}
	} else {
		// Use default extensions
		allowedExtensions = make(map[string]bool)
		for _, ext := range defaultSearchExtensions {
			allowedExtensions[ext] = true
		}
	}

	var results []SearchResult

	if len(searchFiles) == 0 {
		// Search all files in directory
		err := filepath.Walk(te.rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil //nolint:nilerr // Skip errors
			}

			if info.IsDir() && !recursive && path != te.rootPath {
				return filepath.SkipDir
			}

			if !info.IsDir() && te.shouldSearchFile(path, allowedExtensions) {
				matches, err := te.searchInFileWithContext(path, regex, contextLines)
				if err == nil && len(matches) > 0 {
					relPath, _ := filepath.Rel(te.rootPath, path)
					for _, match := range matches {
						results = append(results, SearchResult{
							File:    relPath,
							Line:    match.LineNumber,
							Content: match.Line,
							Context: match.Context,
						})
					}
				}
			}
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("failed to search files: %w", err)
		}
	} else {
		// Search specific files
		for _, file := range searchFiles {
			filePath := file
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(te.rootPath, file)
			}

			matches, err := te.searchInFileWithContext(filePath, regex, contextLines)
			if err != nil {
				loggy.Debug("Failed to search file", "file", file, "error", err)
				continue
			}

			for _, match := range matches {
				results = append(results, SearchResult{
					File:    file,
					Line:    match.LineNumber,
					Content: match.Line,
					Context: match.Context,
				})
			}
		}
	}

	if len(results) == 0 {
		return "No matches found", nil
	}

	return te.formatSearchResults(results), nil
}

// shouldSearchFile checks if a file should be searched based on extensions
func (te *ToolExecutor) shouldSearchFile(path string, allowedExtensions map[string]bool) bool {
	ext := filepath.Ext(path)
	if ext == "" {
		// Check for files without extensions (like Makefile, Dockerfile)
		basename := filepath.Base(path)
		commonFiles := map[string]bool{
			"Makefile": true, "Dockerfile": true, "Containerfile": true,
			"LICENSE": true, "README": true, "CHANGELOG": true,
			"Jenkinsfile": true, "Vagrantfile": true, "Gemfile": true,
		}
		return commonFiles[basename]
	}
	return allowedExtensions[ext]
}

// findFiles finds files matching criteria
func (te *ToolExecutor) findFiles(input map[string]interface{}) (string, error) {
	searchPath := te.rootPath
	if path, ok := input["path"].(string); ok && path != "" {
		searchPath = path
		if !filepath.IsAbs(searchPath) {
			searchPath = filepath.Join(te.rootPath, path)
		}
	}

	namePattern := ""
	if name, ok := input["name"].(string); ok {
		namePattern = name
	}

	fileType := ""
	if typ, ok := input["type"].(string); ok {
		fileType = typ
	}

	var results []string

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Skip errors
		}

		// Filter by type
		if fileType != "" {
			if fileType == "file" && info.IsDir() {
				return nil
			}
			if fileType == "dir" && !info.IsDir() {
				return nil
			}
		}

		// Filter by name pattern
		if namePattern != "" {
			matched, err := filepath.Match(namePattern, info.Name())
			if err != nil || !matched {
				return nil //nolint:nilerr
			}
		}

		relPath, _ := filepath.Rel(te.rootPath, path)
		if info.IsDir() {
			results = append(results, relPath+"/")
		} else {
			results = append(results, relPath)
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to search files: %w", err)
	}

	if len(results) == 0 {
		return "No files found matching criteria", nil
	}

	return fmt.Sprintf("Found %d files:\n%s", len(results), strings.Join(results, "\n")), nil
}

// SearchMatch represents a search match with context
type SearchMatch struct {
	LineNumber int
	Line       string
	Context    []string // Context lines around the match
}

// SearchResult represents a formatted search result
type SearchResult struct {
	File    string
	Line    int
	Content string
	Context []string
}

// searchInFileWithContext searches for pattern in a file with context lines
func (te *ToolExecutor) searchInFileWithContext(filePath string, regex *regexp.Regexp, contextLines int) ([]SearchMatch, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var matches []SearchMatch

	for i, line := range lines {
		if regex.MatchString(line) {
			match := SearchMatch{
				LineNumber: i + 1,
				Line:       strings.TrimSpace(line),
			}

			// Add context lines if requested
			if contextLines > 0 {
				start := i - contextLines
				if start < 0 {
					start = 0
				}
				end := i + contextLines + 1
				if end > len(lines) {
					end = len(lines)
				}

				for j := start; j < end; j++ {
					if j != i { // Don't include the match line in context
						match.Context = append(match.Context, strings.TrimSpace(lines[j]))
					}
				}
			}

			matches = append(matches, match)
		}
	}

	return matches, nil
}

// formatSearchResults formats search results for display
func (te *ToolExecutor) formatSearchResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No matches found"
	}

	var output []string
	output = append(output, fmt.Sprintf("Found %d matches:", len(results)))

	currentFile := ""
	for _, result := range results {
		if result.File != currentFile {
			currentFile = result.File
			output = append(output, "")
			output = append(output, fmt.Sprintf(" %s", result.File))
		}

		// Main match line
		output = append(output, fmt.Sprintf("  %d: %s", result.Line, result.Content))

		// Context lines if available
		if len(result.Context) > 0 {
			for _, contextLine := range result.Context {
				if contextLine != "" {
					output = append(output, fmt.Sprintf("     %s", contextLine))
				}
			}
		}
	}

	return strings.Join(output, "\n")
}

// fuzzySearch provides fuzzy file searching
func (te *ToolExecutor) fuzzySearch(input map[string]interface{}) (string, error) {
	query, ok := input["query"].(string)
	if !ok {
		return "", fmt.Errorf("query is required for fuzzy search")
	}

	// Try using fzf if available
	if result, err := te.fzfSearch(query); err == nil {
		return result, nil
	}

	// Fallback to native fuzzy search
	return te.nativeFuzzySearch(query)
}

// fzfSearch uses fzf for fuzzy finding
func (te *ToolExecutor) fzfSearch(query string) (string, error) {
	if _, err := exec.LookPath("fzf"); err != nil {
		return "", fmt.Errorf("fzf not available")
	}

	// Use find to get all files, pipe to fzf
	findCmd := exec.Command("find", ".", "-type", "f")
	findCmd.Dir = te.rootPath

	fzfCmd := exec.Command("fzf", "--filter", query, "--no-sort")
	fzfCmd.Dir = te.rootPath

	// Pipe find output to fzf
	fzfCmd.Stdin, _ = findCmd.StdoutPipe()

	if err := findCmd.Start(); err != nil {
		return "", fmt.Errorf("find command failed: %w", err)
	}

	output, err := fzfCmd.CombinedOutput()
	if err != nil {
		_ = findCmd.Wait()
		return "", fmt.Errorf("fzf command failed: %w", err)
	}

	_ = findCmd.Wait()

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "No files found matching query", nil
	}

	lines := strings.Split(result, "\n")
	return fmt.Sprintf("Found %d files:\n%s", len(lines), result), nil
}

// nativeFuzzySearch provides fallback fuzzy search
func (te *ToolExecutor) nativeFuzzySearch(query string) (string, error) {
	var files []string
	queryLower := strings.ToLower(query)

	err := filepath.Walk(te.rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr
		}

		if !info.IsDir() {
			relPath, _ := filepath.Rel(te.rootPath, path)
			fileName := strings.ToLower(filepath.Base(relPath))

			// Simple fuzzy matching - check if all query characters appear in order
			if te.fuzzyMatch(fileName, queryLower) {
				files = append(files, relPath)
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to search files: %w", err)
	}

	if len(files) == 0 {
		return "No files found matching query", nil
	}

	// Sort by relevance (shorter names first, then alphabetical)
	// This is a simple heuristic
	return fmt.Sprintf("Found %d files:\n%s", len(files), strings.Join(files, "\n")), nil
}

// fuzzyMatch checks if all characters in query appear in target in order
func (te *ToolExecutor) fuzzyMatch(target, query string) bool {
	if query == "" {
		return true
	}

	targetIdx := 0
	for _, queryChar := range query {
		found := false
		for targetIdx < len(target) {
			if rune(target[targetIdx]) == queryChar {
				found = true
				targetIdx++
				break
			}
			targetIdx++
		}
		if !found {
			return false
		}
	}
	return true
}
