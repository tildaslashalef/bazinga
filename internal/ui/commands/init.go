package commands

import (
	"context"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// InitCommand handles the /init command for project analysis and Bazinga.md creation
type InitCommand struct{}

func (c *InitCommand) Execute(ctx context.Context, args []string, model CommandModel) tea.Msg {
	session := model.GetSession()

	// Check if Bazinga.md already exists
	bazingaMdPath := filepath.Join(session.GetRootPath(), "Bazinga.md")
	if _, err := os.Stat(bazingaMdPath); err == nil {
		// File exists, ask for confirmation to overwrite
		return ResponseMsg{
			Content: "Bazinga.md already exists. Would you like me to analyze the codebase and suggest improvements to it?",
		}
	}

	// No Bazinga.md exists, start analysis
	return c.performAnalysis(ctx, model, false)
}

func (c *InitCommand) performAnalysis(ctx context.Context, model CommandModel, isUpdate bool) tea.Msg {
	session := model.GetSession()

	// Add status message that we're analyzing
	statusMsg := "🔍 Analyzing codebase to understand its structure and create a comprehensive Bazinga.md file..."
	model.AddMessage("system", statusMsg, false)

	// Detect project structure
	project := session.GetProject()
	if project == nil {
		return ResponseMsg{Content: "❌ No project detected. Make sure you're in a valid project directory."}
	}

	// Get key architectural files
	keyFiles := c.getKeyArchitecturalFiles(session.GetRootPath(), project)

	// Add files to session for analysis
	var readFiles []string
	for _, file := range keyFiles {
		fullPath := filepath.Join(session.GetRootPath(), file)
		if err := session.AddFile(ctx, fullPath); err != nil {
			loggy.Debug("Failed to add file for analysis", "file", file, "error", err)
		} else {
			readFiles = append(readFiles, file)
		}
	}

	// Create the analysis prompt
	prompt := c.createAnalysisPrompt(isUpdate, readFiles, project.Root())

	// Send the analysis prompt to the LLM for processing and file creation
	fullPrompt := prompt + `

**CRITICAL REQUIREMENT - FILE CREATION MANDATORY:**

This is a /init command - DO NOT create todos for this task. Instead, immediately create the Bazinga.md file.

You MUST use the write_file tool to create the Bazinga.md file. This is required for the /init command to work correctly.

REQUIRED TOOL CALL (execute this now):
{
  "name": "write_file",
  "input": {
    "file_path": "Bazinga.md",
    "content": "[your complete analysis and guidance content here]"
  }
}

IMPORTANT EXECUTION ORDER:
1. Brief analysis (1-2 sentences)
2. IMMEDIATELY call write_file tool with complete Bazinga.md content
3. DO NOT create todos or additional analysis

The /init command REQUIRES you to create the actual file. Your response should be:
- Brief analysis
- write_file tool call with complete content
- Success confirmation`

	// Return a message that will trigger LLM processing
	return LLMRequestMsg{Message: fullPrompt}
}

func (c *InitCommand) getKeyArchitecturalFiles(rootPath string, project Project) []string {
	var files []string

	// Always include common project files
	commonFiles := []string{
		"README.md", "readme.md", "Readme.md",
		"package.json", "go.mod", "Cargo.toml", "pyproject.toml", "requirements.txt",
		"Makefile", "makefile", "build.gradle", "pom.xml",
		".gitignore", "tsconfig.json", "webpack.config.js",
		"docker-compose.yml", "Dockerfile",
	}

	for _, file := range commonFiles {
		if c.fileExists(rootPath, file) {
			files = append(files, file)
		}
	}

	// Add project-specific key files based on type
	// Note: project.Type would need to be exposed via the Project interface
	// For now, we'll detect based on files present

	// Add any existing Bazinga.md or similar documentation
	docFiles := []string{"Bazinga.md", "DEVELOPMENT.md", "CONTRIBUTING.md", "ARCHITECTURE.md"}
	for _, file := range docFiles {
		if c.fileExists(rootPath, file) {
			files = append(files, file)
		}
	}

	return files
}

func (c *InitCommand) fileExists(rootPath, file string) bool {
	_, err := os.Stat(filepath.Join(rootPath, file))
	return err == nil
}

func (c *InitCommand) createAnalysisPrompt(isUpdate bool, readFiles []string, projectRoot string) string {
	var prompt strings.Builder

	if isUpdate {
		prompt.WriteString("Please analyze this codebase and suggest improvements to the existing Bazinga.md file.\n\n")
	} else {
		prompt.WriteString("TASK: Create a Bazinga.md file using the write_file tool.\n\nFirst, analyze this codebase briefly, then immediately create the Bazinga.md file.\n\n")
	}

	prompt.WriteString("What to add:\n")
	prompt.WriteString("1. Commands that will be commonly used, such as how to build, lint, and run tests. Include the necessary commands to develop in this codebase, such as how to run a single test.\n")
	prompt.WriteString("2. High-level code architecture and structure so that future instances can be productive more quickly. Focus on the \"big picture\" architecture that requires reading multiple files to understand\n\n")

	prompt.WriteString("Usage notes:\n")
	if isUpdate {
		prompt.WriteString("- Suggest improvements to the existing Bazinga.md\n")
	} else {
		prompt.WriteString("- When you make the initial Bazinga.md, do not repeat yourself and do not include obvious instructions\n")
	}
	prompt.WriteString("- Avoid listing every component or file structure that can be easily discovered\n")
	prompt.WriteString("- Don't include generic development practices\n")
	prompt.WriteString("- If there are Cursor rules or Copilot rules, make sure to include the important parts\n")
	prompt.WriteString("- If there is a README.md, make sure to include the important parts\n")
	prompt.WriteString("- Do not make up information unless this is expressly included in other files that you read\n")
	prompt.WriteString("- Be sure to prefix the file with the following text:\n\n")
	prompt.WriteString("```\n# Bazinga.md\n\nThis file provides guidance to Bazinga when working with code in this repository.\n```\n\n")

	if len(readFiles) > 0 {
		prompt.WriteString("I have read the following key files for analysis:\n")
		for _, file := range readFiles {
			prompt.WriteString(fmt.Sprintf("- %s\n", file))
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString(fmt.Sprintf("Project root: %s\n", projectRoot))

	return prompt.String()
}

func (c *InitCommand) GetName() string {
	return "init"
}

func (c *InitCommand) GetUsage() string {
	return "/init"
}

func (c *InitCommand) GetDescription() string {
	return "Analyze the codebase to understand its structure and create a comprehensive Bazinga.md file"
}
