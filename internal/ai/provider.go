package ai

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/pibot/pibot/internal/config"
)

// Role represents the role of a message sender
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// Message represents a chat message
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// Provider defines the interface for AI providers.
type Provider interface {
	// Name returns the provider name.
	Name() string
	// Chat sends messages and returns a complete response.
	Chat(ctx context.Context, messages []Message) (string, error)
	// StreamChat sends messages and streams the response token by token.
	StreamChat(ctx context.Context, messages []Message, ch chan<- string) error
}

// Closer is an optional interface implemented by providers that hold
// long-lived resources (e.g. network connections) that must be released.
type Closer interface {
	Close() error
}

// Manager manages multiple AI providers
type Manager struct {
	providers map[string]Provider
	config    *config.Config
	mu        sync.RWMutex
}

// NewManager creates a new AI provider manager
func NewManager(cfg *config.Config) *Manager {
	m := &Manager{
		providers: make(map[string]Provider),
		config:    cfg,
	}
	return m
}

// RegisterProvider adds a provider to the manager
func (m *Manager) RegisterProvider(p Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[p.Name()] = p
}

// GetProvider returns a specific provider by name
func (m *Manager) GetProvider(name string) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	p, ok := m.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", name)
	}
	return p, nil
}

// GetDefaultProvider returns the configured default provider
func (m *Manager) GetDefaultProvider() (Provider, error) {
	aiCfg := m.config.GetAI()
	return m.GetProvider(aiCfg.DefaultProvider)
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

// Chat sends a message using the default provider
func (m *Manager) Chat(ctx context.Context, messages []Message) (string, error) {
	p, err := m.GetDefaultProvider()
	if err != nil {
		return "", err
	}
	return p.Chat(ctx, messages)
}

// StreamChat streams a response using the default provider
func (m *Manager) StreamChat(ctx context.Context, messages []Message, ch chan<- string) error {
	p, err := m.GetDefaultProvider()
	if err != nil {
		return err
	}
	return p.StreamChat(ctx, messages, ch)
}

// ChatWithProvider sends a message using a specific provider
func (m *Manager) ChatWithProvider(ctx context.Context, providerName string, messages []Message) (string, error) {
	p, err := m.GetProvider(providerName)
	if err != nil {
		return "", err
	}
	return p.Chat(ctx, messages)
}

// StreamChatWithProvider streams a response using a specific provider
func (m *Manager) StreamChatWithProvider(ctx context.Context, providerName string, messages []Message, ch chan<- string) error {
	p, err := m.GetProvider(providerName)
	if err != nil {
		return err
	}
	return p.StreamChat(ctx, messages, ch)
}

// ErrProviderNotConfigured indicates a provider lacks required configuration
var ErrProviderNotConfigured = errors.New("provider not configured (missing API key)")
