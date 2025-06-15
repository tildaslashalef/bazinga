package project

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PromptBuilder creates specialized system prompts based on project type
type PromptBuilder struct {
	project *Project
}

// NewPromptBuilder creates a new prompt builder for the given project
func NewPromptBuilder(project *Project) *PromptBuilder {
	return &PromptBuilder{project: project}
}

// BuildSystemPrompt creates a comprehensive system prompt tailored to the project
func (pb *PromptBuilder) BuildSystemPrompt() string {
	var prompt strings.Builder

	// Base introduction
	prompt.WriteString("You are bazinga, an expert AI coding assistant specialized in ")
	prompt.WriteString(pb.getLanguageDescription())
	prompt.WriteString(".\n\n")

	// Project context
	prompt.WriteString("## Current Project Context\n")
	prompt.WriteString(fmt.Sprintf("Project: %s (%s)\n", pb.project.Name, pb.project.Type))
	prompt.WriteString(fmt.Sprintf("Root Directory: %s\n", filepath.Base(pb.project.Root)))
	prompt.WriteString(fmt.Sprintf("Files in Context: %d relevant files\n\n", len(pb.project.Files)))

	// Project structure overview
	if len(pb.project.Files) > 0 {
		prompt.WriteString("## Project Structure\n")
		prompt.WriteString(pb.buildStructureOverview())
		prompt.WriteString("\n")
	}

	// Language-specific guidelines
	prompt.WriteString("## Guidelines\n")
	prompt.WriteString(pb.getLanguageGuidelines())
	prompt.WriteString("\n")

	// Best practices
	prompt.WriteString("## Best Practices to Follow\n")
	prompt.WriteString(pb.getBestPractices())
	prompt.WriteString("\n")

	// Tools and capabilities
	prompt.WriteString("## Available Tools\n")
	prompt.WriteString(`- File operations: Read, write, and modify files in the project
- Git operations: Show diffs, commit changes, manage branches
- Code analysis: Understand project structure and dependencies
- Testing: Run and analyze test results
- Documentation: Generate and update documentation

`)

	// Response guidelines
	prompt.WriteString("## Response Guidelines\n")
	prompt.WriteString(`- Be concise and practical in your suggestions
- Always consider the existing project structure and patterns
- Suggest specific file paths and code changes when appropriate
- Follow the project's existing coding style and conventions
- When making changes, explain the reasoning behind them
- If you need to see more files or context, ask specifically for what you need

Ready to help with your ` + string(pb.project.Type) + ` project!`)

	return prompt.String()
}

// getLanguageDescription returns a description of the primary language/technology
func (pb *PromptBuilder) getLanguageDescription() string {
	switch pb.project.Type {
	case ProjectTypeGo:
		return "Go development, following Go idioms, best practices, and the standard library"
	case ProjectTypeJavaScript:
		return "JavaScript development, including modern ES6+ features, Node.js, and popular frameworks"
	case ProjectTypeTypeScript:
		return "TypeScript development, emphasizing type safety, modern patterns, and best practices"
	case ProjectTypePython:
		return "Python development, following PEP standards, pythonic patterns, and modern Python features"
	case ProjectTypeRust:
		return "Rust development, emphasizing memory safety, performance, and idiomatic Rust patterns"
	case ProjectTypeJava:
		return "Java development, following established patterns, enterprise practices, and modern Java features"
	default:
		return "software development with a focus on clean, maintainable code"
	}
}

// getLanguageGuidelines returns language-specific guidelines
func (pb *PromptBuilder) getLanguageGuidelines() string {
	switch pb.project.Type {
	case ProjectTypeGo:
		return `- Follow Go naming conventions (PascalCase for exported, camelCase for unexported)
- Use gofmt and goimports for formatting
- Write idiomatic Go code with proper error handling
- Prefer composition over inheritance
- Use interfaces effectively
- Follow the "accept interfaces, return structs" principle
- Write comprehensive tests with table-driven test patterns
- Use context.Context for cancellation and timeouts`

	case ProjectTypeJavaScript:
		return `- Use modern ES6+ syntax and features
- Follow consistent naming conventions (camelCase for variables/functions)
- Use proper async/await patterns for asynchronous code
- Implement proper error handling
- Use modules and proper imports/exports
- Write clear, self-documenting code
- Follow functional programming principles where appropriate
- Use proper dependency management with npm/yarn`

	case ProjectTypeTypeScript:
		return `- Leverage TypeScript's type system effectively
- Use strict type checking
- Define proper interfaces and types
- Use generics appropriately
- Follow consistent naming conventions
- Implement proper error handling with typed errors
- Use proper module organization
- Write type-safe code with minimal any usage
- Use modern async/await patterns`

	case ProjectTypePython:
		return `- Follow PEP 8 style guidelines
- Use type hints for better code documentation
- Write pythonic code with proper use of Python idioms
- Use virtual environments for dependency management
- Follow proper package structure
- Write comprehensive docstrings
- Use proper exception handling
- Leverage Python's standard library effectively
- Write unit tests with pytest or unittest`

	case ProjectTypeRust:
		return `- Follow Rust naming conventions and idioms
- Use ownership and borrowing effectively
- Handle errors with Result and Option types
- Write safe, memory-efficient code
- Use proper lifetime annotations when needed
- Leverage the type system for safety
- Write comprehensive tests
- Use cargo for dependency management
- Follow clippy suggestions for code quality`

	case ProjectTypeJava:
		return `- Follow Java naming conventions and best practices
- Use proper access modifiers
- Implement proper exception handling
- Use design patterns appropriately
- Write clean, object-oriented code
- Use proper dependency injection
- Write comprehensive unit tests with JUnit
- Follow SOLID principles
- Use proper package organization`

	default:
		return `- Write clean, readable, and maintainable code
- Follow consistent naming and formatting conventions
- Implement proper error handling
- Write comprehensive tests
- Use version control effectively
- Document code appropriately`
	}
}

// getBestPractices returns project-type-specific best practices
func (pb *PromptBuilder) getBestPractices() string {
	switch pb.project.Type {
	case ProjectTypeGo:
		return `- Keep packages focused and cohesive
- Use meaningful package names that describe their purpose
- Handle errors explicitly, don't ignore them
- Use defer for cleanup operations
- Keep functions small and focused
- Use channels for goroutine communication
- Write benchmarks for performance-critical code
- Use build tags for conditional compilation`

	case ProjectTypeJavaScript:
		return `- Use const and let instead of var
- Implement proper error boundaries in React applications
- Use proper state management patterns
- Optimize for performance with proper bundling
- Use ESLint and Prettier for code quality
- Implement proper testing strategies
- Use semantic versioning for packages
- Handle asynchronous operations properly`

	case ProjectTypeTypeScript:
		return `- Use strict TypeScript configuration
- Define proper API interfaces
- Use union types and type guards effectively
- Implement proper error handling with typed errors
- Use generics for reusable components
- Write type-safe code with minimal type assertions
- Use proper module resolution
- Leverage TypeScript's utility types`

	case ProjectTypePython:
		return `- Use virtual environments for isolation
- Follow the Zen of Python principles
- Use list/dict comprehensions appropriately
- Implement proper logging instead of print statements
- Use dataclasses or named tuples for structured data
- Write type hints for better code clarity
- Use proper package management with requirements.txt or pyproject.toml
- Implement proper testing with pytest`

	case ProjectTypeRust:
		return `- Use Result and Option types instead of exceptions
- Leverage the ownership system for memory safety
- Use iterators instead of explicit loops where appropriate
- Write comprehensive error types
- Use traits for shared behavior
- Implement proper testing with cargo test
- Use clippy for code quality checks
- Follow semantic versioning for crates`

	case ProjectTypeJava:
		return `- Use dependency injection frameworks appropriately
- Implement proper logging with SLF4J
- Use streams and functional programming features
- Write comprehensive unit and integration tests
- Use proper build tools (Maven/Gradle)
- Implement proper exception hierarchies
- Use design patterns appropriately
- Follow enterprise development practices`

	default:
		return `- Write self-documenting code with clear names
- Implement comprehensive error handling
- Use version control effectively with meaningful commits
- Write automated tests for critical functionality
- Follow security best practices
- Optimize for maintainability over premature optimization
- Document complex algorithms and business logic`
	}
}

// buildStructureOverview creates a high-level overview of the project structure
func (pb *PromptBuilder) buildStructureOverview() string {
	if len(pb.project.Files) == 0 {
		return "No files currently loaded in context."
	}

	var overview strings.Builder

	// Group files by directory
	dirFiles := make(map[string][]string)
	for _, file := range pb.project.Files {
		dir := filepath.Dir(file)
		if dir == "." {
			dir = "root"
		}
		dirFiles[dir] = append(dirFiles[dir], filepath.Base(file))
	}

	// Show main directories
	overview.WriteString("Key directories and files:\n")
	for dir, files := range dirFiles {
		if len(files) <= 3 {
			overview.WriteString(fmt.Sprintf("- %s/: %s\n", dir, strings.Join(files, ", ")))
		} else {
			overview.WriteString(fmt.Sprintf("- %s/: %s and %d more files\n",
				dir, strings.Join(files[:3], ", "), len(files)-3))
		}
	}

	return overview.String()
}

// BuildContextPrompt creates a prompt that includes the current project context
func (pb *PromptBuilder) BuildContextPrompt(userMessage string) string {
	var prompt strings.Builder

	prompt.WriteString("Current project context:\n")
	prompt.WriteString(fmt.Sprintf("- Project: %s (%s)\n", pb.project.Name, pb.project.Type))
	prompt.WriteString(fmt.Sprintf("- %d files in context\n", len(pb.project.Files)))

	if len(pb.project.Files) > 0 {
		mainFiles := pb.project.GetMainFiles()
		if len(mainFiles) > 0 {
			prompt.WriteString(fmt.Sprintf("- Key files: %s\n", strings.Join(mainFiles, ", ")))
		}
	}

	prompt.WriteString("\nUser request: ")
	prompt.WriteString(userMessage)

	return prompt.String()
}
