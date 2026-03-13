package ai

import (
	"context"

	"github.com/liushuangls/go-anthropic/v2"
	"github.com/pibot/pibot/internal/config"
)

// AnthropicProvider implements the Provider interface for Anthropic
type AnthropicProvider struct {
	config *config.Config
	client *anthropic.Client
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(cfg *config.Config) *AnthropicProvider {
	aiCfg := cfg.GetAI()
	var client *anthropic.Client
	if aiCfg.Anthropic.APIKey != "" {
		client = anthropic.NewClient(aiCfg.Anthropic.APIKey)
	}
	return &AnthropicProvider{
		config: cfg,
		client: client,
	}
}

// Name returns the provider name
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// Chat sends messages and returns a complete response
func (p *AnthropicProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	if p.client == nil {
		return "", ErrProviderNotConfigured
	}

	aiCfg := p.config.GetAI()
	anthropicMessages := convertToAnthropicMessages(messages)

	resp, err := p.client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model:     anthropic.Model(aiCfg.Anthropic.Model),
		MaxTokens: 4096,
		Messages:  anthropicMessages,
	})
	if err != nil {
		return "", err
	}

	// Extract text from response
	var result string
	for _, block := range resp.Content {
		if block.Type == anthropic.MessagesContentTypeText {
			result += block.GetText()
		}
	}

	return result, nil
}

// StreamChat sends messages and streams the response
func (p *AnthropicProvider) StreamChat(ctx context.Context, messages []Message, ch chan<- string) error {
	if p.client == nil {
		return ErrProviderNotConfigured
	}

	defer close(ch)

	aiCfg := p.config.GetAI()
	anthropicMessages := convertToAnthropicMessages(messages)

	_, err := p.client.CreateMessagesStream(ctx, anthropic.MessagesStreamRequest{
		MessagesRequest: anthropic.MessagesRequest{
			Model:     anthropic.Model(aiCfg.Anthropic.Model),
			MaxTokens: 4096,
			Messages:  anthropicMessages,
		},
		OnContentBlockDelta: func(data anthropic.MessagesEventContentBlockDeltaData) {
			if data.Delta.Type == anthropic.MessagesContentTypeTextDelta && data.Delta.Text != nil {
				ch <- *data.Delta.Text
			}
		},
	})

	return err
}

// convertToAnthropicMessages converts internal messages to Anthropic format
func convertToAnthropicMessages(messages []Message) []anthropic.Message {
	result := make([]anthropic.Message, 0, len(messages))
	for _, msg := range messages {
		// Skip system messages as they're handled separately in Anthropic
		if msg.Role == RoleSystem {
			continue
		}
		role := anthropic.RoleUser
		if msg.Role == RoleAssistant {
			role = anthropic.RoleAssistant
		}
		result = append(result, anthropic.Message{
			Role: role,
			Content: []anthropic.MessageContent{
				anthropic.NewTextMessageContent(msg.Content),
			},
		})
	}
	return result
}

// RefreshClient updates the client with new credentials
func (p *AnthropicProvider) RefreshClient() {
	aiCfg := p.config.GetAI()
	if aiCfg.Anthropic.APIKey != "" {
		p.client = anthropic.NewClient(aiCfg.Anthropic.APIKey)
	}
}
