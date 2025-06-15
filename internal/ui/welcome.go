package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// createWelcomeMessage creates the initial welcome message displayed to users
func (m *Model) createWelcomeMessage() string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "unknown"
	}

	var parts []string
	parts = append(parts, "🧙 Welcome to Bazinga!")
	parts = append(parts, "")
	parts = append(parts, "💡 Quick Start:")
	parts = append(parts, "  • Run /init to analyze your project")
	parts = append(parts, "  • Ask questions about your code")
	parts = append(parts, "  • Use /help to see all commands")
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("📁 Working directory: %s", cwd))

	content := strings.Join(parts, "\n")
	welcomeBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(AccentColor).
		Padding(1, 2).
		MarginBottom(1).
		Width(60).
		Render(content)

	var additionalInfo []string
	fullWelcome := welcomeBox + "\n\n" + strings.Join(additionalInfo, "\n")

	return fullWelcome
}
