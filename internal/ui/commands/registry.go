package commands

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Registry manages all available commands
type Registry struct {
	commands map[string]Command
}

// NewRegistry creates a new command registry
func NewRegistry() *Registry {
	registry := &Registry{
		commands: make(map[string]Command),
	}

	// Register essential commands only
	registry.Register(&HelpCommand{})
	registry.Register(&InitCommand{})
	registry.Register(&CommitCommand{})
	registry.Register(&MemoryCommand{})
	registry.Register(&ConfigCommand{})
	registry.Register(&NoteCommand{})

	return registry
}

// Register adds a command to the registry
func (r *Registry) Register(cmd Command) {
	r.commands[cmd.GetName()] = cmd
}

// Execute executes a command by name
func (r *Registry) Execute(ctx context.Context, commandLine string, model CommandModel) tea.Msg {
	parts := strings.Fields(commandLine)
	if len(parts) == 0 {
		return ResponseMsg{Content: "No command provided"}
	}

	// Remove the leading slash or hash
	cmdName := strings.TrimPrefix(strings.TrimPrefix(parts[0], "/"), "#")
	args := parts[1:]

	// Special case for # command - map it to the note command
	if strings.HasPrefix(parts[0], "#") {
		cmdName = "#"
	}

	if cmd, exists := r.commands[cmdName]; exists {
		return cmd.Execute(ctx, args, model)
	}

	// Command not found
	return ResponseMsg{Content: "Unknown command: " + parts[0] + "\nType /help for available commands."}
}

// GetCommand returns a command by name
func (r *Registry) GetCommand(name string) (Command, bool) {
	cmd, exists := r.commands[name]
	return cmd, exists
}

// ListCommands returns all registered commands
func (r *Registry) ListCommands() []Command {
	var commands []Command
	for _, cmd := range r.commands {
		commands = append(commands, cmd)
	}
	return commands
}
