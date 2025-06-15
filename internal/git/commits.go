package git

import (
	"context"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// CommitGenerator generates AI-powered commit messages
type CommitGenerator struct {
	provider    llm.Provider
	maxTokens   int
	temperature float64
}

// NewCommitGenerator creates a new commit message generator
func NewCommitGenerator(provider llm.Provider) *CommitGenerator {
	return &CommitGenerator{
		provider:    provider,
		maxTokens:   150, // Short commit messages
		temperature: 0.3, // More deterministic for consistency
	}
}

// GenerateCommitMessage creates an AI-generated commit message based on diff
func (cg *CommitGenerator) GenerateCommitMessage(ctx context.Context, repo *git.Repository) (string, error) {
	// Get diff content
	diffOutput, err := GetDiffOutput(repo)
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	if strings.Contains(diffOutput, "Working tree clean") {
		return "", fmt.Errorf("no changes to commit")
	}

	// Enhance diff analysis for better AI context
	repoStatus, err := GetRepositoryStatus(repo)
	if err != nil {
		return "", fmt.Errorf("failed to get repository status: %w", err)
	}

	if len(repoStatus) == 0 {
		return "", fmt.Errorf("no changes to commit")
	}

	// Create enhanced prompt for AI with better context
	changesSummary := cg.analyzeChanges(repoStatus)
	prompt := fmt.Sprintf(`Generate a concise git commit message for the following changes. Follow conventional commit format:

Rules:
- Use format: type(scope): description
- Types: feat, fix, docs, style, refactor, test, chore
- Keep under 50 characters for the title
- Be specific and clear about what changed
- Focus on WHAT changed, not HOW

Changes Summary:
%s

Detailed Status:
%s

Commit message:`, changesSummary, diffOutput)

	// Generate commit message using AI
	req := &llm.GenerateRequest{
		Messages: []llm.Message{
			{
				Role:    "system",
				Content: "You are a git commit message generator. Generate concise, clear commit messages following conventional commit format.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   cg.maxTokens,
		Temperature: cg.temperature,
	}

	response, err := cg.provider.GenerateResponse(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to generate commit message: %w", err)
	}

	// Clean up the response
	message := strings.TrimSpace(response.Content)
	message = strings.ReplaceAll(message, "\n", " ")

	// Remove any quotes if AI added them
	message = strings.Trim(message, "\"'`")

	return message, nil
}

// CommitWithAI commits changes with an AI-generated message
func (cg *CommitGenerator) CommitWithAI(ctx context.Context, repo *git.Repository, authorName, authorEmail string) (string, error) {
	// Generate commit message
	message, err := cg.GenerateCommitMessage(ctx, repo)
	if err != nil {
		return "", err
	}

	// Perform the commit
	commitHash, err := CommitChanges(repo, message, authorName, authorEmail)
	if err != nil {
		return "", fmt.Errorf("failed to commit: %w", err)
	}

	return fmt.Sprintf("Committed with message: %s\nCommit: %s", message, commitHash), nil
}

// CommitChanges commits all staged changes with the given message
func CommitChanges(repo *git.Repository, message, authorName, authorEmail string) (string, error) {
	if repo == nil {
		return "", fmt.Errorf("no git repository")
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	// Add all changes to staging
	status, err := worktree.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}

	if status.IsClean() {
		return "", fmt.Errorf("nothing to commit, working tree clean")
	}

	// Stage all changes
	for file := range status {
		_, err = worktree.Add(file)
		if err != nil {
			return "", fmt.Errorf("failed to add file %s: %w", file, err)
		}
	}

	// Create commit
	commit, err := worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  authorName,
			Email: authorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create commit: %w", err)
	}

	return commit.String(), nil
}

// GetCommitHistory returns recent commit history
func GetCommitHistory(repo *git.Repository, limit int) ([]CommitInfo, error) {
	if repo == nil {
		return nil, fmt.Errorf("no git repository")
	}

	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Get commit iterator
	commitIter, err := repo.Log(&git.LogOptions{
		From: ref.Hash(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}
	defer commitIter.Close()

	var commits []CommitInfo
	count := 0

	err = commitIter.ForEach(func(c *object.Commit) error {
		if count >= limit {
			return fmt.Errorf("limit reached") // Stop iteration
		}

		commits = append(commits, CommitInfo{
			Hash:    c.Hash.String()[:8],
			Message: c.Message,
			Author:  c.Author.Name,
			Date:    c.Author.When,
		})
		count++
		return nil
	})

	if err != nil && err.Error() != "limit reached" {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return commits, nil
}

// CommitInfo represents commit information
type CommitInfo struct {
	Hash    string    `json:"hash"`
	Message string    `json:"message"`
	Author  string    `json:"author"`
	Date    time.Time `json:"date"`
}

// GetBranchInfo returns current branch information
func GetBranchInfo(repo *git.Repository) (*BranchInfo, error) {
	if repo == nil {
		return nil, fmt.Errorf("no git repository")
	}

	// Get current branch
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	branchName := "HEAD"
	if ref.Name().IsBranch() {
		branchName = ref.Name().Short()
	}

	// Simplified - just return current branch info
	// TODO: Add full branch listing when needed
	return &BranchInfo{
		Current:    branchName,
		All:        []string{branchName}, // Simplified for now
		CommitHash: ref.Hash().String()[:8],
	}, nil
}

// BranchInfo represents branch information
type BranchInfo struct {
	Current    string   `json:"current"`
	All        []string `json:"all"`
	CommitHash string   `json:"commit_hash"`
}

// analyzeChanges creates a summary of changes for better AI context
func (cg *CommitGenerator) analyzeChanges(repoStatus map[string]FileStatus) string {
	var added, modified, deleted, untracked []string

	for file, status := range repoStatus {
		switch status {
		case StatusAdded:
			added = append(added, file)
		case StatusModified:
			modified = append(modified, file)
		case StatusDeleted:
			deleted = append(deleted, file)
		case StatusUntracked:
			untracked = append(untracked, file)
		}
	}

	var summary strings.Builder

	if len(added) > 0 {
		summary.WriteString(fmt.Sprintf("Added %d file(s): %s\n", len(added), strings.Join(added[:min(len(added), 3)], ", ")))
		if len(added) > 3 {
			summary.WriteString(fmt.Sprintf("... and %d more\n", len(added)-3))
		}
	}

	if len(modified) > 0 {
		summary.WriteString(fmt.Sprintf("Modified %d file(s): %s\n", len(modified), strings.Join(modified[:min(len(modified), 3)], ", ")))
		if len(modified) > 3 {
			summary.WriteString(fmt.Sprintf("... and %d more\n", len(modified)-3))
		}
	}

	if len(deleted) > 0 {
		summary.WriteString(fmt.Sprintf("Deleted %d file(s): %s\n", len(deleted), strings.Join(deleted[:min(len(deleted), 3)], ", ")))
		if len(deleted) > 3 {
			summary.WriteString(fmt.Sprintf("... and %d more\n", len(deleted)-3))
		}
	}

	if len(untracked) > 0 {
		summary.WriteString(fmt.Sprintf("Untracked %d file(s): %s\n", len(untracked), strings.Join(untracked[:min(len(untracked), 3)], ", ")))
		if len(untracked) > 3 {
			summary.WriteString(fmt.Sprintf("... and %d more\n", len(untracked)-3))
		}
	}

	return summary.String()
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TODO: Branch creation and switching functionality
// Will be implemented in future versions when go-git API stabilizes
