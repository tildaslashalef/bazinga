package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tildaslashalef/bazinga/internal/memory"
)

// MemoryCommand handles the /memory command
type MemoryCommand struct{}

func (c *MemoryCommand) Execute(ctx context.Context, args []string, model CommandModel) tea.Msg {
	session := model.GetSession()

	if len(args) == 0 {
		response := c.formatMemoryHelp()
		return ResponseMsg{Content: response}
	}

	command := args[0]
	var response string
	var err error

	switch command {
	case "show":
		memContent := session.GetMemoryContent()
		response = c.formatMemoryContent(memContent)

	case "paths":
		userPath, projectPath := session.GetMemoryFilePaths()
		_, userErr := os.Stat(userPath)
		_, projectErr := os.Stat(projectPath)
		response = c.formatMemoryPaths(userPath, projectPath, userErr == nil, projectErr == nil)

	case "create":
		var isUserMemory bool
		var memoryType string

		if len(args) > 1 {
			memoryType = args[1]
			switch memoryType {
			case "user":
				isUserMemory = true
			case "project":
				isUserMemory = false
			default:
				response = c.formatError("Usage: /memory create [user|project]\nDefaults to project if not specified.")
				return ResponseMsg{Content: response}
			}
		} else {
			// Default to project memory
			isUserMemory = false
			memoryType = "project"
		}

		err = session.CreateMemoryFile(ctx, isUserMemory)
		if err != nil {
			response = c.formatError(fmt.Sprintf("Error creating %s memory file: %s", memoryType, err.Error()))
		} else {
			userPath, projectPath := session.GetMemoryFilePaths()
			var createdPath string
			if isUserMemory {
				createdPath = userPath
			} else {
				createdPath = projectPath
			}
			response = c.formatSuccess(fmt.Sprintf("Created %s memory file: %s\nUse your editor to customize it.", memoryType, createdPath))
		}

	case "reload":
		err = session.ReloadMemory(ctx)
		if err != nil {
			response = c.formatError("Error reloading memory: " + err.Error())
		} else {
			response = c.formatSuccess("Memory reloaded successfully.")
		}

	default:
		if strings.HasPrefix(command, "#") {
			// Quick note shortcut: /memory #This is a quick note
			note := strings.Join(args, " ")[1:] // Remove the # and join
			if note == "" {
				response = c.formatError("Usage: /memory #Your quick note here")
			} else {
				// Default to project memory for quick notes
				err = session.AddQuickMemory(ctx, note, false)
				if err != nil {
					response = c.formatError("Error adding quick note: " + err.Error())
				} else {
					response = c.formatSuccess(fmt.Sprintf("Added quick note to project memory: %s", note))
				}
			}
		} else {
			response = c.formatError(fmt.Sprintf("Unknown memory command: %s\nType '/memory' for help.", command))
		}
	}

	if err != nil {
		response = c.formatError(fmt.Sprintf("Memory error: %v", err))
	}

	return ResponseMsg{Content: response}
}

func (c *MemoryCommand) GetName() string {
	return "memory"
}

func (c *MemoryCommand) GetUsage() string {
	return "/memory [show|paths|create|reload|#note]"
}

func (c *MemoryCommand) GetDescription() string {
	return "Memory management commands"
}

// Formatting functions
func (c *MemoryCommand) formatError(content string) string {
	return fmt.Sprintf("✗ %s", content)
}

func (c *MemoryCommand) formatWarning(content string) string {
	return fmt.Sprintf("⚠ %s", content)
}

func (c *MemoryCommand) formatSuccess(content string) string {
	return fmt.Sprintf("✓ %s", content)
}

func (c *MemoryCommand) formatMemoryHelp() string {
	var result strings.Builder

	result.WriteString("🧠 **Memory Management Commands:**\n\n")
	result.WriteString("- `/memory show` - Display current memory content\n")
	result.WriteString("- `/memory create` - Create memory files\n")
	result.WriteString("- `/memory paths` - Show memory file paths and status\n")
	result.WriteString("- `/memory reload` - Reload memory from files\n")
	result.WriteString("- `/memory #note` - Add quick note to project memory\n")

	return result.String()
}

func (c *MemoryCommand) formatMemoryPaths(userPath, projectPath string, userExists, projectExists bool) string {
	var result strings.Builder

	result.WriteString("🧠 **Memory File Paths:**\n\n")

	// User memory status
	userStatus := "✓ Exists"
	if !userExists {
		userStatus = "✗ Not found"
	}
	result.WriteString(fmt.Sprintf("👤 **User Memory:** %s\n", userStatus))
	result.WriteString(fmt.Sprintf("   `%s`\n\n", userPath))

	// Project memory status
	projectStatus := "✓ Exists"
	if !projectExists {
		projectStatus = "✗ Not found"
	}
	result.WriteString(fmt.Sprintf("📁 **Project Memory:** %s\n", projectStatus))
	result.WriteString(fmt.Sprintf("   `%s`\n", projectPath))

	return result.String()
}

func (c *MemoryCommand) formatMemoryContent(memContent interface{}) string {
	if memContent == nil {
		return c.formatWarning("No memory content available")
	}

	var result strings.Builder
	result.WriteString("🧠 Memory Content:\n\n")

	// Type assertion to access memory content fields
	content, ok := memContent.(*memory.MemoryContent)
	if !ok {
		return c.formatError("Failed to parse memory content format")
	}

	// User memory
	result.WriteString("👤 **User Memory:**\n")
	if content.UserMemory != "" {
		result.WriteString(fmt.Sprintf("```\n%s\n```\n\n", strings.TrimSpace(content.UserMemory)))
	} else {
		result.WriteString("   *No user memory found*\n\n")
	}

	// Project memory
	result.WriteString("📁 **Project Memory:**\n")
	if content.ProjectMemory != "" {
		result.WriteString(fmt.Sprintf("```\n%s\n```\n\n", strings.TrimSpace(content.ProjectMemory)))
	} else {
		result.WriteString("   *No project memory found*\n\n")
	}

	// Imported files
	result.WriteString("📄 **Imported Files:**\n")
	if len(content.ImportedFiles) > 0 {
		for _, file := range content.ImportedFiles {
			result.WriteString(fmt.Sprintf("- %s\n", file))
		}
	} else {
		result.WriteString("   *No files imported*\n")
	}

	return result.String()
}
