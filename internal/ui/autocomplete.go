package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// CommandDefinition represents a command with its metadata
type CommandDefinition struct {
	Command     string
	Args        string
	Description string
	Category    string
}

// AutocompleteState manages command autocomplete functionality
type AutocompleteState struct {
	active           bool
	commands         []CommandDefinition
	filteredCommands []CommandDefinition
	selectedIndex    int
	maxVisible       int
}

// NewAutocompleteState creates a new autocomplete state
func NewAutocompleteState() *AutocompleteState {
	commands := []CommandDefinition{
		// Project Setup
		{Command: "/init", Args: "", Description: "Analyze project and create Bazinga.md", Category: "files"},

		// Quick Notes
		// {Command: "/#", Args: "<note>", Description: "Quickly add a timestamped note to Bazinga.md", Category: "files"},
		// {Command: "#", Args: "<note>", Description: "Quickly add a timestamped note to Bazinga.md", Category: "files"},

		// Git Operations
		{Command: "/commit", Args: "[message]", Description: "Commit changes (AI-generated if none provided)", Category: "git"},

		// Memory Management
		{Command: "/memory", Args: "", Description: "View/manage memory", Category: "memory"},

		// Session Management
		// {Command: "/sessions", Args: "", Description: "List and manage saved sessions", Category: "sessions"},

		// Configuration
		{Command: "/config", Args: "", Description: "View/update configuration", Category: "config"},

		// Help
		{Command: "/help", Args: "", Description: "Show available commands", Category: "help"},
	}

	return &AutocompleteState{
		commands:   commands,
		maxVisible: 12, // Show up to 12 commands at once
	}
}

// Update updates the autocomplete based on current input
func (a *AutocompleteState) Update(input string) {
	if !strings.HasPrefix(input, "/") && !strings.HasPrefix(input, "#") {
		a.active = false
		return
	}

	// Extract the command part (before any space)
	parts := strings.Fields(input)
	if len(parts) == 0 {
		a.active = false
		return
	}

	query := parts[0]

	// If command is complete and user is typing args, hide autocomplete
	if len(parts) > 1 {
		a.active = false
		return
	}

	// Filter commands based on input
	a.filteredCommands = []CommandDefinition{}
	for _, cmd := range a.commands {
		if strings.HasPrefix(cmd.Command, query) {
			a.filteredCommands = append(a.filteredCommands, cmd)
		}
	}

	// Sort by relevance (exact prefix matches first, then alphabetical)
	sort.Slice(a.filteredCommands, func(i, j int) bool {
		cmd1, cmd2 := a.filteredCommands[i], a.filteredCommands[j]

		// Exact match comes first
		if cmd1.Command == query {
			return true
		}
		if cmd2.Command == query {
			return false
		}

		// Then by length (shorter commands first)
		if len(cmd1.Command) != len(cmd2.Command) {
			return len(cmd1.Command) < len(cmd2.Command)
		}

		// Finally alphabetical
		return cmd1.Command < cmd2.Command
	})

	a.active = len(a.filteredCommands) > 0
	a.selectedIndex = 0
}

// Navigate changes the selected command
func (a *AutocompleteState) Navigate(direction int) {
	if !a.active || len(a.filteredCommands) == 0 {
		return
	}

	a.selectedIndex += direction
	if a.selectedIndex < 0 {
		a.selectedIndex = len(a.filteredCommands) - 1
	} else if a.selectedIndex >= len(a.filteredCommands) {
		a.selectedIndex = 0
	}
}

// GetSelected returns the currently selected command
func (a *AutocompleteState) GetSelected() *CommandDefinition {
	if !a.active || len(a.filteredCommands) == 0 || a.selectedIndex >= len(a.filteredCommands) {
		return nil
	}
	return &a.filteredCommands[a.selectedIndex]
}

// IsActive returns whether autocomplete is currently active
func (a *AutocompleteState) IsActive() bool {
	return a.active
}

// Deactivate turns off autocomplete
func (a *AutocompleteState) Deactivate() {
	a.active = false
}

// Render renders the autocomplete overlay
func (a *AutocompleteState) Render(width int) string {
	if !a.active || len(a.filteredCommands) == 0 {
		return ""
	}

	var items []string

	// Show up to maxVisible items
	start := 0
	end := len(a.filteredCommands)
	if end > a.maxVisible {
		// Calculate scrolling window based on selected index
		if a.selectedIndex < a.maxVisible {
			// Show from beginning
			start = 0
			end = a.maxVisible
		} else if a.selectedIndex >= len(a.filteredCommands)-a.maxVisible {
			// Show end items
			start = len(a.filteredCommands) - a.maxVisible
			end = len(a.filteredCommands)
		} else {
			// Scroll to keep selected item in view, biased toward showing more after
			start = a.selectedIndex - a.maxVisible/3
			end = start + a.maxVisible
		}
	}

	// Category colors
	categoryColors := map[string]lipgloss.Color{
		"files":   "#b8bb26", // Gruvbox green
		"git":     "#fb4934", // Gruvbox red
		"config":  "#83a598", // Gruvbox blue
		"session": "#d3869b", // Gruvbox purple
		"memory":  "#fabd2f", // Gruvbox yellow
		"help":    "#fe8019", // Gruvbox orange
	}

	for i := start; i < end; i++ {
		cmd := a.filteredCommands[i]
		isSelected := i == a.selectedIndex

		// Build command text
		commandText := cmd.Command
		if cmd.Args != "" {
			commandText += " " + cmd.Args
		}

		// Get category color
		categoryColor, exists := categoryColors[cmd.Category]
		if !exists {
			categoryColor = "#928374" // Gruvbox gray
		}

		var itemStyle lipgloss.Style
		if isSelected {
			// Selected item: highlighted background
			itemStyle = lipgloss.NewStyle().
				Background(categoryColor).
				Foreground(lipgloss.Color("#1d2021")). // Gruvbox dark background
				Bold(true).
				Padding(0, 1)
		} else {
			// Normal item
			itemStyle = lipgloss.NewStyle().
				Foreground(categoryColor).
				Padding(0, 1)
		}

		// Format: "command args" - description
		line := itemStyle.Render(commandText) + " - " +
			lipgloss.NewStyle().
				Foreground(lipgloss.Color("#928374")). // Gruvbox gray
				Render(cmd.Description)

		items = append(items, line)
	}

	// Add more indicator if there are more items
	if end < len(a.filteredCommands) {
		moreText := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#928374")). // Gruvbox gray
			Faint(true).
			Render(fmt.Sprintf("... %d more", len(a.filteredCommands)-end))
		items = append(items, moreText)
	}

	// Create the autocomplete box
	content := strings.Join(items, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#928374")). // Gruvbox gray
		Background(lipgloss.Color("#1d2021")).       // Gruvbox dark background
		Padding(0, 1).
		MaxWidth(width - 4)

	return boxStyle.Render(content)
}

// GetCompletionText returns the text to complete for the selected command
func (a *AutocompleteState) GetCompletionText() string {
	selected := a.GetSelected()
	if selected == nil {
		return ""
	}

	// Return just the command part for completion
	return selected.Command
}
