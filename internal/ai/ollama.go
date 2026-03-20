package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pibot/pibot/internal/config"
)

// OllamaProvider implements the Provider interface for Ollama
type OllamaProvider struct {
	config *config.Config
	client *http.Client
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(cfg *config.Config) *OllamaProvider {
	return &OllamaProvider{
		config: cfg,
		client: &http.Client{},
	}
}

// Name returns the provider name
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// ollamaMessage represents a message in Ollama format
type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaChatRequest represents a chat request to Ollama
type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

// ollamaChatResponse represents a chat response from Ollama
type ollamaChatResponse struct {
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

// Chat sends messages and returns a complete response
func (p *OllamaProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	aiCfg := p.config.GetAI()
	
	ollamaMessages := make([]ollamaMessage, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = ollamaMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	reqBody := ollamaChatRequest{
		Model:    aiCfg.Ollama.Model,
		Messages: ollamaMessages,
		Stream:   false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/api/chat", aiCfg.Ollama.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama request failed: %s - %s", resp.Status, string(respBody))
	}

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}

	return chatResp.Message.Content, nil
}

// StreamChat sends messages and streams the response
func (p *OllamaProvider) StreamChat(ctx context.Context, messages []Message, ch chan<- string) error {
	defer close(ch)

	aiCfg := p.config.GetAI()
	
	ollamaMessages := make([]ollamaMessage, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = ollamaMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	reqBody := ollamaChatRequest{
		Model:    aiCfg.Ollama.Model,
		Messages: ollamaMessages,
		Stream:   true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/chat", aiCfg.Ollama.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama request failed: %s - %s", resp.Status, string(respBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		// Check for cancellation between chunks.
		if ctx.Err() != nil {
			return nil
		}
		var chatResp ollamaChatResponse
		if err := json.Unmarshal(scanner.Bytes(), &chatResp); err != nil {
			continue
		}
		if chatResp.Message.Content != "" {
			select {
			case ch <- chatResp.Message.Content:
			case <-ctx.Done():
				return nil
			}
		}
		if chatResp.Done {
			break
		}
	}

	if ctx.Err() != nil {
		return nil
	}
	return scanner.Err()
}
