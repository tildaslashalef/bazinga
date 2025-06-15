package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Gruvbox Dark color palette
var (
	// Primary colors
	AccentColor    = lipgloss.Color("#fe8019") // Gruvbox bright orange
	SecondaryColor = lipgloss.Color("#83a598") // Gruvbox bright blue
	SuccessColor   = lipgloss.Color("#b8bb26") // Gruvbox bright green
	ErrorColor     = lipgloss.Color("#fb4934") // Gruvbox bright red
	WarningColor   = lipgloss.Color("#fabd2f") // Gruvbox bright yellow

	// Neutral colors
	TextPrimary   = lipgloss.Color("#ebdbb2") // Gruvbox light
	TextSecondary = lipgloss.Color("#a89984") // Gruvbox gray
	TextMuted     = lipgloss.Color("#928374") // Gruvbox dark gray

	// Background colors
	BackgroundPrimary   = lipgloss.Color("#1d2021") // Gruvbox hard dark
	BackgroundSecondary = lipgloss.Color("#282828") // Gruvbox dark
	BorderColor         = lipgloss.Color("#504945") // Gruvbox medium gray
)

// Common style definitions
var (
	// Base styles
	BaseStyle = lipgloss.NewStyle().
			Foreground(TextPrimary).
			Background(BackgroundPrimary)

	// Plain text style for logo - minimal formatting to avoid terminal issues
	LogoStyle = lipgloss.NewStyle().
			Bold(true)

	// Header styles
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(AccentColor).
			Background(BackgroundSecondary).
			Padding(0, 1).
			MarginBottom(1)

	// Chat styles
	ChatPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(1).
			MarginRight(1)

	UserMessageStyle = lipgloss.NewStyle().
				Foreground(TextPrimary).
				Background(BackgroundSecondary).
				Padding(0, 1).
				MarginBottom(1).
				Bold(true)

	AssistantMessageStyle = lipgloss.NewStyle().
				Foreground(AccentColor).
				MarginBottom(1)

	SystemMessageStyle = lipgloss.NewStyle().
				Foreground(TextMuted).
				Italic(true).
				MarginBottom(1)

	// Input styles
	InputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(AccentColor).
			Padding(0, 1).
			MarginTop(1)

	InputPromptStyle = lipgloss.NewStyle().
				Foreground(AccentColor).
				Bold(true)

	// Status styles
	StatusBarStyle = lipgloss.NewStyle().
			Background(BackgroundSecondary).
			Foreground(TextSecondary).
			Padding(0, 1)

	StatusItemStyle = lipgloss.NewStyle().
			Foreground(TextSecondary).
			MarginRight(2)

	StatusActiveStyle = lipgloss.NewStyle().
				Foreground(SuccessColor).
				Bold(true).
				MarginRight(2)

	// Code styles
	CodeBlockStyle = lipgloss.NewStyle().
			Background(BackgroundSecondary).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(1).
			MarginTop(1).
			MarginBottom(1)

	// Progress styles
	ProgressStyle = lipgloss.NewStyle().
			Foreground(AccentColor)

	SpinnerStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	// Help styles
	HelpStyle = lipgloss.NewStyle().
			Foreground(TextMuted).
			MarginTop(1)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(TextSecondary)
)

// GetChatDimensions returns adjusted dimensions for single-pane chat
func GetChatDimensions(width, height int) (chatWidth, chatHeight int) {
	// Full width chat
	chatWidth = width - 4 // Account for margins

	// Full height minus status bar, input area, and margins
	chatHeight = height - 10 // Account for header, status, input, help text
	if chatHeight < 10 {
		chatHeight = 10
	}

	return chatWidth, chatHeight
}

// RenderTitle creates a styled title bar
func RenderTitle(title string, width int) string {
	titleStyle := HeaderStyle.Width(width).Align(lipgloss.Center)
	return titleStyle.Render(title)
}

// RenderStatusBar creates a styled status bar
func RenderStatusBar(items []StatusItem, width int) string {
	var parts []string

	for _, item := range items {
		var style lipgloss.Style
		if item.Active {
			style = StatusActiveStyle
		} else {
			style = StatusItemStyle
		}
		parts = append(parts, style.Render(item.Text))
	}

	content := lipgloss.JoinHorizontal(lipgloss.Left, parts...)
	return StatusBarStyle.Width(width).Render(content)
}

// StatusItem represents an item in the status bar
type StatusItem struct {
	Text   string
	Active bool
}

// RenderHelp creates styled help text
func RenderHelp(shortcuts []HelpShortcut) string {
	var parts []string

	for _, shortcut := range shortcuts {
		key := HelpKeyStyle.Render(shortcut.Key)
		desc := HelpDescStyle.Render(shortcut.Description)
		parts = append(parts, key+" "+desc)
	}

	return HelpStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, parts...))
}

// HelpShortcut represents a keyboard shortcut
type HelpShortcut struct {
	Key         string
	Description string
}
