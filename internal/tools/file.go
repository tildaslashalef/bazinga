package tools

import (
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"os"
	"path/filepath"
	"strings"
)

// FileChange represents a file modification for diff display
type FileChange struct {
	FilePath  string
	Before    string
	After     string
	Operation string // "edit", "create", "write"
}

// readFile reads the contents of a file
func (te *ToolExecutor) readFile(input map[string]interface{}) (string, error) {
	loggy.Debug("ToolExecutor readFile", "input", input, "input_length", len(input))

	filePath, ok := input["file_path"].(string)
	if !ok {
		return "", fmt.Errorf("file_path is required")
	}

	// Resolve relative path
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(te.rootPath, filePath)
	}

	loggy.Debug("ToolExecutor readFile", "resolved_path", filePath)

	content, err := os.ReadFile(filePath)
	if err != nil {
		loggy.Error("ToolExecutor readFile failed", "path", filePath, "error", err)
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Count lines for display summary
	lines := strings.Count(string(content), "\n") + 1
	if len(content) == 0 {
		lines = 0
	}

	// Get relative path for display
	var displayPath string
	if relPath, err := filepath.Rel(te.rootPath, filePath); err == nil && !strings.HasPrefix(relPath, "..") {
		displayPath = relPath
	} else {
		displayPath = filepath.Base(filePath)
	}

	loggy.Info("ToolExecutor readFile success", "path", filePath, "size", len(content), "lines", lines)

	// Return content with line count for display
	return fmt.Sprintf("File: %s\nLines: %d\nContent:\n\n%s", displayPath, lines, string(content)), nil
}

// writeFile writes content to a file
func (te *ToolExecutor) writeFile(input map[string]interface{}) (string, error) {
	filePath, ok := input["file_path"].(string)
	if !ok {
		return "", fmt.Errorf("file_path is required")
	}

	content, ok := input["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is required")
	}

	// Resolve relative path
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(te.rootPath, filePath)
	}

	// Capture before state for diff
	var beforeContent string
	if existingContent, err := os.ReadFile(filePath); err == nil {
		beforeContent = string(existingContent)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	err := os.WriteFile(filePath, []byte(content), 0o644)
	if err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	// Call file change callback for diff display
	if te.fileChangeCallback != nil {
		// Convert to relative path for display
		displayPath := filePath
		if relPath, err := filepath.Rel(te.rootPath, filePath); err == nil {
			displayPath = relPath
		}

		te.fileChangeCallback(FileChange{
			FilePath:  displayPath,
			Before:    beforeContent,
			After:     content,
			Operation: "write",
		})
	}

	return fmt.Sprintf("File %s written successfully (%d bytes)", filePath, len(content)), nil
}

// createFile creates a new file with content
func (te *ToolExecutor) createFile(input map[string]interface{}) (string, error) {
	filePath, ok := input["file_path"].(string)
	if !ok {
		return "", fmt.Errorf("file_path is required")
	}

	content, ok := input["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is required")
	}

	// Resolve relative path
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(te.rootPath, filePath)
	}

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		return "", fmt.Errorf("file %s already exists", filePath)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	err := os.WriteFile(filePath, []byte(content), 0o644)
	if err != nil {
		return "", fmt.Errorf("failed to create file %s: %w", filePath, err)
	}

	// Call file change callback for diff display
	if te.fileChangeCallback != nil {
		// Convert to relative path for display
		displayPath := filePath
		if relPath, err := filepath.Rel(te.rootPath, filePath); err == nil {
			displayPath = relPath
		}

		te.fileChangeCallback(FileChange{
			FilePath:  displayPath,
			Before:    "", // Empty since it's a new file
			After:     content,
			Operation: "create",
		})
	}

	return fmt.Sprintf("File %s created successfully (%d bytes)", filePath, len(content)), nil
}

// editFile edits a file by replacing text
func (te *ToolExecutor) editFile(input map[string]interface{}) (string, error) {
	filePath, ok := input["file_path"].(string)
	if !ok {
		return "", fmt.Errorf("file_path is required")
	}

	oldText, ok := input["old_text"].(string)
	if !ok {
		return "", fmt.Errorf("old_text is required")
	}

	newText, ok := input["new_text"].(string)
	if !ok {
		return "", fmt.Errorf("new_text is required")
	}

	// Resolve relative path
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(te.rootPath, filePath)
	}

	// Read current content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	contentStr := string(content)

	// Check if old text exists
	if !strings.Contains(contentStr, oldText) {
		return "", fmt.Errorf("old text not found in file %s", filePath)
	}

	// Replace text
	newContentStr := strings.Replace(contentStr, oldText, newText, 1)

	// Write back to file
	err = os.WriteFile(filePath, []byte(newContentStr), 0o644)
	if err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	// Call file change callback for diff display
	if te.fileChangeCallback != nil {
		// Convert to relative path for display
		displayPath := filePath
		if relPath, err := filepath.Rel(te.rootPath, filePath); err == nil {
			displayPath = relPath
		}

		te.fileChangeCallback(FileChange{
			FilePath:  displayPath,
			Before:    contentStr,
			After:     newContentStr,
			Operation: "edit",
		})
	}

	return fmt.Sprintf("File %s edited successfully", filePath), nil
}

// multiEditFile performs multiple edits on a file in sequence
func (te *ToolExecutor) multiEditFile(input map[string]interface{}) (string, error) {
	filePath, ok := input["file_path"].(string)
	if !ok {
		return "", fmt.Errorf("file_path is required")
	}

	editsInterface, ok := input["edits"]
	if !ok {
		return "", fmt.Errorf("edits array is required")
	}

	edits, ok := editsInterface.([]interface{})
	if !ok {
		return "", fmt.Errorf("edits must be an array of edit objects")
	}

	if len(edits) == 0 {
		return "", fmt.Errorf("at least one edit is required")
	}

	// Resolve relative path
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(te.rootPath, filePath)
	}

	// Read current content
	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	contentStr := string(originalContent)
	currentContent := contentStr

	// Apply each edit in sequence
	editCount := 0
	for i, editInterface := range edits {
		edit, ok := editInterface.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("edit %d must be an object", i+1)
		}

		oldText, ok := edit["old_text"].(string)
		if !ok {
			return "", fmt.Errorf("edit %d: old_text is required", i+1)
		}

		newText, ok := edit["new_text"].(string)
		if !ok {
			return "", fmt.Errorf("edit %d: new_text is required", i+1)
		}

		// Check if old text exists in current content
		if !strings.Contains(currentContent, oldText) {
			return "", fmt.Errorf("edit %d: old text not found in file", i+1)
		}

		// Apply the replacement
		currentContent = strings.Replace(currentContent, oldText, newText, 1)
		editCount++
	}

	// Write back to file if anything changed
	if currentContent != contentStr {
		err = os.WriteFile(filePath, []byte(currentContent), 0o644)
		if err != nil {
			return "", fmt.Errorf("failed to write file %s: %w", filePath, err)
		}

		// Call file change callback for diff display
		if te.fileChangeCallback != nil {
			// Convert to relative path for display
			displayPath := filePath
			if relPath, err := filepath.Rel(te.rootPath, filePath); err == nil {
				displayPath = relPath
			}

			te.fileChangeCallback(FileChange{
				FilePath:  displayPath,
				Before:    contentStr,
				After:     currentContent,
				Operation: "multi_edit",
			})
		}

		return fmt.Sprintf("File %s: applied %d edits successfully", filePath, editCount), nil
	}

	return fmt.Sprintf("File %s: no changes needed", filePath), nil
}

// moveFile moves or renames a file
func (te *ToolExecutor) moveFile(input map[string]interface{}) (string, error) {
	sourcePath, ok := input["source_path"].(string)
	if !ok {
		return "", fmt.Errorf("source_path is required")
	}

	destPath, ok := input["dest_path"].(string)
	if !ok {
		return "", fmt.Errorf("dest_path is required")
	}

	// Resolve relative paths
	if !filepath.IsAbs(sourcePath) {
		sourcePath = filepath.Join(te.rootPath, sourcePath)
	}
	if !filepath.IsAbs(destPath) {
		destPath = filepath.Join(te.rootPath, destPath)
	}

	// Check if source exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return "", fmt.Errorf("source file %s does not exist", sourcePath)
	}

	// Read source content for diff tracking before moving
	var beforeContent string
	if content, err := os.ReadFile(sourcePath); err == nil {
		beforeContent = string(content)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	// Check if destination already exists
	if _, err := os.Stat(destPath); err == nil {
		return "", fmt.Errorf("destination file %s already exists", destPath)
	}

	// Move the file
	err := os.Rename(sourcePath, destPath)
	if err != nil {
		return "", fmt.Errorf("failed to move file from %s to %s: %w", sourcePath, destPath, err)
	}

	// Call file change callback for diff display
	if te.fileChangeCallback != nil {
		// Convert to relative paths for display
		sourceDisplay := sourcePath
		if relPath, err := filepath.Rel(te.rootPath, sourcePath); err == nil {
			sourceDisplay = relPath
		}
		destDisplay := destPath
		if relPath, err := filepath.Rel(te.rootPath, destPath); err == nil {
			destDisplay = relPath
		}

		te.fileChangeCallback(FileChange{
			FilePath:  fmt.Sprintf("%s → %s", sourceDisplay, destDisplay),
			Before:    beforeContent,
			After:     beforeContent, // Content unchanged, just moved
			Operation: "move",
		})
	}

	return fmt.Sprintf("File moved from %s to %s", sourcePath, destPath), nil
}

// copyFile copies a file to a new location
func (te *ToolExecutor) copyFile(input map[string]interface{}) (string, error) {
	sourcePath, ok := input["source_path"].(string)
	if !ok {
		return "", fmt.Errorf("source_path is required")
	}

	destPath, ok := input["dest_path"].(string)
	if !ok {
		return "", fmt.Errorf("dest_path is required")
	}

	// Resolve relative paths
	if !filepath.IsAbs(sourcePath) {
		sourcePath = filepath.Join(te.rootPath, sourcePath)
	}
	if !filepath.IsAbs(destPath) {
		destPath = filepath.Join(te.rootPath, destPath)
	}

	// Check if source exists
	sourceInfo, err := os.Stat(sourcePath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("source file %s does not exist", sourcePath)
	}
	if sourceInfo.IsDir() {
		return "", fmt.Errorf("source %s is a directory, use copy_dir for directories", sourcePath)
	}

	// Read source content
	sourceContent, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to read source file %s: %w", sourcePath, err)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	// Check if destination already exists
	if _, err := os.Stat(destPath); err == nil {
		return "", fmt.Errorf("destination file %s already exists", destPath)
	}

	// Copy the file
	err = os.WriteFile(destPath, sourceContent, sourceInfo.Mode())
	if err != nil {
		return "", fmt.Errorf("failed to copy file to %s: %w", destPath, err)
	}

	// Call file change callback for diff display
	if te.fileChangeCallback != nil {
		// Convert to relative path for display
		destDisplay := destPath
		if relPath, err := filepath.Rel(te.rootPath, destPath); err == nil {
			destDisplay = relPath
		}

		te.fileChangeCallback(FileChange{
			FilePath:  destDisplay,
			Before:    "", // New file
			After:     string(sourceContent),
			Operation: "copy",
		})
	}

	return fmt.Sprintf("File copied from %s to %s (%d bytes)", sourcePath, destPath, len(sourceContent)), nil
}

// deleteFile deletes a file
func (te *ToolExecutor) deleteFile(input map[string]interface{}) (string, error) {
	filePath, ok := input["file_path"].(string)
	if !ok {
		return "", fmt.Errorf("file_path is required")
	}

	// Resolve relative path
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(te.rootPath, filePath)
	}

	// Check if file exists and get info
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("file %s does not exist", filePath)
	}
	if fileInfo.IsDir() {
		return "", fmt.Errorf("path %s is a directory, use delete_dir for directories", filePath)
	}

	// Read content before deletion for diff tracking
	var beforeContent string
	if content, err := os.ReadFile(filePath); err == nil {
		beforeContent = string(content)
	}

	// Delete the file
	err = os.Remove(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to delete file %s: %w", filePath, err)
	}

	// Call file change callback for diff display
	if te.fileChangeCallback != nil {
		// Convert to relative path for display
		displayPath := filePath
		if relPath, err := filepath.Rel(te.rootPath, filePath); err == nil {
			displayPath = relPath
		}

		te.fileChangeCallback(FileChange{
			FilePath:  displayPath,
			Before:    beforeContent,
			After:     "", // File deleted
			Operation: "delete",
		})
	}

	return fmt.Sprintf("File %s deleted successfully", filePath), nil
}

// createDir creates a directory
func (te *ToolExecutor) createDir(input map[string]interface{}) (string, error) {
	dirPath, ok := input["dir_path"].(string)
	if !ok {
		return "", fmt.Errorf("dir_path is required")
	}

	// Resolve relative path
	if !filepath.IsAbs(dirPath) {
		dirPath = filepath.Join(te.rootPath, dirPath)
	}

	// Check if directory already exists
	if _, err := os.Stat(dirPath); err == nil {
		return "", fmt.Errorf("directory %s already exists", dirPath)
	}

	// Create directory with parents
	err := os.MkdirAll(dirPath, 0o755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	return fmt.Sprintf("Directory %s created successfully", dirPath), nil
}

// deleteDir deletes a directory
func (te *ToolExecutor) deleteDir(input map[string]interface{}) (string, error) {
	dirPath, ok := input["dir_path"].(string)
	if !ok {
		return "", fmt.Errorf("dir_path is required")
	}

	// Resolve relative path
	if !filepath.IsAbs(dirPath) {
		dirPath = filepath.Join(te.rootPath, dirPath)
	}

	// Check if directory exists
	dirInfo, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("directory %s does not exist", dirPath)
	}
	if !dirInfo.IsDir() {
		return "", fmt.Errorf("path %s is not a directory", dirPath)
	}

	// Safety check - prevent deletion of root path or parent directories
	rootAbs, _ := filepath.Abs(te.rootPath)
	dirAbs, _ := filepath.Abs(dirPath)
	if dirAbs == rootAbs || strings.HasPrefix(rootAbs, dirAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("cannot delete root directory or parent directories")
	}

	// Check if directory is empty (optional safety)
	recursive := false
	if rec, ok := input["recursive"].(bool); ok {
		recursive = rec
	}

	if !recursive {
		// Check if directory is empty
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			return "", fmt.Errorf("failed to read directory %s: %w", dirPath, err)
		}
		if len(entries) > 0 {
			return "", fmt.Errorf("directory %s is not empty, use recursive=true to force deletion", dirPath)
		}
	}

	// Delete the directory
	if recursive {
		err = os.RemoveAll(dirPath)
	} else {
		err = os.Remove(dirPath)
	}

	if err != nil {
		return "", fmt.Errorf("failed to delete directory %s: %w", dirPath, err)
	}

	return fmt.Sprintf("Directory %s deleted successfully", dirPath), nil
}

// listFiles lists files in a directory
func (te *ToolExecutor) listFiles(input map[string]interface{}) (string, error) {
	directory := te.rootPath
	if dir, ok := input["directory"].(string); ok && dir != "" {
		directory = dir
		if !filepath.IsAbs(directory) {
			directory = filepath.Join(te.rootPath, directory)
		}
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", fmt.Errorf("failed to list directory %s: %w", directory, err)
	}

	var files []string
	var dirs []string

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			dirs = append(dirs, name+"/")
		} else {
			files = append(files, name)
		}
	}

	var result []string
	result = append(result, fmt.Sprintf("Directory: %s", directory))
	result = append(result, "")

	if len(dirs) > 0 {
		result = append(result, "Directories:")
		for _, dir := range dirs {
			result = append(result, "  "+dir)
		}
		result = append(result, "")
	}

	if len(files) > 0 {
		result = append(result, "Files:")
		for _, file := range files {
			result = append(result, "  "+file)
		}
	}

	if len(dirs) == 0 && len(files) == 0 {
		result = append(result, "Directory is empty")
	}

	return strings.Join(result, "\n"), nil
}
