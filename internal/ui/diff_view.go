package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// FileDiff represents a file change with before/after content
type FileDiff struct {
	FilePath     string
	Before       string
	After        string
	Operation    string // "edit", "create", "write"
	LinesAdded   int
	LinesRemoved int
}

// DiffLine represents a single line in a diff
type DiffLine struct {
	Type    string // "unchanged", "added", "removed"
	Content string
	LineNum int
}

// GenerateDiff creates a unified diff view between before and after content
func GenerateDiff(filePath, before, after, operation string) *FileDiff {
	diff := &FileDiff{
		FilePath:  filePath,
		Before:    before,
		After:     after,
		Operation: operation,
	}

	// Calculate line changes
	beforeLines := strings.Split(before, "\n")
	afterLines := strings.Split(after, "\n")

	// Simple diff calculation
	diff.LinesAdded = len(afterLines) - len(beforeLines)
	if diff.LinesAdded < 0 {
		diff.LinesRemoved = -diff.LinesAdded
		diff.LinesAdded = 0
	}

	return diff
}

// RenderDiff renders a diff
func (d *FileDiff) RenderDiff() string {
	if d.Before == d.After {
		return "" // No changes
	}

	var result []string

	headerStyle := lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true)

	var headerText string
	switch d.Operation {
	case "create":
		headerText = fmt.Sprintf("📄 Created %s", d.FilePath)
	case "edit":
		headerText = fmt.Sprintf("✏️ Modified %s", d.FilePath)
	case "write":
		headerText = fmt.Sprintf("💾 Updated %s", d.FilePath)
	case "move":
		headerText = fmt.Sprintf("📁 Moved %s", d.FilePath)
	case "copy":
		headerText = fmt.Sprintf("📋 Copied %s", d.FilePath)
	case "delete":
		headerText = fmt.Sprintf("🗑️ Deleted %s", d.FilePath)
	default:
		headerText = fmt.Sprintf("📄 Changed %s", d.FilePath)
	}

	result = append(result, headerStyle.Render(headerText))

	if d.LinesAdded > 0 || d.LinesRemoved > 0 {
		statsStyle := lipgloss.NewStyle().Foreground(TextSecondary)
		statsText := ""
		if d.LinesAdded > 0 && d.LinesRemoved > 0 {
			statsText = fmt.Sprintf("(+%d -%d lines)", d.LinesAdded, d.LinesRemoved)
		} else if d.LinesAdded > 0 {
			statsText = fmt.Sprintf("(+%d lines)", d.LinesAdded)
		} else if d.LinesRemoved > 0 {
			statsText = fmt.Sprintf("(-%d lines)", d.LinesRemoved)
		}
		result = append(result, statsStyle.Render(statsText))
	}

	result = append(result, "")

	// Generate and render diff lines
	diffLines := d.generateDiffLines()
	renderedLines := d.renderDiffLines(diffLines)
	result = append(result, renderedLines...)

	return strings.Join(result, "\n")
}

// generateDiffLines creates a simple unified diff
func (d *FileDiff) generateDiffLines() []DiffLine {
	beforeLines := strings.Split(d.Before, "\n")
	afterLines := strings.Split(d.After, "\n")

	var diffLines []DiffLine

	// Simple implementation: show removed lines then added lines
	// This is not a true unified diff but works for basic visualization

	// Handle file creation
	if d.Operation == "create" {
		for i, line := range afterLines {
			diffLines = append(diffLines, DiffLine{
				Type:    "added",
				Content: line,
				LineNum: i + 1,
			})
		}
		return diffLines
	}

	// Handle file editing/writing
	maxLines := len(beforeLines)
	if len(afterLines) > maxLines {
		maxLines = len(afterLines)
	}

	for i := 0; i < maxLines; i++ {
		var beforeLine, afterLine string

		if i < len(beforeLines) {
			beforeLine = beforeLines[i]
		}
		if i < len(afterLines) {
			afterLine = afterLines[i]
		}

		if beforeLine == afterLine {
			// Unchanged line
			diffLines = append(diffLines, DiffLine{
				Type:    "unchanged",
				Content: beforeLine,
				LineNum: i + 1,
			})
		} else {
			// Line changed
			if beforeLine != "" {
				diffLines = append(diffLines, DiffLine{
					Type:    "removed",
					Content: beforeLine,
					LineNum: i + 1,
				})
			}
			if afterLine != "" {
				diffLines = append(diffLines, DiffLine{
					Type:    "added",
					Content: afterLine,
					LineNum: i + 1,
				})
			}
		}
	}

	return diffLines
}

// renderDiffLines renders diff lines with appropriate styling
func (d *FileDiff) renderDiffLines(diffLines []DiffLine) []string {
	var result []string
	contextLines := 3 // Show 3 lines of context around changes

	// Find changed line indices
	var changedIndices []int
	for i, line := range diffLines {
		if line.Type == "added" || line.Type == "removed" {
			changedIndices = append(changedIndices, i)
		}
	}

	if len(changedIndices) == 0 {
		return result // No changes
	}

	// Determine which lines to show (with context)
	showLines := make(map[int]bool)
	for _, idx := range changedIndices {
		// Add context around each changed line
		start := idx - contextLines
		if start < 0 {
			start = 0
		}
		end := idx + contextLines + 1
		if end > len(diffLines) {
			end = len(diffLines)
		}

		for i := start; i < end; i++ {
			showLines[i] = true
		}
	}

	// Render the lines
	lastShown := -2
	for i, line := range diffLines {
		if !showLines[i] {
			continue
		}

		// Add separator if there's a gap
		if i > lastShown+1 && lastShown >= 0 {
			result = append(result, lipgloss.NewStyle().
				Foreground(TextMuted).
				Render("..."))
		}

		// Style based on line type
		var lineStyle lipgloss.Style
		var prefix string

		switch line.Type {
		case "added":
			lineStyle = lipgloss.NewStyle().
				Foreground(SuccessColor)
			prefix = "+"
		case "removed":
			lineStyle = lipgloss.NewStyle().
				Foreground(ErrorColor)
			prefix = "-"
		case "unchanged":
			lineStyle = lipgloss.NewStyle().
				Foreground(TextSecondary)
			prefix = " "
		}

		// Line number (optional)
		lineNumStyle := lipgloss.NewStyle().
			Foreground(TextMuted).
			Width(4).
			Align(lipgloss.Right)

		lineNum := lineNumStyle.Render(fmt.Sprintf("%d", line.LineNum))
		content := lineStyle.Render(fmt.Sprintf("%s %s", prefix, line.Content))

		result = append(result, lineNum+" "+content)
		lastShown = i
	}

	return result
}

// RenderCompactDiff renders a compact single-line diff summary
func (d *FileDiff) RenderCompactDiff() string {
	if d.Before == d.After {
		return ""
	}

	var icon string
	switch d.Operation {
	case "create":
		icon = "📄"
	case "edit":
		icon = "✏️"
	case "write":
		icon = "💾"
	case "move":
		icon = "📁"
	case "copy":
		icon = "📋"
	case "delete":
		icon = "🗑️"
	default:
		icon = "📄"
	}

	fileStyle := lipgloss.NewStyle().Foreground(AccentColor)
	statsStyle := lipgloss.NewStyle().Foreground(TextSecondary)

	var statsText string
	if d.LinesAdded > 0 && d.LinesRemoved > 0 {
		statsText = fmt.Sprintf(" (+%d -%d)", d.LinesAdded, d.LinesRemoved)
	} else if d.LinesAdded > 0 {
		statsText = fmt.Sprintf(" (+%d)", d.LinesAdded)
	} else if d.LinesRemoved > 0 {
		statsText = fmt.Sprintf(" (-%d)", d.LinesRemoved)
	}

	return fmt.Sprintf("%s %s%s",
		icon,
		fileStyle.Render(d.FilePath),
		statsStyle.Render(statsText))
}
