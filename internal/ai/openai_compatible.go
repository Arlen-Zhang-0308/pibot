package ai

import (
	"context"
	"errors"
	"io"

	openai "github.com/sashabaranov/go-openai"
)

// openAICompatibleProvider is a shared base for providers that use the OpenAI-compatible API.
type openAICompatibleProvider struct {
	name   string
	model  string
	client *openai.Client
}

func (p *openAICompatibleProvider) Name() string {
	return p.name
}

func (p *openAICompatibleProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	if p.client == nil {
		return "", ErrProviderNotConfigured
	}

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: convertToOpenAIMessages(messages),
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("no choices in response")
	}

	return resp.Choices[0].Message.Content, nil
}

func (p *openAICompatibleProvider) StreamChat(ctx context.Context, messages []Message, ch chan<- string) error {
	if p.client == nil {
		return ErrProviderNotConfigured
	}

	defer close(ch)

	stream, err := p.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: convertToOpenAIMessages(messages),
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

// convertToOpenAIMessages converts internal messages to OpenAI format.
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
