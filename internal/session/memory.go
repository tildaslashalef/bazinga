package session

import (
	"context"
	"fmt"
)

// GetMemoryFilePaths returns the paths to memory files
func (s *Session) GetMemoryFilePaths() (userPath, projectPath string) {
	if s.memorySystem == nil {
		return "", ""
	}
	return s.memorySystem.GetMemoryFilePaths(s.RootPath)
}

// CreateMemoryFile creates a new MEMORY.md file
func (s *Session) CreateMemoryFile(ctx context.Context, isUserMemory bool) error {
	if s.memorySystem == nil {
		return fmt.Errorf("memory system not available")
	}

	userPath, projectPath := s.memorySystem.GetMemoryFilePaths(s.RootPath)
	var targetPath string

	if isUserMemory {
		targetPath = userPath
	} else {
		targetPath = projectPath
	}

	if err := s.memorySystem.CreateMemoryFile(ctx, targetPath, isUserMemory); err != nil {
		return err
	}

	// Reload memory content
	if memContent, err := s.memorySystem.LoadMemory(ctx, s.RootPath); err == nil {
		s.memoryContent = memContent
	}

	return nil
}

// AddQuickMemory adds a quick note to memory
func (s *Session) AddQuickMemory(ctx context.Context, note string, isUserMemory bool) error {
	if s.memorySystem == nil {
		return fmt.Errorf("memory system not available")
	}

	if err := s.memorySystem.AddQuickMemory(ctx, s.RootPath, note, isUserMemory); err != nil {
		return err
	}

	// Reload memory content
	if memContent, err := s.memorySystem.LoadMemory(ctx, s.RootPath); err == nil {
		s.memoryContent = memContent
	}

	return nil
}

// ReloadMemory reloads memory content from files
func (s *Session) ReloadMemory(ctx context.Context) error {
	if s.memorySystem == nil {
		return fmt.Errorf("memory system not available")
	}

	memContent, err := s.memorySystem.LoadMemory(ctx, s.RootPath)
	if err != nil {
		return err
	}

	s.memoryContent = memContent
	return nil
}
