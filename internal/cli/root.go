package cli

import (
	"context"
	"fmt"
	"github.com/tildaslashalef/bazinga/internal/config"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"github.com/tildaslashalef/bazinga/internal/llm/anthropic"
	"github.com/tildaslashalef/bazinga/internal/llm/bedrock"
	"github.com/tildaslashalef/bazinga/internal/llm/ollama"
	"github.com/tildaslashalef/bazinga/internal/llm/openai"
	"github.com/tildaslashalef/bazinga/internal/loggy"
	"github.com/tildaslashalef/bazinga/internal/session"
	"github.com/tildaslashalef/bazinga/internal/ui"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// BuildInfo contains build-time information
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
	Author  string
	Email   string
}

// GlobalFlags contains global command-line flags
type GlobalFlags struct {
	ConfigFile string
	Model      string
	Provider   string
	Region     string
	SessionID  string
	Terminator bool // Bypass all permission checks
}

// NewRootCommand creates the root cobra command
func NewRootCommand(buildInfo *BuildInfo) *cobra.Command {
	var flags GlobalFlags

	cmd := &cobra.Command{
		Use:   "bazinga [files...]",
		Short: "AI-powered coding assistant CLI",
		Long: `bazinga is an AI-powered coding assistant that helps you write, edit, and review code
using Large Language Models. It supports multiple providers and maintains session context
for intelligent pair programming.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", buildInfo.Version, buildInfo.Commit, buildInfo.Date),
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInteractiveSession(cmd.Context(), &flags, args)
		},
		SilenceUsage: true,
	}

	// Global flags
	cmd.PersistentFlags().StringVar(&flags.ConfigFile, "config", "", "config file (default: ~/.github.com/tildaslashalef/bazinga/config.yaml)")
	cmd.PersistentFlags().StringVar(&flags.Model, "model", "", "LLM model to use")
	cmd.PersistentFlags().StringVar(&flags.Provider, "provider", "", "LLM provider (bedrock, openai, anthropic, ollama)")
	cmd.PersistentFlags().StringVar(&flags.Region, "region", "", "AWS region for Bedrock")
	cmd.PersistentFlags().StringVar(&flags.SessionID, "session", "", "Continue existing session by ID")
	cmd.PersistentFlags().BoolVar(&flags.Terminator, "terminator", false, "DANGEROUS: Bypass all permission checks")

	// Add subcommands
	cmd.AddCommand(newVersionCommand(buildInfo))

	// Setup configuration
	cobra.OnInitialize(func() {
		initConfig(&flags)
	})

	return cmd
}

// initConfig initializes the configuration
func initConfig(flags *GlobalFlags) {
	if flags.ConfigFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(flags.ConfigFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding home directory: %v\n", err)
			os.Exit(1)
		}

		// Search config in home directory with name ".bazinga" (without extension)
		configDir := filepath.Join(home, ".bazinga")
		viper.AddConfigPath(configDir)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	// Environment variable prefix
	viper.SetEnvPrefix("BAZINGA")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		// If config file not found, create a default one
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("No config file found, creating default config...")
			if err := config.Init(); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating default config: %v\n", err)
			}
			// Try to read the newly created config
			if err := viper.ReadInConfig(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not read created config file: %v\n", err)
			}
		} else {
			// This is a different error than not found
			fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
			os.Exit(1)
		}
	}
}

// runInteractiveSession starts an interactive coding session
func runInteractiveSession(ctx context.Context, flags *GlobalFlags, files []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Reconfigure logging with the loaded config
	if err := loggy.Reconfigure(&cfg.Logging); err != nil {
		fmt.Printf("Warning: Failed to reconfigure logging: %v\n", err)
	}

	// Override config with command-line flags
	if flags.Model != "" {
		cfg.LLM.DefaultModel = flags.Model
	}
	if flags.Provider != "" {
		cfg.LLM.DefaultProvider = flags.Provider
	}
	if flags.Region != "" {
		cfg.Providers.Bedrock.Region = flags.Region
	}
	if flags.Terminator {
		cfg.Security.Terminator = true
		fmt.Printf("⚠️  TERMINATOR MODE ENABLED - All permission checks bypassed!\n")
	}

	// Initialize LLM manager
	llmManager := llm.NewManager()

	// Register Bedrock provider
	if cfg.Providers.Bedrock.Enabled {
		bedrockProvider, err := bedrock.NewProvider(&bedrock.Config{
			Region:       cfg.Providers.Bedrock.Region,
			AccessKeyID:  cfg.Providers.Bedrock.AccessKeyID,
			SecretKey:    cfg.Providers.Bedrock.SecretAccessKey,
			SessionToken: cfg.Providers.Bedrock.SessionToken,
			Profile:      cfg.Providers.Bedrock.Profile,
			AuthMethod:   cfg.Providers.Bedrock.AuthMethod,
		})
		if err != nil {
			return fmt.Errorf("failed to create Bedrock provider: %w", err)
		}

		if err := llmManager.RegisterProvider("bedrock", bedrockProvider); err != nil {
			return fmt.Errorf("failed to register Bedrock provider: %w", err)
		}

		// Set as default if specified
		if cfg.LLM.DefaultProvider == "bedrock" || cfg.LLM.DefaultProvider == "" {
			if err := llmManager.SetDefaultProvider("bedrock"); err != nil {
				return fmt.Errorf("failed to set default provider: %w", err)
			}
		}
	}

	// Register OpenAI provider
	if cfg.Providers.OpenAI.Enabled && cfg.Providers.OpenAI.APIKey != "" {
		openaiProvider := openai.NewProviderWithConfig(&openai.Config{
			APIKey:  cfg.Providers.OpenAI.APIKey,
			BaseURL: cfg.Providers.OpenAI.BaseURL,
			OrgID:   cfg.Providers.OpenAI.OrgID,
		})
		if err := llmManager.RegisterProvider("openai", openaiProvider); err != nil {
			return fmt.Errorf("failed to register OpenAI provider: %w", err)
		}

		// Set as default if specified
		if cfg.LLM.DefaultProvider == "openai" {
			if err := llmManager.SetDefaultProvider("openai"); err != nil {
				return fmt.Errorf("failed to set default provider: %w", err)
			}
		}
	}

	// Register Anthropic provider
	if cfg.Providers.Anthropic.Enabled && cfg.Providers.Anthropic.APIKey != "" {
		anthropicProvider := anthropic.NewProviderWithConfig(&anthropic.Config{
			APIKey:  cfg.Providers.Anthropic.APIKey,
			BaseURL: cfg.Providers.Anthropic.BaseURL,
		})
		if err := llmManager.RegisterProvider("anthropic", anthropicProvider); err != nil {
			return fmt.Errorf("failed to register Anthropic provider: %w", err)
		}

		// Set as default if specified
		if cfg.LLM.DefaultProvider == "anthropic" {
			if err := llmManager.SetDefaultProvider("anthropic"); err != nil {
				return fmt.Errorf("failed to set default provider: %w", err)
			}
		}
	}

	// Register Ollama provider
	if cfg.Providers.Ollama.Enabled {
		ollamaProvider := ollama.NewProviderWithConfig(&ollama.Config{
			BaseURL: cfg.Providers.Ollama.BaseURL,
			Model:   cfg.Providers.Ollama.Model,
		})
		if err := llmManager.RegisterProvider("ollama", ollamaProvider); err != nil {
			return fmt.Errorf("failed to register Ollama provider: %w", err)
		}

		// Set as default if specified
		if cfg.LLM.DefaultProvider == "ollama" {
			if err := llmManager.SetDefaultProvider("ollama"); err != nil {
				return fmt.Errorf("failed to set default provider: %w", err)
			}
		}
	}

	// Create session manager
	sessionManager := session.NewManager(llmManager, cfg)

	// Start or resume session
	var sess *session.Session
	if flags.SessionID != "" {
		// Resume existing session
		sess, err = sessionManager.LoadSession(ctx, flags.SessionID)
		if err != nil {
			return fmt.Errorf("failed to load session %s: %w", flags.SessionID, err)
		}
		fmt.Printf("Resumed session: %s\n", sess.ID)
	} else {
		// Check for existing sessions in current directory
		cwd, err := os.Getwd()
		if err == nil {
			existingSessions, err := sessionManager.FindSessionsByRootPath(cwd)
			if err == nil && len(existingSessions) > 0 {
				// Found existing sessions - prompt user
				fmt.Printf("Found %d existing session(s) for this project:\n", len(existingSessions))
				for i, session := range existingSessions {
					fmt.Printf("  %d. %s (updated: %s)\n", i+1, session.ID, session.UpdatedAt.Format("2006-01-02 15:04:05"))
				}
				fmt.Print("Resume most recent session? [Y/n]: ")

				var response string
				_, _ = fmt.Scanln(&response)

				if response == "" || strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
					// Resume most recent session
					sess, err = sessionManager.LoadSession(ctx, existingSessions[0].ID)
					if err != nil {
						return fmt.Errorf("failed to load session %s: %w", existingSessions[0].ID, err)
					}
					fmt.Printf("Resumed session: %s\n", sess.ID)
				} else {
					// Create new session
					sess = nil
				}
			}
		}

		// Create new session if we didn't resume one
		if sess == nil {
			sessionOpts := &session.CreateOptions{
				Files:           files,
				AutoDetectFiles: len(files) == 0, // Auto-detect only if no files specified
			}

			sess, err = sessionManager.CreateSession(ctx, sessionOpts)
			if err != nil {
				return fmt.Errorf("failed to create session: %w", err)
			}
			fmt.Printf("Created new session: %s\n", sess.ID)
		}
	}

	// Add files if provided
	if len(files) > 0 {
		for _, file := range files {
			if err := sess.AddFile(ctx, file); err != nil {
				fmt.Printf("Warning: failed to add file %s: %v\n", file, err)
			}
		}
		fmt.Printf("Added %d files to session\n", len(files))
	}

	// Start interactive mode with enhanced UI
	return startTUI(ctx, sess, sessionManager, flags)
}

// startEnhancedUI starts the Bubble Tea interface
func startTUI(_ context.Context, sess *session.Session, sessionManager *session.Manager, flags *GlobalFlags) error {
	var model tea.Model = ui.NewModel(sess, sessionManager)

	// Configure Bubble Tea program
	program := tea.NewProgram(
		model,
		tea.WithAltScreen(), // Use alternate screen buffer
		// Mouse support disabled to allow text selection
	)

	// Run the program
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	return nil
}
