package ai

import (
	"github.com/pibot/pibot/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements the Provider interface for OpenAI.
type OpenAIProvider struct {
	openAICompatibleProvider
	config *config.Config
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(cfg *config.Config) *OpenAIProvider {
	aiCfg := cfg.GetAI()
	p := &OpenAIProvider{config: cfg}
	p.name = "openai"
	p.model = aiCfg.OpenAI.Model
	if aiCfg.OpenAI.APIKey != "" {
		p.client = openai.NewClient(aiCfg.OpenAI.APIKey)
	}
	return p
}

// RefreshClient updates the client when credentials change.
func (p *OpenAIProvider) RefreshClient() {
	aiCfg := p.config.GetAI()
	p.model = aiCfg.OpenAI.Model
	if aiCfg.OpenAI.APIKey != "" {
		p.client = openai.NewClient(aiCfg.OpenAI.APIKey)
	}
}
