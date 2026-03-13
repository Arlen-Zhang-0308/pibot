package ai

import (
	"context"
	"errors"
	"io"

	"github.com/google/generative-ai-go/genai"
	"github.com/pibot/pibot/internal/config"
	"google.golang.org/api/option"
)

// GoogleProvider implements the Provider interface for Google Gemini
type GoogleProvider struct {
	config *config.Config
	client *genai.Client
}

// NewGoogleProvider creates a new Google Gemini provider
func NewGoogleProvider(cfg *config.Config) *GoogleProvider {
	return &GoogleProvider{
		config: cfg,
	}
}

// Name returns the provider name
func (p *GoogleProvider) Name() string {
	return "google"
}

// getClient creates or returns the Gemini client
func (p *GoogleProvider) getClient(ctx context.Context) (*genai.Client, error) {
	aiCfg := p.config.GetAI()
	if aiCfg.Google.APIKey == "" {
		return nil, ErrProviderNotConfigured
	}

	if p.client == nil {
		client, err := genai.NewClient(ctx, option.WithAPIKey(aiCfg.Google.APIKey))
		if err != nil {
			return nil, err
		}
		p.client = client
	}
	return p.client, nil
}

// Chat sends messages and returns a complete response
func (p *GoogleProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	client, err := p.getClient(ctx)
	if err != nil {
		return "", err
	}

	aiCfg := p.config.GetAI()
	model := client.GenerativeModel(aiCfg.Google.Model)

	// Convert messages to Gemini format
	var parts []genai.Part
	for _, msg := range messages {
		parts = append(parts, genai.Text(msg.Content))
	}

	resp, err := model.GenerateContent(ctx, parts...)
	if err != nil {
		return "", err
	}

	// Extract text from response
	var result string
	for _, candidate := range resp.Candidates {
		if candidate.Content != nil {
			for _, part := range candidate.Content.Parts {
				if text, ok := part.(genai.Text); ok {
					result += string(text)
				}
			}
		}
	}

	return result, nil
}

// StreamChat sends messages and streams the response
func (p *GoogleProvider) StreamChat(ctx context.Context, messages []Message, ch chan<- string) error {
	client, err := p.getClient(ctx)
	if err != nil {
		return err
	}

	defer close(ch)

	aiCfg := p.config.GetAI()
	model := client.GenerativeModel(aiCfg.Google.Model)

	// Convert messages to Gemini format
	var parts []genai.Part
	for _, msg := range messages {
		parts = append(parts, genai.Text(msg.Content))
	}

	iter := model.GenerateContentStream(ctx, parts...)

	for {
		resp, err := iter.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		for _, candidate := range resp.Candidates {
			if candidate.Content != nil {
				for _, part := range candidate.Content.Parts {
					if text, ok := part.(genai.Text); ok {
						ch <- string(text)
					}
				}
			}
		}
	}
}

// Close closes the Gemini client
func (p *GoogleProvider) Close() {
	if p.client != nil {
		p.client.Close()
		p.client = nil
	}
}
