package ai

import (
	"context"

	"github.com/liushuangls/go-anthropic/v2"
	"github.com/pibot/pibot/internal/config"
)

// AnthropicProvider implements the Provider interface for Anthropic.
type AnthropicProvider struct {
	config *config.Config
	client *anthropic.Client
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(cfg *config.Config) *AnthropicProvider {
	aiCfg := cfg.GetAI()
	p := &AnthropicProvider{config: cfg}
	if aiCfg.Anthropic.APIKey != "" {
		p.client = anthropic.NewClient(aiCfg.Anthropic.APIKey)
	}
	return p
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// Chat sends messages and returns a complete response.
func (p *AnthropicProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	if p.client == nil {
		return "", ErrProviderNotConfigured
	}

	aiCfg := p.config.GetAI()

	resp, err := p.client.CreateMessages(ctx, anthropic.MessagesRequest{
		Model:     anthropic.Model(aiCfg.Anthropic.Model),
		MaxTokens: 4096,
		Messages:  convertToAnthropicMessages(messages),
	})
	if err != nil {
		return "", err
	}

	var result string
	for _, block := range resp.Content {
		if block.Type == anthropic.MessagesContentTypeText {
			result += block.GetText()
		}
	}
	return result, nil
}

// StreamChat sends messages and streams the response.
func (p *AnthropicProvider) StreamChat(ctx context.Context, messages []Message, ch chan<- string) error {
	if p.client == nil {
		return ErrProviderNotConfigured
	}

	defer close(ch)

	aiCfg := p.config.GetAI()

	_, err := p.client.CreateMessagesStream(ctx, anthropic.MessagesStreamRequest{
		MessagesRequest: anthropic.MessagesRequest{
			Model:     anthropic.Model(aiCfg.Anthropic.Model),
			MaxTokens: 4096,
			Messages:  convertToAnthropicMessages(messages),
		},
		OnContentBlockDelta: func(data anthropic.MessagesEventContentBlockDeltaData) {
			if data.Delta.Type == anthropic.MessagesContentTypeTextDelta && data.Delta.Text != nil {
				ch <- *data.Delta.Text
			}
		},
	})
	return err
}

// RefreshClient updates the client when credentials change.
func (p *AnthropicProvider) RefreshClient() {
	aiCfg := p.config.GetAI()
	if aiCfg.Anthropic.APIKey != "" {
		p.client = anthropic.NewClient(aiCfg.Anthropic.APIKey)
	}
}

// convertToAnthropicMessages converts internal messages to Anthropic format.
// System messages are skipped — pass them via MessagesRequest.System instead.
func convertToAnthropicMessages(messages []Message) []anthropic.Message {
	result := make([]anthropic.Message, 0, len(messages))
	for _, msg := range messages {
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
