package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	LLM       LLMConfig       `yaml:"llm"`
	Providers ProvidersConfig `yaml:"providers"`
	Git       GitConfig       `yaml:"git"`
	Security  SecurityConfig  `yaml:"security"`
	Logging   LoggingConfig   `yaml:"logging"`
}

// LLMConfig contains LLM-related configuration
type LLMConfig struct {
	DefaultProvider string  `yaml:"default_provider"`
	DefaultModel    string  `yaml:"default_model"`
	MaxTokens       int     `yaml:"max_tokens"`
	Temperature     float64 `yaml:"temperature"`
}

// ProvidersConfig contains provider-specific configurations
type ProvidersConfig struct {
	Bedrock   BedrockConfig   `yaml:"bedrock"`
	OpenAI    OpenAIConfig    `yaml:"openai"`
	Anthropic AnthropicConfig `yaml:"anthropic"`
	Ollama    OllamaConfig    `yaml:"ollama"`
}

// BedrockConfig contains AWS Bedrock configuration
type BedrockConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Region          string `yaml:"region"`
	AuthMethod      string `yaml:"auth_method"`   // "default", "static", "profile", "assume_role"
	Profile         string `yaml:"profile"`       // AWS profile name
	AccessKeyID     string `yaml:"access_key_id"` // Static credentials
	SecretAccessKey string `yaml:"secret_access_key"`
	SessionToken    string `yaml:"session_token"` // For temporary credentials
	RoleARN         string `yaml:"role_arn"`      // For assume role
	RoleSessionName string `yaml:"role_session_name"`
	ExternalID      string `yaml:"external_id"` // For assume role with external ID
}

// OpenAIConfig contains OpenAI configuration
type OpenAIConfig struct {
	Enabled bool   `yaml:"enabled"`
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
	OrgID   string `yaml:"org_id"`
}

// AnthropicConfig contains Anthropic configuration
type AnthropicConfig struct {
	Enabled bool   `yaml:"enabled"`
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
}

// OllamaConfig contains Ollama configuration
type OllamaConfig struct {
	Enabled bool   `yaml:"enabled"`
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
}

// GitConfig contains Git-related configuration
type GitConfig struct {
	AuthorName  string `yaml:"author_name"`
	AuthorEmail string `yaml:"author_email"`
}

// SecurityConfig contains security-related configuration
type SecurityConfig struct {
	Terminator bool `yaml:"terminator"` // Bypass all permission checks (DANGEROUS)
}

// LoggingConfig contains logging-related configuration
type LoggingConfig struct {
	Level      string `yaml:"level"`       // debug, info, warn, error
	Format     string `yaml:"format"`      // text, json
	Output     string `yaml:"output"`      // file, console, both
	FilePath   string `yaml:"file_path"`   // custom log file path (optional)
	MaxSize    int    `yaml:"max_size"`    // max file size in MB
	MaxBackups int    `yaml:"max_backups"` // max number of backup files
	MaxAge     int    `yaml:"max_age"`     // max age in days
	AddSource  bool   `yaml:"add_source"`  // include source file/line in logs
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			DefaultProvider: "bedrock",
			DefaultModel:    "eu.anthropic.claude-3-7-sonnet-20250219-v1:0",
			MaxTokens:       4096,
			Temperature:     0.7,
		},
		Providers: ProvidersConfig{
			Bedrock: BedrockConfig{
				Enabled:    true,
				Region:     "eu-west-1",
				AuthMethod: "profile",
				Profile:    "sso-bedrock",
			},
			OpenAI: OpenAIConfig{
				Enabled: false,
			},
			Anthropic: AnthropicConfig{
				Enabled: false,
				BaseURL: "https://api.anthropic.com",
			},
			Ollama: OllamaConfig{
				Enabled: false,
				BaseURL: "http://localhost:11434",
				Model:   "qwen2.5-coder:latest",
			},
		},
		Git: GitConfig{
			AuthorName:  "", // Will fallback to git config
			AuthorEmail: "", // Will fallback to git config
		},
		Security: SecurityConfig{
			Terminator: false, // Default to safe mode
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "text",
			Output:     "file",
			MaxSize:    10,
			MaxBackups: 5,
			MaxAge:     30,
			AddSource:  true,
		},
	}
}

// Load loads the configuration from file and environment
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Bind environment variables
	viper.SetEnvPrefix("BAZINGA")
	viper.AutomaticEnv()

	// First, unmarshal the entire config from viper to override defaults
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Manual extraction of Bedrock config values (for debugging config loading issues)
	if viper.IsSet("providers.bedrock.auth_method") {
		cfg.Providers.Bedrock.AuthMethod = viper.GetString("providers.bedrock.auth_method")
	}
	if viper.IsSet("providers.bedrock.profile") {
		cfg.Providers.Bedrock.Profile = viper.GetString("providers.bedrock.profile")
	}
	if viper.IsSet("providers.bedrock.access_key_id") {
		cfg.Providers.Bedrock.AccessKeyID = viper.GetString("providers.bedrock.access_key_id")
	}
	if viper.IsSet("providers.bedrock.secret_access_key") {
		cfg.Providers.Bedrock.SecretAccessKey = viper.GetString("providers.bedrock.secret_access_key")
	}
	if viper.IsSet("providers.bedrock.session_token") {
		cfg.Providers.Bedrock.SessionToken = viper.GetString("providers.bedrock.session_token")
	}
	if viper.IsSet("providers.bedrock.role_arn") {
		cfg.Providers.Bedrock.RoleARN = viper.GetString("providers.bedrock.role_arn")
	}
	if viper.IsSet("providers.bedrock.role_session_name") {
		cfg.Providers.Bedrock.RoleSessionName = viper.GetString("providers.bedrock.role_session_name")
	}
	if viper.IsSet("providers.bedrock.external_id") {
		cfg.Providers.Bedrock.ExternalID = viper.GetString("providers.bedrock.external_id")
	}

	// Override with viper values (for backward compatibility)
	if viper.IsSet("llm.default_provider") {
		cfg.LLM.DefaultProvider = viper.GetString("llm.default_provider")
	}
	if viper.IsSet("llm.default_model") {
		cfg.LLM.DefaultModel = viper.GetString("llm.default_model")
	}
	if viper.IsSet("providers.bedrock.region") {
		cfg.Providers.Bedrock.Region = viper.GetString("providers.bedrock.region")
	}
	if viper.IsSet("providers.bedrock.enabled") {
		cfg.Providers.Bedrock.Enabled = viper.GetBool("providers.bedrock.enabled")
	}

	// Load AWS credentials from environment (these override config file)
	if awsRegion := os.Getenv("AWS_REGION"); awsRegion != "" {
		cfg.Providers.Bedrock.Region = awsRegion
	}
	if awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID"); awsAccessKey != "" {
		cfg.Providers.Bedrock.AccessKeyID = awsAccessKey
	}
	if awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY"); awsSecretKey != "" {
		cfg.Providers.Bedrock.SecretAccessKey = awsSecretKey
	}
	if awsSessionToken := os.Getenv("AWS_SESSION_TOKEN"); awsSessionToken != "" {
		cfg.Providers.Bedrock.SessionToken = awsSessionToken
	}
	if awsProfile := os.Getenv("AWS_PROFILE"); awsProfile != "" {
		cfg.Providers.Bedrock.Profile = awsProfile
	}

	// Load OpenAI credentials
	if openaiKey := os.Getenv("OPENAI_API_KEY"); openaiKey != "" {
		cfg.Providers.OpenAI.APIKey = openaiKey
		cfg.Providers.OpenAI.Enabled = true
	}

	// Load Anthropic credentials
	if anthropicKey := os.Getenv("ANTHROPIC_API_KEY"); anthropicKey != "" {
		cfg.Providers.Anthropic.APIKey = anthropicKey
		cfg.Providers.Anthropic.Enabled = true
	}

	// Load Ollama configuration
	if ollamaURL := os.Getenv("OLLAMA_BASE_URL"); ollamaURL != "" {
		cfg.Providers.Ollama.BaseURL = ollamaURL
	}
	if ollamaEnabled := os.Getenv("OLLAMA_ENABLED"); ollamaEnabled == "true" {
		cfg.Providers.Ollama.Enabled = true
	}

	return cfg, nil
}

// Init creates a default configuration file
func Init() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".bazinga")
	configFile := filepath.Join(configDir, "config.yaml")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Check if config file already exists
	if _, err := os.Stat(configFile); err == nil {
		fmt.Printf("Configuration file already exists: %s\n", configFile)
		return nil
	}

	// Create default configuration
	cfg := DefaultConfig()

	// Write to file
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Created configuration file: %s\n", configFile)
	fmt.Println("Please edit the file to configure your providers.")

	return nil
}

// GetConfigDir returns the configuration directory path
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".bazinga"), nil
}
