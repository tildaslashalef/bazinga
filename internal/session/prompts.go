package session

import (
	"fmt"
	"strings"
)

// buildBazingaPrompt creates a natural, helpful Bazinga system prompt
func (s *Session) buildBazingaPrompt() string {
	// Check if MEMORY.md contains a complete system prompt template
	if s.memoryContent != nil && s.memoryContent.ProjectMemory != "" {
		// If MEMORY.md starts with "You are" or contains system prompt patterns, use it as the main prompt
		projectMemory := strings.TrimSpace(s.memoryContent.ProjectMemory)
		if s.isSystemPromptTemplate(projectMemory) {
			// Use MEMORY.md as the complete system prompt with minimal additions
			prompt := projectMemory

			// Add only essential context (files, user preferences)
			additionalContext := s.buildAdditionalContext()
			if additionalContext != "" {
				prompt += "\n\n" + additionalContext
			}

			return prompt
		}
	}

	// Bazinga system prompt
	prompt := `You are Bazinga, an AI coding assistant that helps with software development. You're precise, thorough, and focused on understanding code deeply before making suggestions.

## Core Capabilities

You have access to comprehensive development tools:
- **File Operations**: Read, write, edit, create, move, copy, delete files and directories
- **Git Integration**: Status, diff, add, commit, log, branch management
- **Code Analysis**: Search patterns, find files, explore project structure  
- **System Operations**: Execute commands, run tests, build projects
- **Project Management**: Todo tracking, memory management

## Bazinga Behavior Patterns

**Intelligent File Reading**: 
- Read files to understand context, not to display content to users
- When analyzing code, read systematically and provide insights
- File content is for your understanding - users see summaries like "Read 245 lines"
- Only show code snippets when explaining specific issues or solutions

**Comprehensive Analysis**:
- For project analysis requests, read ALL relevant files first
- Plan your approach: identify which files to examine and why
- Read main files, configuration files, and key architectural components
- Provide thorough analysis after understanding the full context

**Natural Tool Usage**:
- Use tools fluidly as part of your reasoning process
- **ALWAYS create todos first** for complex requests before using other tools
- **Use bash tool for ALL commands**: When users ask to run, build, test, install, or execute anything, use the bash tool automatically
- **Use search tools naturally**: When you need to find code, functions, or files, use grep, find, or fuzzy_search tools
- Read files to verify assumptions and understand current state
- Check git status before making changes
- Run tests after modifications to ensure correctness
- Update todo status throughout task execution

**Command Execution Guidelines**:
- "run make build" → Use bash tool with command "make build"
- "build the project" → Use bash tool with appropriate build command
- "install dependencies" → Use bash tool with npm install, go mod download, etc.
- "run tests" → Use bash tool with test command
- "find function X" → Use grep tool to search for function definitions
- "where is file Y" → Use find or fuzzy_search tool

**Project Understanding**:
- Always read README, main entry points, and configuration files
- Understand project structure before making suggestions
- Consider existing patterns and conventions in the codebase
- Read related files to understand dependencies and relationships

**Communication Style**:
- Be direct and actionable in your responses
- Focus on what needs to be done and why
- Explain your reasoning when making significant changes
- Ask for clarification when requirements are ambiguous

## Task Management & Todo System

**Smart Todo Usage for Complex Multi-Step Tasks:**

**When to create todos (use TodoWrite when appropriate):**
- User explicitly requests task breakdown or project planning
- Implementation tasks with 4+ distinct development steps  
- User provides explicit numbered or comma-separated task lists
- Multi-file refactoring or architectural changes
- Complex setup or deployment workflows with dependencies

**Simple todo workflow:**
1. **Create todos once** at the start of complex multi-step work
2. **Work through tasks** systematically using appropriate tools
3. **Update status** only when major milestones are completed
4. **Avoid repetitive todo updates** - focus on actual work

**Important: Do NOT create todos for:**
- Simple file analysis or reading tasks
- Single-step operations
- General code review or exploration
- Questions about existing code

**Mandatory todo format:**
- Specific, actionable tasks with clear completion criteria
- Logical order of execution (dependencies first)
- Appropriate priorities: high (critical/blocking), medium (important), low (nice-to-have)
- Granular enough that each todo represents 1-3 tool uses maximum

**REQUIRED progress tracking behavior:**
- Mark "in_progress" BEFORE starting any task (use TodoWrite)
- Mark "completed" IMMEDIATELY after finishing (use TodoWrite)
- Show visual progress update after each completion
- NEVER mark completed if errors occurred or task is partial
- Only have ONE task in_progress at any time

**Visual progress display (mandatory after each todo update):**
- Always show current todo status with visual formatting
- Use format: [x] ✅ completed, [ ] ⏳ in-progress, [ ] ⭕ pending
- Include 🔥 for high priority, 💫 for low priority tasks
- Display progress percentage: "Progress: X/Y tasks completed (Z%)"
- Celebrate with "✨ All tasks completed!" when finished

**Proactive task management:**
- Create todos even for seemingly simple requests if they involve multiple steps
- Break down user requests more granularly than they provided
- Add discovered tasks during execution (e.g., "fix failing tests" if tests break)
- Remove or modify todos that become irrelevant during execution

**Real-world examples:**
- "Add dark mode" → [Read theme files, Create theme config, Add toggle component, Update components, Test switching]
- "Fix the build" → [Check build errors, Identify root causes, Fix dependency issues, Update configs, Verify build success]
- "Analyze this project" → [Read main files, Check architecture, Review dependencies, Identify patterns, Summarize findings]
- "Update documentation" → [Review current docs, Identify gaps, Update content, Check links, Verify formatting]

## Tool Usage Philosophy

- **Smart Planning**: Create todos only for genuinely complex multi-step work
- **Read First**: Always read relevant files before making changes
- **Verify Context**: Check current state before proposing solutions
- **Be Thorough**: For complex tasks, read 5-10+ files to understand fully
- **Natural Tool Use**: Choose the right tool for each specific task
- **Stay Focused**: Tool usage should serve the goal of helping the user effectively

You maintain context across the conversation and can reference previously read files. Focus on providing accurate, helpful assistance based on deep understanding of the codebase.`

	// Add memory context with better organization and hierarchy
	memorySection := s.buildMemorySection()
	if memorySection != "" {
		prompt += "\n\n" + memorySection
	}

	return prompt
}

// buildMemorySection creates a well-formatted memory section with proper hierarchy
func (s *Session) buildMemorySection() string {
	var sections []string

	// User Preferences section
	if s.memoryContent != nil && s.memoryContent.UserMemory != "" {
		sections = append(sections, "## User Preferences")
		sections = append(sections, s.formatMemoryContent(s.memoryContent.UserMemory))
		sections = append(sections, "")
	}

	// Project Context section
	if s.memoryContent != nil && s.memoryContent.ProjectMemory != "" {
		sections = append(sections, "## Project Context")
		sections = append(sections, s.formatMemoryContent(s.memoryContent.ProjectMemory))
		sections = append(sections, "")
	}

	// Current Session Files section
	if len(s.Files) > 0 {
		sections = append(sections, "## Current Session Files")
		sections = append(sections, s.formatSessionFiles())
		sections = append(sections, "")
	}

	// Project Structure section
	if s.project != nil {
		projectSummary := s.project.GetProjectSummary()
		if projectSummary != "No project detected" && projectSummary != "" {
			sections = append(sections, "## Project Structure")
			sections = append(sections, s.formatProjectStructure(projectSummary))
			sections = append(sections, "")
		}
	}

	// Imported Files section (if any)
	if s.memoryContent != nil && len(s.memoryContent.ImportedFiles) > 0 {
		sections = append(sections, "## Imported Memory Files")
		sections = append(sections, s.formatImportedFiles())
		sections = append(sections, "")
	}

	if len(sections) == 0 {
		return ""
	}

	// Remove trailing empty line
	if len(sections) > 0 && sections[len(sections)-1] == "" {
		sections = sections[:len(sections)-1]
	}

	return strings.Join(sections, "\n")
}

// formatMemoryContent formats memory content with proper indentation and structure
func (s *Session) formatMemoryContent(content string) string {
	// Remove any existing top-level headers that might conflict
	lines := strings.Split(content, "\n")
	var formattedLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			formattedLines = append(formattedLines, "")
			continue
		}

		// Convert top-level headers to subsections to maintain hierarchy
		if strings.HasPrefix(trimmed, "# ") {
			formattedLines = append(formattedLines, "### "+strings.TrimPrefix(trimmed, "# "))
		} else if strings.HasPrefix(trimmed, "## ") {
			formattedLines = append(formattedLines, "### "+strings.TrimPrefix(trimmed, "## "))
		} else {
			formattedLines = append(formattedLines, line)
		}
	}

	return strings.Join(formattedLines, "\n")
}

// formatSessionFiles formats the current session files in a clean list
func (s *Session) formatSessionFiles() string {
	if len(s.Files) == 0 {
		return "No files currently loaded in session."
	}

	var fileLines []string
	fileLines = append(fileLines, fmt.Sprintf("**%d files available for analysis:**", len(s.Files)))

	for _, file := range s.Files {
		// Show relative path from root
		relPath := strings.TrimPrefix(file, s.RootPath+"/")
		if relPath == file {
			// Fallback if TrimPrefix didn't work
			relPath = strings.TrimPrefix(file, s.RootPath)
			relPath = strings.TrimPrefix(relPath, "/")
		}
		fileLines = append(fileLines, "- "+relPath)
	}

	// Add hint for comprehensive analysis
	if len(s.Files) > 1 {
		fileLines = append(fileLines, "")
		fileLines = append(fileLines, "*For comprehensive analysis requests, read ALL of these files systematically.*")
	}

	return strings.Join(fileLines, "\n")
}

// formatProjectStructure formats project structure information
func (s *Session) formatProjectStructure(summary string) string {
	// If summary contains multiple lines, format it nicely
	lines := strings.Split(summary, "\n")
	var formattedLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			formattedLines = append(formattedLines, "")
			continue
		}

		// Add proper indentation to structure items if they're not already formatted
		if !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "*") && !strings.HasPrefix(trimmed, "  ") {
			formattedLines = append(formattedLines, "- "+trimmed)
		} else {
			formattedLines = append(formattedLines, line)
		}
	}

	return strings.Join(formattedLines, "\n")
}

// formatImportedFiles formats the list of imported memory files
func (s *Session) formatImportedFiles() string {
	var fileLines []string
	for _, file := range s.memoryContent.ImportedFiles {
		// Show relative path from root if possible
		relPath := strings.TrimPrefix(file, s.RootPath+"/")
		if relPath == file {
			// Fallback if TrimPrefix didn't work
			relPath = strings.TrimPrefix(file, s.RootPath)
			relPath = strings.TrimPrefix(relPath, "/")
		}
		if relPath == file {
			// Still no change, just show the filename
			relPath = strings.TrimPrefix(file, "/")
		}
		fileLines = append(fileLines, "- "+relPath)
	}
	return strings.Join(fileLines, "\n")
}

// isSystemPromptTemplate detects if content appears to be a complete system prompt
func (s *Session) isSystemPromptTemplate(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}

	// Check for common system prompt patterns
	patterns := []string{
		"You are an expert",
		"You are a",
		"You are specialized",
		"# TASK",
		"# RESPONSE INSTRUCTIONS",
		"```\nYou are", // Template blocks
	}

	contentLower := strings.ToLower(content)
	for _, pattern := range patterns {
		if strings.Contains(contentLower, strings.ToLower(pattern)) {
			return true
		}
	}

	// Check if it starts with a typical system prompt beginning
	if strings.HasPrefix(contentLower, "you are") {
		return true
	}

	return false
}

// buildAdditionalContext builds minimal context when using MEMORY.md as main prompt
func (s *Session) buildAdditionalContext() string {
	var sections []string

	// User Preferences (always useful)
	if s.memoryContent != nil && s.memoryContent.UserMemory != "" {
		sections = append(sections, "## User Preferences")
		sections = append(sections, s.formatMemoryContent(s.memoryContent.UserMemory))
		sections = append(sections, "")
	}

	// Current Session Files (essential for tool usage)
	if len(s.Files) > 0 {
		sections = append(sections, "## Current Session Files")
		sections = append(sections, s.formatSessionFiles())
		sections = append(sections, "")
	}

	if len(sections) == 0 {
		return ""
	}

	// Remove trailing empty line
	if len(sections) > 0 && sections[len(sections)-1] == "" {
		sections = sections[:len(sections)-1]
	}

	return strings.Join(sections, "\n")
}
