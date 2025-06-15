package git

import (
	"fmt"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

// FileStatus represents the git status of a file
type FileStatus int

const (
	StatusUnknown FileStatus = iota
	StatusUntracked
	StatusModified
	StatusAdded
	StatusDeleted
	StatusRenamed
	StatusClean
)

// String returns a human-readable representation of the file status
func (fs FileStatus) String() string {
	switch fs {
	case StatusUntracked:
		return "untracked"
	case StatusModified:
		return "modified"
	case StatusAdded:
		return "added"
	case StatusDeleted:
		return "deleted"
	case StatusRenamed:
		return "renamed"
	case StatusClean:
		return "clean"
	default:
		return "unknown"
	}
}

// GetFileStatus returns the git status of a specific file
func GetFileStatus(repo *git.Repository, filePath, rootPath string) (FileStatus, error) {
	if repo == nil {
		return StatusUnknown, nil
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return StatusUnknown, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return StatusUnknown, fmt.Errorf("failed to get status: %w", err)
	}

	// Convert absolute path to relative path from repo root
	relPath, err := filepath.Rel(rootPath, filePath)
	if err != nil {
		return StatusUnknown, fmt.Errorf("failed to get relative path: %w", err)
	}

	// Check git status for this file
	for file, stat := range status {
		if file == relPath {
			// Check staging area first, then working tree
			if stat.Staging != git.Unmodified {
				switch stat.Staging {
				case git.Added:
					return StatusAdded, nil
				case git.Modified:
					return StatusModified, nil
				case git.Deleted:
					return StatusDeleted, nil
				case git.Renamed:
					return StatusRenamed, nil
				}
			}

			// Check working tree
			if stat.Worktree != git.Unmodified {
				switch stat.Worktree {
				case git.Modified:
					return StatusModified, nil
				case git.Deleted:
					return StatusDeleted, nil
				case git.Untracked:
					return StatusUntracked, nil
				}
			}
		}
	}

	// File is clean (tracked and unchanged)
	return StatusClean, nil
}

// GetRepositoryStatus returns overall repository status
func GetRepositoryStatus(repo *git.Repository) (map[string]FileStatus, error) {
	if repo == nil {
		return make(map[string]FileStatus), nil
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	result := make(map[string]FileStatus)

	for file, stat := range status {
		// Prioritize staging area status
		if stat.Staging != git.Unmodified {
			switch stat.Staging {
			case git.Added:
				result[file] = StatusAdded
			case git.Modified:
				result[file] = StatusModified
			case git.Deleted:
				result[file] = StatusDeleted
			case git.Renamed:
				result[file] = StatusRenamed
			}
		} else if stat.Worktree != git.Unmodified {
			switch stat.Worktree {
			case git.Modified:
				result[file] = StatusModified
			case git.Deleted:
				result[file] = StatusDeleted
			case git.Untracked:
				result[file] = StatusUntracked
			}
		}
	}

	return result, nil
}

// GetDiffOutput returns formatted diff output (enhanced status for now)
func GetDiffOutput(repo *git.Repository) (string, error) {
	if repo == nil {
		return "No git repository found", nil
	}

	// For now, return enhanced status output
	// TODO: Implement proper diff when go-git API is more stable
	return GetStatusOutput(repo)
}

// GetStatusOutput returns git status without diff content (fallback)
func GetStatusOutput(repo *git.Repository) (string, error) {
	if repo == nil {
		return "No git repository found", nil
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}

	if status.IsClean() {
		return "Working tree clean - no changes to display", nil
	}

	output := "Git Status:\n\n"

	// Group files by status
	var modified, added, deleted, untracked []string

	for file, stat := range status {
		if stat.Staging != git.Unmodified {
			switch stat.Staging {
			case git.Added:
				added = append(added, file)
			case git.Modified:
				modified = append(modified, file)
			case git.Deleted:
				deleted = append(deleted, file)
			}
		} else if stat.Worktree != git.Unmodified {
			switch stat.Worktree {
			case git.Modified:
				modified = append(modified, file)
			case git.Deleted:
				deleted = append(deleted, file)
			case git.Untracked:
				untracked = append(untracked, file)
			}
		}
	}

	if len(added) > 0 {
		output += "Added files:\n"
		for _, file := range added {
			output += fmt.Sprintf("  + %s\n", file)
		}
		output += "\n"
	}

	if len(modified) > 0 {
		output += "Modified files:\n"
		for _, file := range modified {
			output += fmt.Sprintf("  M %s\n", file)
		}
		output += "\n"
	}

	if len(deleted) > 0 {
		output += "Deleted files:\n"
		for _, file := range deleted {
			output += fmt.Sprintf("  D %s\n", file)
		}
		output += "\n"
	}

	if len(untracked) > 0 {
		output += "Untracked files:\n"
		for _, file := range untracked {
			output += fmt.Sprintf("  ? %s\n", file)
		}
		output += "\n"
	}

	return output, nil
}
