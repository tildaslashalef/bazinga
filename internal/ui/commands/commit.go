package commands

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// CommitCommand handles the /commit command
type CommitCommand struct{}

func (c *CommitCommand) Execute(ctx context.Context, args []string, model CommandModel) tea.Msg {
	session := model.GetSession()
	if session == nil {
		return ResponseMsg{Content: c.formatError("No active session")}
	}

	// Check repository status before attempting commit
	diffOutput, err := session.GetDiffOutput()
	if err != nil {
		return ResponseMsg{Content: c.formatError("Failed to check repository status: " + err.Error())}
	}

	if strings.Contains(diffOutput, "Working tree clean") {
		return ResponseMsg{Content: c.formatInfo("Working tree clean - nothing to commit")}
	}

	// Show what will be committed
	statusMsg := c.formatStatusPreview(diffOutput)

	if len(args) > 0 {
		// Manual commit message
		message := strings.Join(args, " ")
		err := session.CommitChanges(ctx, message)
		if err != nil {
			return ResponseMsg{Content: c.formatError("Error committing changes: " + err.Error())}
		}

		return ResponseMsg{Content: statusMsg + "\n" + c.formatSuccess("Changes committed: "+message)}

	} else {
		// AI-generated commit message
		response := statusMsg + "\n" + c.formatInfo("Generating AI commit message...")

		// Start async commit process
		go func() {
			result, err := session.CommitWithAI(ctx)
			if err != nil {
				model.AddMessage("system", c.formatError("AI commit failed: "+err.Error()), false)
			} else {
				model.AddMessage("system", c.formatSuccess(result), false)
			}
		}()

		return ResponseMsg{Content: response}
	}
}

func (c *CommitCommand) GetName() string {
	return "commit"
}

func (c *CommitCommand) GetUsage() string {
	return "/commit [message]"
}

func (c *CommitCommand) GetDescription() string {
	return "Commit changes with optional message (AI-generated if none provided)"
}

// formatStatusPreview creates a preview of changes to be committed
func (c *CommitCommand) formatStatusPreview(diffOutput string) string {
	if strings.Contains(diffOutput, "Working tree clean") {
		return ""
	}

	var preview strings.Builder
	preview.WriteString("📋 Changes to be committed:\n")

	// Count different types of changes from the diff output
	lines := strings.Split(diffOutput, "\n")
	var addedCount, modifiedCount, deletedCount, untrackedCount int

	for _, line := range lines {
		if strings.HasPrefix(line, "  + ") {
			addedCount++
		} else if strings.HasPrefix(line, "  M ") {
			modifiedCount++
		} else if strings.HasPrefix(line, "  D ") {
			deletedCount++
		} else if strings.HasPrefix(line, "  ? ") {
			untrackedCount++
		}
	}

	if addedCount > 0 {
		preview.WriteString(fmt.Sprintf("  + %d new file(s)\n", addedCount))
	}
	if modifiedCount > 0 {
		preview.WriteString(fmt.Sprintf("  ~ %d modified file(s)\n", modifiedCount))
	}
	if deletedCount > 0 {
		preview.WriteString(fmt.Sprintf("  - %d deleted file(s)\n", deletedCount))
	}
	if untrackedCount > 0 {
		preview.WriteString(fmt.Sprintf("  ? %d untracked file(s)\n", untrackedCount))
	}

	return preview.String()
}

// Formatting functions
func (c *CommitCommand) formatSuccess(content string) string {
	return fmt.Sprintf("✓ %s", content)
}

func (c *CommitCommand) formatError(content string) string {
	return fmt.Sprintf("✗ %s", content)
}

func (c *CommitCommand) formatInfo(content string) string {
	return fmt.Sprintf("ℹ %s", content)
}
