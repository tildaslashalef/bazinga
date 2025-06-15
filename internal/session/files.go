package session

import (
	"context"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"github.com/tildaslashalef/bazinga/internal/project"
	"os"
	"path/filepath"
	"slices"
	"time"

	gitstatus "github.com/tildaslashalef/bazinga/internal/git"
)

// AddFile adds a file to the session
func (s *Session) AddFile(ctx context.Context, filePath string) error {
	// Resolve absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", absPath)
	}

	// Check if already added
	if slices.Contains(s.Files, absPath) {
		return fmt.Errorf("file already in session: %s", absPath)
	}

	s.Files = append(s.Files, absPath)
	s.UpdatedAt = time.Now()

	// Auto-save session after adding file
	if err := s.Save(); err != nil {
		loggy.Warn("Failed to auto-save session after adding file", "session_id", s.ID, "file", absPath, "error", err)
	}

	// Add file to watcher if available
	if s.fileWatcher != nil {
		if err := s.fileWatcher.AddFile(absPath); err != nil {
			loggy.Warn("Could not watch file", "file", filePath, "error", err)
		}
	}

	return nil
}

// RemoveFile removes a file from the session
func (s *Session) RemoveFile(ctx context.Context, filePath string) error {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Find and remove from Files slice
	for i, existing := range s.Files {
		if existing == absPath {
			s.Files = append(s.Files[:i], s.Files[i+1:]...)
			s.UpdatedAt = time.Now()

			// Remove from watcher if available
			if s.fileWatcher != nil {
				if err := s.fileWatcher.RemoveFile(absPath); err != nil {
					loggy.Warn("Could not unwatch file", "file", filePath, "error", err)
				}
			}

			loggy.Debug("Removed file", "file", filePath)
			return nil
		}
	}

	return fmt.Errorf("file not in session: %s", absPath)
}

// ScanForMoreFiles scans the project for additional relevant files
func (s *Session) ScanForMoreFiles(ctx context.Context) error {
	if s.project == nil {
		return fmt.Errorf("no project detected")
	}

	// Re-scan the project to pick up any new files
	detector := project.NewDetector()
	updatedProject, err := detector.DetectProject(s.RootPath)
	if err != nil {
		return fmt.Errorf("failed to rescan project: %w", err)
	}

	// Add any new files that aren't already in the session
	newFilesAdded := 0
	for _, file := range updatedProject.Files {
		fullPath := filepath.Join(s.RootPath, file)

		// Check if already in session
		alreadyAdded := false
		for _, existing := range s.Files {
			if existing == fullPath {
				alreadyAdded = true
				break
			}
		}

		if !alreadyAdded {
			if err := s.AddFile(ctx, fullPath); err == nil {
				newFilesAdded++
			}
		}
	}

	s.project = updatedProject
	s.promptBuilder = project.NewPromptBuilder(updatedProject)

	loggy.Info("Scanned project and added new files", "count", newFilesAdded)
	return nil
}

// GetFileStatus returns the git status of a specific file
func (s *Session) GetFileStatus(filePath string) gitstatus.FileStatus {
	if s.gitRepo == nil {
		return gitstatus.StatusUnknown
	}

	status, err := gitstatus.GetFileStatus(s.gitRepo, filePath, s.RootPath)
	if err != nil {
		return gitstatus.StatusUnknown
	}

	return status
}

// GetAllFileStatuses returns git status for all files in the session
func (s *Session) GetAllFileStatuses() map[string]gitstatus.FileStatus {
	result := make(map[string]gitstatus.FileStatus)

	for _, filePath := range s.Files {
		result[filePath] = s.GetFileStatus(filePath)
	}

	return result
}
