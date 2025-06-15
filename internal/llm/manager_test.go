package llm

import (
	"context"
	"testing"
)

// mockProvider implements Provider interface for testing
type mockProvider struct {
	name         string
	models       []Model
	supportsFunc bool
}

func (m *mockProvider) Name() string                   { return m.name }
func (m *mockProvider) GetAvailableModels() []Model    { return m.models }
func (m *mockProvider) GetDefaultModel() string        { return "test-model" }
func (m *mockProvider) SupportsFunctionCalling() bool  { return m.supportsFunc }
func (m *mockProvider) EstimateTokens(text string) int { return len(text) / 4 }
func (m *mockProvider) GetTokenLimit() int             { return 4096 }
func (m *mockProvider) Close() error                   { return nil }
func (m *mockProvider) GenerateResponse(ctx context.Context, req *GenerateRequest) (*Response, error) {
	return &Response{Content: "mock response"}, nil
}

func (m *mockProvider) StreamResponse(ctx context.Context, req *GenerateRequest) (<-chan *StreamChunk, error) {
	ch := make(chan *StreamChunk, 1)
	ch <- &StreamChunk{Content: "mock stream"}
	close(ch)
	return ch, nil
}

func TestManager_RegisterProvider(t *testing.T) {
	manager := NewManager()
	provider := &mockProvider{name: "test"}

	err := manager.RegisterProvider("test", provider)
	if err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	// Test duplicate registration
	err = manager.RegisterProvider("test", provider)
	if err == nil {
		t.Fatal("Expected error for duplicate provider registration")
	}
}

func TestManager_GetProvider(t *testing.T) {
	manager := NewManager()
	provider := &mockProvider{name: "test"}

	// Test getting non-existent provider
	_, err := manager.GetProvider("nonexistent")
	if err == nil {
		t.Fatal("Expected error for non-existent provider")
	}

	// Register and get provider
	err = manager.RegisterProvider("test", provider)
	if err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	retrieved, err := manager.GetProvider("test")
	if err != nil {
		t.Fatalf("GetProvider failed: %v", err)
	}

	if retrieved.Name() != "test" {
		t.Errorf("Expected provider name 'test', got '%s'", retrieved.Name())
	}
}

func TestManager_SetDefaultProvider(t *testing.T) {
	manager := NewManager()
	provider := &mockProvider{name: "test"}

	// Test setting non-existent provider as default
	err := manager.SetDefaultProvider("nonexistent")
	if err == nil {
		t.Fatal("Expected error for non-existent provider")
	}

	// Register provider and set as default
	err = manager.RegisterProvider("test", provider)
	if err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	err = manager.SetDefaultProvider("test")
	if err != nil {
		t.Fatalf("SetDefaultProvider failed: %v", err)
	}

	// Test getting default provider
	defaultProvider, err := manager.GetDefaultProvider()
	if err != nil {
		t.Fatalf("GetDefaultProvider failed: %v", err)
	}

	if defaultProvider.Name() != "test" {
		t.Errorf("Expected default provider name 'test', got '%s'", defaultProvider.Name())
	}
}

func TestManager_GenerateResponse(t *testing.T) {
	manager := NewManager()
	provider := &mockProvider{name: "test"}

	err := manager.RegisterProvider("test", provider)
	if err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	req := &GenerateRequest{
		Messages: []Message{{Role: "user", Content: "test"}},
	}

	response, err := manager.GenerateResponse(context.Background(), req, "test")
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	if response.Content != "mock response" {
		t.Errorf("Expected 'mock response', got '%s'", response.Content)
	}
}

func TestManager_ListProviders(t *testing.T) {
	manager := NewManager()
	provider1 := &mockProvider{name: "test1"}
	provider2 := &mockProvider{name: "test2"}

	err := manager.RegisterProvider("test1", provider1)
	if err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	err = manager.RegisterProvider("test2", provider2)
	if err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	providers := manager.ListProviders()
	if len(providers) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(providers))
	}

	// Check that both providers are listed
	found := make(map[string]bool)
	for _, name := range providers {
		found[name] = true
	}

	if !found["test1"] || !found["test2"] {
		t.Error("Not all registered providers were listed")
	}
}
