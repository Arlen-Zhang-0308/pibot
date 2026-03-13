package ai

import (
	"context"
	"testing"

	"github.com/pibot/pibot/internal/config"
)

func TestManager_RegisterAndGetProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	mgr := NewManager(cfg)

	// Register providers
	mgr.RegisterProvider(NewOpenAIProvider(cfg))
	mgr.RegisterProvider(NewAnthropicProvider(cfg))
	mgr.RegisterProvider(NewGoogleProvider(cfg))
	mgr.RegisterProvider(NewOllamaProvider(cfg))

	// Test ListProviders
	providers := mgr.ListProviders()
	if len(providers) != 4 {
		t.Errorf("Expected 4 providers, got %d", len(providers))
	}

	// Test GetProvider
	openai, err := mgr.GetProvider("openai")
	if err != nil {
		t.Fatalf("Failed to get openai provider: %v", err)
	}
	if openai.Name() != "openai" {
		t.Errorf("Expected name 'openai', got '%s'", openai.Name())
	}

	anthropic, err := mgr.GetProvider("anthropic")
	if err != nil {
		t.Fatalf("Failed to get anthropic provider: %v", err)
	}
	if anthropic.Name() != "anthropic" {
		t.Errorf("Expected name 'anthropic', got '%s'", anthropic.Name())
	}

	// Test non-existent provider
	_, err = mgr.GetProvider("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent provider")
	}
}

func TestManager_GetDefaultProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AI.DefaultProvider = "ollama"

	mgr := NewManager(cfg)
	mgr.RegisterProvider(NewOllamaProvider(cfg))

	provider, err := mgr.GetDefaultProvider()
	if err != nil {
		t.Fatalf("Failed to get default provider: %v", err)
	}

	if provider.Name() != "ollama" {
		t.Errorf("Expected default provider 'ollama', got '%s'", provider.Name())
	}
}

func TestManager_DefaultProviderNotRegistered(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AI.DefaultProvider = "nonexistent"

	mgr := NewManager(cfg)

	_, err := mgr.GetDefaultProvider()
	if err == nil {
		t.Error("Expected error when default provider not registered")
	}
}

func TestOpenAIProvider_Name(t *testing.T) {
	cfg := config.DefaultConfig()
	p := NewOpenAIProvider(cfg)

	if p.Name() != "openai" {
		t.Errorf("Expected name 'openai', got '%s'", p.Name())
	}
}

func TestOpenAIProvider_ChatWithoutAPIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AI.OpenAI.APIKey = "" // No API key

	p := NewOpenAIProvider(cfg)

	_, err := p.Chat(context.Background(), []Message{{Role: RoleUser, Content: "test"}})
	if err != ErrProviderNotConfigured {
		t.Errorf("Expected ErrProviderNotConfigured, got %v", err)
	}
}

func TestAnthropicProvider_Name(t *testing.T) {
	cfg := config.DefaultConfig()
	p := NewAnthropicProvider(cfg)

	if p.Name() != "anthropic" {
		t.Errorf("Expected name 'anthropic', got '%s'", p.Name())
	}
}

func TestAnthropicProvider_ChatWithoutAPIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AI.Anthropic.APIKey = ""

	p := NewAnthropicProvider(cfg)

	_, err := p.Chat(context.Background(), []Message{{Role: RoleUser, Content: "test"}})
	if err != ErrProviderNotConfigured {
		t.Errorf("Expected ErrProviderNotConfigured, got %v", err)
	}
}

func TestGoogleProvider_Name(t *testing.T) {
	cfg := config.DefaultConfig()
	p := NewGoogleProvider(cfg)

	if p.Name() != "google" {
		t.Errorf("Expected name 'google', got '%s'", p.Name())
	}
}

func TestGoogleProvider_ChatWithoutAPIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AI.Google.APIKey = ""

	p := NewGoogleProvider(cfg)

	_, err := p.Chat(context.Background(), []Message{{Role: RoleUser, Content: "test"}})
	if err != ErrProviderNotConfigured {
		t.Errorf("Expected ErrProviderNotConfigured, got %v", err)
	}
}

func TestOllamaProvider_Name(t *testing.T) {
	cfg := config.DefaultConfig()
	p := NewOllamaProvider(cfg)

	if p.Name() != "ollama" {
		t.Errorf("Expected name 'ollama', got '%s'", p.Name())
	}
}

func TestQwenProvider_Name(t *testing.T) {
	cfg := config.DefaultConfig()
	p := NewQwenProvider(cfg)

	if p.Name() != "qwen" {
		t.Errorf("Expected name 'qwen', got '%s'", p.Name())
	}
}

func TestQwenProvider_ChatWithoutAPIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AI.Qwen.APIKey = ""

	p := NewQwenProvider(cfg)

	_, err := p.Chat(context.Background(), []Message{{Role: RoleUser, Content: "test"}})
	if err != ErrProviderNotConfigured {
		t.Errorf("Expected ErrProviderNotConfigured, got %v", err)
	}
}

func TestMessage_Roles(t *testing.T) {
	if RoleUser != "user" {
		t.Errorf("Expected RoleUser to be 'user'")
	}
	if RoleAssistant != "assistant" {
		t.Errorf("Expected RoleAssistant to be 'assistant'")
	}
	if RoleSystem != "system" {
		t.Errorf("Expected RoleSystem to be 'system'")
	}
}

func TestConvertToOpenAIMessages(t *testing.T) {
	messages := []Message{
		{Role: RoleSystem, Content: "You are a helpful assistant"},
		{Role: RoleUser, Content: "Hello"},
		{Role: RoleAssistant, Content: "Hi there!"},
	}

	converted := convertToOpenAIMessages(messages)

	if len(converted) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(converted))
	}

	if converted[0].Role != "system" {
		t.Errorf("Expected first role 'system', got '%s'", converted[0].Role)
	}
	if converted[1].Content != "Hello" {
		t.Errorf("Expected second content 'Hello', got '%s'", converted[1].Content)
	}
}

func TestConvertToAnthropicMessages(t *testing.T) {
	messages := []Message{
		{Role: RoleSystem, Content: "System message"},
		{Role: RoleUser, Content: "User message"},
		{Role: RoleAssistant, Content: "Assistant message"},
	}

	converted := convertToAnthropicMessages(messages)

	// System messages should be filtered out
	if len(converted) != 2 {
		t.Errorf("Expected 2 messages (system filtered), got %d", len(converted))
	}
}
