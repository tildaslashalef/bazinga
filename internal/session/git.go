package session

import (
	"context"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	gitstatus "github.com/tildaslashalef/bazinga/internal/git"
)

// ShowDiff shows the git diff for current changes
func (s *Session) ShowDiff(ctx context.Context) error {
	diffOutput, err := s.GetDiffOutput()
	if err != nil {
		return fmt.Errorf("failed to get diff: %w", err)
	}

	fmt.Print(diffOutput)
	return nil
}

// GetDiffOutput returns formatted diff output for display
func (s *Session) GetDiffOutput() (string, error) {
	return gitstatus.GetDiffOutput(s.gitRepo)
}

// CommitChanges commits the current changes
func (s *Session) CommitChanges(ctx context.Context, message string) error {
	if s.DryRun {
		loggy.Info("DRY RUN: Would commit with message", "message", message)
		return nil
	}

	if s.gitRepo == nil {
		return fmt.Errorf("no git repository found")
	}

	// Get working tree
	worktree, err := s.gitRepo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Add all changes
	if err := worktree.AddGlob("*"); err != nil {
		return fmt.Errorf("failed to add changes: %w", err)
	}

	// Create commit
	commit, err := worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  s.getGitAuthorName(),
			Email: s.getGitAuthorEmail(),
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	loggy.Info("Committed changes", "commit", commit.String())
	return nil
}

// CommitWithAI commits changes with an AI-generated commit message
func (s *Session) CommitWithAI(ctx context.Context) (string, error) {
	if s.gitRepo == nil {
		return "", fmt.Errorf("no git repository found")
	}

	// Get current provider for AI commit message generation
	provider, err := s.llmManager.GetProvider(s.Provider)
	if err != nil {
		return "", fmt.Errorf("failed to get provider for AI commit: %w", err)
	}

	// Create commit generator
	commitGen := gitstatus.NewCommitGenerator(provider)

	// Generate and commit with AI message
	authorName := s.getGitAuthorName()
	authorEmail := s.getGitAuthorEmail()

	result, err := commitGen.CommitWithAI(ctx, s.gitRepo, authorName, authorEmail)
	if err != nil {
		return "", fmt.Errorf("AI commit failed: %w", err)
	}

	return result, nil
}

// GetBranchInfo returns current git branch information
func (s *Session) GetBranchInfo() (string, error) {
	if s.gitRepo == nil {
		return "No git repository", nil
	}

	branchInfo, err := gitstatus.GetBranchInfo(s.gitRepo)
	if err != nil {
		return "", fmt.Errorf("failed to get branch info: %w", err)
	}

	return fmt.Sprintf("Current branch: %s (%s)\nAll branches: %s",
		branchInfo.Current, branchInfo.CommitHash, strings.Join(branchInfo.All, ", ")), nil
}

// GetCommitHistory returns recent commit history
func (s *Session) GetCommitHistory(limit int) (string, error) {
	if s.gitRepo == nil {
		return "No git repository", nil
	}

	commits, err := gitstatus.GetCommitHistory(s.gitRepo, limit)
	if err != nil {
		return "", fmt.Errorf("failed to get commit history: %w", err)
	}

	if len(commits) == 0 {
		return "No commits found", nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Recent %d commits:\n\n", len(commits)))

	for _, commit := range commits {
		result.WriteString(fmt.Sprintf("%s - %s\n", commit.Hash, strings.TrimSpace(commit.Message)))
		result.WriteString(fmt.Sprintf("        %s - %s\n\n", commit.Author, commit.Date.Format("2006-01-02 15:04")))
	}

	return result.String(), nil
}

// getGitAuthorName returns the git author name, falling back to system git config
func (s *Session) getGitAuthorName() string {
	if s.config != nil && s.config.Git.AuthorName != "" {
		return s.config.Git.AuthorName
	}

	// Try to get from git config
	if name := getGitConfigValue("user.name"); name != "" {
		return name
	}

	// Default fallback
	return "bazinga"
}

// getGitAuthorEmail returns the git author email, falling back to system git config
func (s *Session) getGitAuthorEmail() string {
	if s.config != nil && s.config.Git.AuthorEmail != "" {
		return s.config.Git.AuthorEmail
	}

	// Try to get from git config
	if email := getGitConfigValue("user.email"); email != "" {
		return email
	}

	// Default fallback
	return "bazinga@ai-assistant.com"
}

// getGitConfigValue gets a value from global git config
func getGitConfigValue(key string) string {
	// This would require executing `git config --global user.name` etc.
	// For now, return empty to use defaults
	return ""
}
