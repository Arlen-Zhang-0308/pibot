package ai

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/google/generative-ai-go/genai"
	"github.com/pibot/pibot/internal/config"
	"google.golang.org/api/option"
)

// GoogleProvider implements the Provider interface for Google Gemini.
type GoogleProvider struct {
	config *config.Config
	mu     sync.Mutex
	client *genai.Client
}

// NewGoogleProvider creates a new Google Gemini provider.
func NewGoogleProvider(cfg *config.Config) *GoogleProvider {
	return &GoogleProvider{config: cfg}
}

// Name returns the provider name.
func (p *GoogleProvider) Name() string {
	return "google"
}

// getClient returns the Gemini client, initializing it lazily on first use.
// genai.NewClient requires a context, so initialization is deferred to the first call.
func (p *GoogleProvider) getClient(ctx context.Context) (*genai.Client, error) {
	aiCfg := p.config.GetAI()
	if aiCfg.Google.APIKey == "" {
		return nil, ErrProviderNotConfigured
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.client == nil {
		client, err := genai.NewClient(ctx, option.WithAPIKey(aiCfg.Google.APIKey))
		if err != nil {
			return nil, err
		}
		p.client = client
	}
	return p.client, nil
}

// Chat sends messages and returns a complete response.
func (p *GoogleProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	client, err := p.getClient(ctx)
	if err != nil {
		return "", err
	}

	aiCfg := p.config.GetAI()
	model := client.GenerativeModel(aiCfg.Google.Model)

	parts := convertToGeminiParts(messages)
	resp, err := model.GenerateContent(ctx, parts...)
	if err != nil {
		return "", err
	}

	return extractGeminiText(resp.Candidates), nil
}

// StreamChat sends messages and streams the response.
func (p *GoogleProvider) StreamChat(ctx context.Context, messages []Message, ch chan<- string) error {
	client, err := p.getClient(ctx)
	if err != nil {
		return err
	}

	defer close(ch)

	aiCfg := p.config.GetAI()
	model := client.GenerativeModel(aiCfg.Google.Model)

	iter := model.GenerateContentStream(ctx, convertToGeminiParts(messages)...)
	for {
		resp, err := iter.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if text := extractGeminiText(resp.Candidates); text != "" {
			ch <- text
		}
	}
}

// Close releases the underlying Gemini client. Implements the Closer interface.
func (p *GoogleProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil {
		err := p.client.Close()
		p.client = nil
		return err
	}
	return nil
}

func convertToGeminiParts(messages []Message) []genai.Part {
	parts := make([]genai.Part, len(messages))
	for i, msg := range messages {
		parts[i] = genai.Text(msg.Content)
	}
	return parts
}

func extractGeminiText(candidates []*genai.Candidate) string {
	var result string
	for _, candidate := range candidates {
		if candidate.Content != nil {
			for _, part := range candidate.Content.Parts {
				if text, ok := part.(genai.Text); ok {
					result += string(text)
				}
			}
		}
	}
	return result
}
