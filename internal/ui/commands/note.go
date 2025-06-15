package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// NoteCommand handles the # command for quickly adding notes to Bazinga.md
type NoteCommand struct{}

func (c *NoteCommand) Execute(ctx context.Context, args []string, model CommandModel) tea.Msg {
	session := model.GetSession()

	if len(args) == 0 {
		return ResponseMsg{Content: "Usage: # <note>\nExample: # Fixed the tool execution display issue"}
	}

	// Join all args to form the note
	note := strings.Join(args, " ")

	// Get the path to Bazinga.md
	bazingaMdPath := filepath.Join(session.GetRootPath(), "Bazinga.md")

	// Check if Bazinga.md exists
	if _, err := os.Stat(bazingaMdPath); os.IsNotExist(err) {
		return ResponseMsg{
			Content: "Bazinga.md doesn't exist. Run `/init` first to create it, or would you like me to create a basic one now?",
		}
	}

	// Read existing content
	existingContent, err := os.ReadFile(bazingaMdPath)
	if err != nil {
		return ResponseMsg{Content: fmt.Sprintf("❌ Failed to read Bazinga.md: %v", err)}
	}

	// Format the new note with timestamp
	timestamp := time.Now().Format("2006-01-02 15:04")
	formattedNote := fmt.Sprintf("- **%s**: %s", timestamp, note)

	// Find where to insert the note
	content := string(existingContent)
	var newContent string

	// Look for existing "## Quick Notes" section
	if strings.Contains(content, "## Quick Notes") {
		// Add to existing section
		lines := strings.Split(content, "\n")
		var result []string
		inQuickNotes := false
		inserted := false

		for _, line := range lines {
			result = append(result, line)

			if strings.HasPrefix(line, "## Quick Notes") {
				inQuickNotes = true
				continue
			}

			// If we're in Quick Notes section and hit another ## heading, insert before it
			if inQuickNotes && strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "## Quick Notes") {
				if !inserted {
					result = append(result[:len(result)-1], "", formattedNote, "", line)
					inserted = true
				}
				inQuickNotes = false
				continue
			}
		}

		// If we didn't insert yet and we were in Quick Notes, add at the end
		if inQuickNotes && !inserted {
			result = append(result, "", formattedNote)
		}

		newContent = strings.Join(result, "\n")
	} else {
		// Add new Quick Notes section
		if strings.HasSuffix(content, "\n") {
			newContent = content + "\n## Quick Notes\n\n" + formattedNote + "\n"
		} else {
			newContent = content + "\n\n## Quick Notes\n\n" + formattedNote + "\n"
		}
	}

	// Write back to file
	if err := os.WriteFile(bazingaMdPath, []byte(newContent), 0o644); err != nil {
		return ResponseMsg{Content: fmt.Sprintf("❌ Failed to write to Bazinga.md: %v", err)}
	}

	return ResponseMsg{Content: fmt.Sprintf("✅ Added note to Bazinga.md: %s", note)}
}

func (c *NoteCommand) GetName() string {
	return "#"
}

func (c *NoteCommand) GetUsage() string {
	return "# <note>"
}

func (c *NoteCommand) GetDescription() string {
	return "Quickly add a timestamped note to Bazinga.md"
}
