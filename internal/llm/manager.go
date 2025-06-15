package llm

import (
	"context"
	"fmt"
	"sync"
)

// Manager manages multiple LLM providers
type Manager struct {
	providers       map[string]Provider
	defaultProvider string
	mu              sync.RWMutex
}

// NewManager creates a new LLM manager
func NewManager() *Manager {
	return &Manager{
		providers: make(map[string]Provider),
	}
}

// RegisterProvider registers a new LLM provider
func (m *Manager) RegisterProvider(name string, provider Provider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	m.providers[name] = provider

	// Set as default if it's the first provider
	if m.defaultProvider == "" {
		m.defaultProvider = name
	}

	return nil
}

// SetDefaultProvider sets the default provider
func (m *Manager) SetDefaultProvider(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.providers[name]; !exists {
		return fmt.Errorf("provider %s not found", name)
	}

	m.defaultProvider = name
	return nil
}

// GetProvider returns a provider by name
func (m *Manager) GetProvider(name string) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if name == "" {
		name = m.defaultProvider
	}

	provider, exists := m.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return provider, nil
}

// GetDefaultProvider returns the default provider
func (m *Manager) GetDefaultProvider() (Provider, error) {
	return m.GetProvider("")
}

// ListProviders returns all registered provider names
func (m *Manager) ListProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}

// GenerateResponse generates a response using the specified or default provider
func (m *Manager) GenerateResponse(ctx context.Context, req *GenerateRequest, providerName string) (*Response, error) {
	provider, err := m.GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	return provider.GenerateResponse(ctx, req)
}

// StreamResponse streams a response using the specified or default provider
func (m *Manager) StreamResponse(ctx context.Context, req *GenerateRequest, providerName string) (<-chan *StreamChunk, error) {
	provider, err := m.GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	return provider.StreamResponse(ctx, req)
}

// GetAvailableModels returns all available models from all providers
func (m *Manager) GetAvailableModels() map[string][]Model {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]Model)
	for name, provider := range m.providers {
		result[name] = provider.GetAvailableModels()
	}
	return result
}

// Close closes all providers
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for name, provider := range m.providers {
		if err := provider.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close provider %s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing providers: %v", errs)
	}

	return nil
}
