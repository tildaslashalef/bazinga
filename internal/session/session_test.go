package session

import (
	"context"
	"github.com/tildaslashalef/bazinga/internal/config"
	"github.com/tildaslashalef/bazinga/internal/llm"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider implements Provider interface for testing
type mockProvider struct {
	name         string
	models       []llm.Model
	supportsFunc bool
}

func (m *mockProvider) Name() string                    { return m.name }
func (m *mockProvider) GetAvailableModels() []llm.Model { return m.models }
func (m *mockProvider) GetDefaultModel() string         { return "test-model" }
func (m *mockProvider) SupportsFunctionCalling() bool   { return m.supportsFunc }
func (m *mockProvider) EstimateTokens(text string) int  { return len(text) / 4 }
func (m *mockProvider) GetTokenLimit() int              { return 4096 }
func (m *mockProvider) Close() error                    { return nil }

func (m *mockProvider) GenerateResponse(ctx context.Context, req *llm.GenerateRequest) (*llm.Response, error) {
	return &llm.Response{Content: "mock response"}, nil
}

func (m *mockProvider) StreamResponse(ctx context.Context, req *llm.GenerateRequest) (<-chan *llm.StreamChunk, error) {
	ch := make(chan *llm.StreamChunk, 1)
	ch <- &llm.StreamChunk{Content: "mock stream"}
	close(ch)
	return ch, nil
}

func newMockLLMManager() *llm.Manager {
	mgr := llm.NewManager()

	// Add mock providers
	provider1 := &mockProvider{name: "openai"}
	provider2 := &mockProvider{name: "anthropic"}

	_ = mgr.RegisterProvider("openai", provider1)
	_ = mgr.RegisterProvider("anthropic", provider2)

	return mgr
}

// Setup helper to create a test session manager
func setupTestSessionManager() (*Manager, *llm.Manager) {
	llmManager := newMockLLMManager()

	cfg := &config.Config{
		LLM: config.LLMConfig{
			DefaultProvider: "openai",
			DefaultModel:    "gpt-4",
			MaxTokens:       4000,
		},
	}

	return NewManager(llmManager, cfg), llmManager
}

// TestCreateSession tests the creation of a new session
func TestCreateSession(t *testing.T) {
	manager, _ := setupTestSessionManager()

	ctx := context.Background()
	opts := &CreateOptions{
		Name:  "Test Session",
		Tags:  []string{"test", "unittest"},
		Files: []string{},
	}

	session, err := manager.CreateSession(ctx, opts)
	require.NoError(t, err, "CreateSession should not return an error")
	require.NotNil(t, session, "Session should not be nil")

	// Verify session attributes
	assert.Equal(t, "Test Session", session.Name)
	assert.ElementsMatch(t, []string{"test", "unittest"}, session.Tags)
	assert.Equal(t, "openai", session.Provider)
	assert.Equal(t, "gpt-4", session.Model)
	assert.NotEmpty(t, session.ID)
	assert.False(t, session.CreatedAt.IsZero())
	assert.False(t, session.UpdatedAt.IsZero())
}

// TestSetProvider tests setting the provider for a session
func TestSetProvider(t *testing.T) {
	manager, _ := setupTestSessionManager()

	ctx := context.Background()
	opts := &CreateOptions{
		Name: "Test Provider Session",
	}

	session, err := manager.CreateSession(ctx, opts)
	require.NoError(t, err)

	// Test setting a valid provider
	err = session.SetProvider("anthropic")
	assert.NoError(t, err)
	assert.Equal(t, "anthropic", session.Provider)

	// Test setting an empty provider
	err = session.SetProvider("")
	assert.Error(t, err)
	assert.Equal(t, "anthropic", session.Provider, "Provider should not change on error")

	// Test setting an invalid provider
	err = session.SetProvider("invalid")
	assert.Error(t, err)
	assert.Equal(t, "anthropic", session.Provider, "Provider should not change on error")
}

// TestSetModel tests setting the model for a session
func TestSetModel(t *testing.T) {
	manager, _ := setupTestSessionManager()

	ctx := context.Background()
	opts := &CreateOptions{
		Name: "Test Model Session",
	}

	session, err := manager.CreateSession(ctx, opts)
	require.NoError(t, err)

	// Test setting a model
	initialUpdatedAt := session.UpdatedAt
	time.Sleep(1 * time.Millisecond) // Ensure timestamp changes

	err = session.SetModel("claude-3-opus-20240229")
	assert.NoError(t, err)
	assert.Equal(t, "claude-3-opus-20240229", session.Model)
	assert.True(t, session.UpdatedAt.After(initialUpdatedAt), "UpdatedAt should be updated")
}

// TestAddSystemMessage tests adding system messages to a session
func TestAddSystemMessage(t *testing.T) {
	manager, _ := setupTestSessionManager()

	ctx := context.Background()
	session, err := manager.CreateSession(ctx, &CreateOptions{Name: "Test Message Session"})
	require.NoError(t, err)

	// Add system message
	err = session.AddSystemMessage("Test system message")
	assert.NoError(t, err)

	// Verify message was added
	assert.Len(t, session.History, 1)
	assert.Equal(t, "system", session.History[0].Role)
	assert.Equal(t, "Test system message", session.History[0].Content)
}

// TestGetAvailableProvidersAndModels tests getting available providers and models
func TestGetAvailableProvidersAndModels(t *testing.T) {
	manager, _ := setupTestSessionManager()

	ctx := context.Background()
	session, err := manager.CreateSession(ctx, &CreateOptions{Name: "Test Providers Session"})
	require.NoError(t, err)

	// Test getting providers
	providers := session.GetAvailableProviders()
	assert.Len(t, providers, 2)
	assert.Contains(t, providers, "openai")
	assert.Contains(t, providers, "anthropic")

	// Test getting models
	models := session.GetAvailableModels()
	assert.NotNil(t, models)
}

// TestTerminatorMode tests the terminator mode functionality
func TestTerminatorMode(t *testing.T) {
	manager, _ := setupTestSessionManager()

	ctx := context.Background()
	session, err := manager.CreateSession(ctx, &CreateOptions{Name: "Terminator Session"})
	require.NoError(t, err)

	// Default should be false
	assert.False(t, session.IsTerminatorMode())

	// Set terminator mode to true
	session.config.Security.Terminator = true
	assert.True(t, session.IsTerminatorMode())
}
