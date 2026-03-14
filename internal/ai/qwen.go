package ai

import (
	"github.com/pibot/pibot/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

// QwenProvider implements the Provider interface for Qwen via the DashScope OpenAI-compatible API.
type QwenProvider struct {
	openAICompatibleProvider
	config *config.Config
}

// NewQwenProvider creates a new Qwen provider.
func NewQwenProvider(cfg *config.Config) *QwenProvider {
	aiCfg := cfg.GetAI()
	p := &QwenProvider{config: cfg}
	p.name = "qwen"
	p.model = aiCfg.Qwen.Model
	if aiCfg.Qwen.APIKey != "" {
		clientConfig := openai.DefaultConfig(aiCfg.Qwen.APIKey)
		clientConfig.BaseURL = aiCfg.Qwen.BaseURL
		p.client = openai.NewClientWithConfig(clientConfig)
	}
	return p
}

// RefreshClient updates the client when credentials change.
func (p *QwenProvider) RefreshClient() {
	aiCfg := p.config.GetAI()
	p.model = aiCfg.Qwen.Model
	if aiCfg.Qwen.APIKey != "" {
		clientConfig := openai.DefaultConfig(aiCfg.Qwen.APIKey)
		clientConfig.BaseURL = aiCfg.Qwen.BaseURL
		p.client = openai.NewClientWithConfig(clientConfig)
	}
}
