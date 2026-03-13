package ai

import (
	"context"
	"errors"
	"io"

	"github.com/pibot/pibot/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

// QwenProvider implements the Provider interface for Qwen (via DashScope or OpenAI-compatible API)
type QwenProvider struct {
	config *config.Config
	client *openai.Client
}

// NewQwenProvider creates a new Qwen provider
func NewQwenProvider(cfg *config.Config) *QwenProvider {
	aiCfg := cfg.GetAI()
	var client *openai.Client
	if aiCfg.Qwen.APIKey != "" {
		// Qwen uses DashScope API which is OpenAI-compatible
		clientConfig := openai.DefaultConfig(aiCfg.Qwen.APIKey)
		clientConfig.BaseURL = aiCfg.Qwen.BaseURL
		client = openai.NewClientWithConfig(clientConfig)
	}
	return &QwenProvider{
		config: cfg,
		client: client,
	}
}

// Name returns the provider name
func (p *QwenProvider) Name() string {
	return "qwen"
}

// Chat sends messages and returns a complete response
func (p *QwenProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	if p.client == nil {
		return "", ErrProviderNotConfigured
	}

	aiCfg := p.config.GetAI()
	openaiMessages := convertToOpenAIMessages(messages)

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    aiCfg.Qwen.Model,
		Messages: openaiMessages,
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("no response from Qwen")
	}

	return resp.Choices[0].Message.Content, nil
}

// StreamChat sends messages and streams the response
func (p *QwenProvider) StreamChat(ctx context.Context, messages []Message, ch chan<- string) error {
	if p.client == nil {
		return ErrProviderNotConfigured
	}

	defer close(ch)

	aiCfg := p.config.GetAI()
	openaiMessages := convertToOpenAIMessages(messages)

	stream, err := p.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    aiCfg.Qwen.Model,
		Messages: openaiMessages,
		Stream:   true,
	})
	if err != nil {
		return err
	}
	defer stream.Close()

	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		if len(response.Choices) > 0 {
			ch <- response.Choices[0].Delta.Content
		}
	}
}

// RefreshClient updates the client with new credentials
func (p *QwenProvider) RefreshClient() {
	aiCfg := p.config.GetAI()
	if aiCfg.Qwen.APIKey != "" {
		clientConfig := openai.DefaultConfig(aiCfg.Qwen.APIKey)
		clientConfig.BaseURL = aiCfg.Qwen.BaseURL
		p.client = openai.NewClientWithConfig(clientConfig)
	}
}
