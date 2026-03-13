package ai

import (
	"context"
	"errors"
	"io"

	"github.com/pibot/pibot/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	config *config.Config
	client *openai.Client
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(cfg *config.Config) *OpenAIProvider {
	aiCfg := cfg.GetAI()
	var client *openai.Client
	if aiCfg.OpenAI.APIKey != "" {
		client = openai.NewClient(aiCfg.OpenAI.APIKey)
	}
	return &OpenAIProvider{
		config: cfg,
		client: client,
	}
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Chat sends messages and returns a complete response
func (p *OpenAIProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	if p.client == nil {
		return "", ErrProviderNotConfigured
	}

	aiCfg := p.config.GetAI()
	openaiMessages := convertToOpenAIMessages(messages)

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    aiCfg.OpenAI.Model,
		Messages: openaiMessages,
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}

// StreamChat sends messages and streams the response
func (p *OpenAIProvider) StreamChat(ctx context.Context, messages []Message, ch chan<- string) error {
	if p.client == nil {
		return ErrProviderNotConfigured
	}

	defer close(ch)

	aiCfg := p.config.GetAI()
	openaiMessages := convertToOpenAIMessages(messages)

	stream, err := p.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    aiCfg.OpenAI.Model,
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

// convertToOpenAIMessages converts internal messages to OpenAI format
func convertToOpenAIMessages(messages []Message) []openai.ChatCompletionMessage {
	result := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		result[i] = openai.ChatCompletionMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}
	return result
}

// RefreshClient updates the client with new credentials
func (p *OpenAIProvider) RefreshClient() {
	aiCfg := p.config.GetAI()
	if aiCfg.OpenAI.APIKey != "" {
		p.client = openai.NewClient(aiCfg.OpenAI.APIKey)
	}
}
