package commands

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ConfigCommand handles the /config command
type ConfigCommand struct{}

func (c *ConfigCommand) Execute(ctx context.Context, args []string, model CommandModel) tea.Msg {
	session := model.GetSession()

	if len(args) == 0 {
		return ResponseMsg{Content: c.showConfig(session)}
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "show", "list":
		return ResponseMsg{Content: c.showConfig(session)}

	case "provider":
		return c.handleProviderConfig(session, subArgs)

	case "model":
		return c.handleModelConfig(session, subArgs)

	default:
		return ResponseMsg{Content: c.showUsage()}
	}
}

func (c *ConfigCommand) GetName() string {
	return "config"
}

func (c *ConfigCommand) GetUsage() string {
	return "/config [show|provider|model] [value]"
}

func (c *ConfigCommand) GetDescription() string {
	return "View or update configuration settings"
}

func (c *ConfigCommand) showConfig(session Session) string {
	var result strings.Builder

	result.WriteString("📋 Current Configuration:\n\n")

	// LLM Settings
	result.WriteString("🤖 LLM Settings:\n")
	result.WriteString(fmt.Sprintf("  • Provider: %s\n", session.GetProvider()))
	result.WriteString(fmt.Sprintf("  • Model: %s\n", session.GetModel()))
	result.WriteString("\n")

	// Available Providers
	result.WriteString("🔌 Available Providers:\n")
	providers := session.GetAvailableProviders()
	for _, provider := range providers {
		indicator := "○"
		if provider == session.GetProvider() {
			indicator = "●"
		}
		result.WriteString(fmt.Sprintf("  %s %s\n", indicator, provider))
	}
	result.WriteString("\n")

	// Available Models for current provider
	result.WriteString("🎯 Available Models (current provider):\n")
	allModels := session.GetAvailableModels()
	if models, exists := allModels[session.GetProvider()]; exists {
		currentModel := session.GetModel()
		for _, model := range models {
			indicator := "○"
			if model.ID == currentModel {
				indicator = "●"
			}
			result.WriteString(fmt.Sprintf("  %s %s\n", indicator, model.Name))
		}
	} else {
		result.WriteString("  No models available for current provider\n")
	}
	result.WriteString("\n")

	result.WriteString("💡 Usage:\n")
	result.WriteString("  • /config provider <name>     - Switch provider\n")
	result.WriteString("  • /config model <name>        - Switch model\n")
	result.WriteString("  • /config show                - Show this information\n")

	return result.String()
}

func (c *ConfigCommand) handleProviderConfig(session Session, args []string) tea.Msg {
	if len(args) == 0 {
		// Show current provider
		return ResponseMsg{Content: fmt.Sprintf("Current provider: %s", session.GetProvider())}
	}

	newProvider := args[0]
	err := session.SetProvider(newProvider)
	if err != nil {
		return ResponseMsg{Content: fmt.Sprintf("❌ Error setting provider: %s", err.Error())}
	}

	return StatusUpdateMsg{
		ModelName: newProvider,
		Response:  fmt.Sprintf("✓ Provider set to: %s", newProvider),
	}
}

func (c *ConfigCommand) handleModelConfig(session Session, args []string) tea.Msg {
	if len(args) == 0 {
		// Show current model
		return ResponseMsg{Content: fmt.Sprintf("Current model: %s", session.GetModel())}
	}

	newModel := args[0]
	err := session.SetModel(newModel)
	if err != nil {
		return ResponseMsg{Content: fmt.Sprintf("❌ Error setting model: %s", err.Error())}
	}

	return StatusUpdateMsg{
		ModelName: newModel,
		Response:  fmt.Sprintf("✓ Model set to: %s", newModel),
	}
}

func (c *ConfigCommand) showUsage() string {
	return `📋 Config Command Usage:\n
🔍 View Configuration:\n
  • /config              - Show current configuration\n
  • /config show         - Show current configuration\n
\n
⚙️ Change Settings:\n
  • /config provider <name>    - Switch LLM provider (bedrock, openai, anthropic)\n
  • /config model <name>       - Switch model\n
\n
💡 Examples:\n
  • /config provider bedrock\n
  • /config model eu.anthropic.claude-3-7-sonnet-20250219-v1:0\n
  • /config show\n
\n
Use '/config show' to see available providers and models.`
}
