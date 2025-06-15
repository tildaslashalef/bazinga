package commands

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// HelpCommand handles the /help command
type HelpCommand struct{}

func (c *HelpCommand) Execute(ctx context.Context, args []string, model CommandModel) tea.Msg {
	response := c.formatHelp()
	return ResponseMsg{Content: response}
}

func (c *HelpCommand) GetName() string {
	return "help"
}

func (c *HelpCommand) GetUsage() string {
	return "/help"
}

func (c *HelpCommand) GetDescription() string {
	return "Show help and available commands"
}

func (c *HelpCommand) formatHelp() string {
	var result strings.Builder

	result.WriteString("ℹ Available Commands:\n\n")

	// Project Setup
	result.WriteString("📁 Project Setup:\n")
	result.WriteString("  • /init            Analyze project and create Bazinga.md\n")
	result.WriteString("\n")

	// Git Operations
	result.WriteString("🌿 Git Operations:\n")
	result.WriteString("  • /commit [msg]    Commit changes (AI message if none provided)\n")
	result.WriteString("\n")

	// Memory Management
	result.WriteString("🧠 Memory Management:\n")
	result.WriteString("  • /memory          View/manage memory\n")
	result.WriteString("\n")

	// Configuration
	result.WriteString("⚙ Configuration:\n")
	result.WriteString("  • /config          View/update configuration\n")
	result.WriteString("\n")

	result.WriteString("💡 Tips:\n")
	result.WriteString("  • Press Tab for command autocomplete\n")
	result.WriteString("  • Use Esc to interrupt AI responses\n")
	result.WriteString("  • Start with /init to analyze your project\n")

	return result.String()
}
