package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.LLM.DefaultProvider != "bedrock" {
		t.Errorf("Expected default provider 'bedrock', got '%s'", cfg.LLM.DefaultProvider)
	}

	if cfg.LLM.DefaultModel != "eu.anthropic.claude-3-7-sonnet-20250219-v1:0" {
		t.Errorf("Expected default model 'eu.anthropic.claude-3-7-sonnet-20250219-v1:0', got '%s'", cfg.LLM.DefaultModel)
	}

	if !cfg.Providers.Bedrock.Enabled {
		t.Error("Expected Bedrock to be enabled by default")
	}

	if cfg.Providers.Bedrock.Region != "eu-west-1" {
		t.Errorf("Expected default region 'eu-west-1', got '%s'", cfg.Providers.Bedrock.Region)
	}

	if cfg.Security.Terminator != false {
		t.Error("Expected terminator mode to be disabled by default for safety")
	}
}

func TestLoad_EnvironmentVariables(t *testing.T) {
	// Set test environment variables
	_ = os.Setenv("AWS_REGION", "us-west-2")
	_ = os.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	_ = os.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	_ = os.Setenv("OPENAI_API_KEY", "test-openai-key")
	defer func() {
		_ = os.Unsetenv("AWS_REGION")
		_ = os.Unsetenv("AWS_ACCESS_KEY_ID")
		_ = os.Unsetenv("AWS_SECRET_ACCESS_KEY")
		_ = os.Unsetenv("OPENAI_API_KEY")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Providers.Bedrock.Region != "us-west-2" {
		t.Errorf("Expected region 'us-west-2', got '%s'", cfg.Providers.Bedrock.Region)
	}

	if cfg.Providers.Bedrock.AccessKeyID != "test-key" {
		t.Errorf("Expected access key 'test-key', got '%s'", cfg.Providers.Bedrock.AccessKeyID)
	}

	if cfg.Providers.Bedrock.SecretAccessKey != "test-secret" {
		t.Errorf("Expected secret key 'test-secret', got '%s'", cfg.Providers.Bedrock.SecretAccessKey)
	}

	if cfg.Providers.OpenAI.APIKey != "test-openai-key" {
		t.Errorf("Expected OpenAI key 'test-openai-key', got '%s'", cfg.Providers.OpenAI.APIKey)
	}

	if !cfg.Providers.OpenAI.Enabled {
		t.Error("Expected OpenAI to be enabled when API key is set")
	}
}

func TestGetConfigDir(t *testing.T) {
	dir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir failed: %v", err)
	}

	if dir == "" {
		t.Error("Expected non-empty config directory")
	}

	// Should end with .bazinga
	if len(dir) < 8 || dir[len(dir)-8:] != ".bazinga" {
		t.Errorf("Expected config dir to end with '.bazinga', got '%s'", dir)
	}
}
