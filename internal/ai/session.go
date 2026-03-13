package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/pibot/pibot/internal/config"
	"github.com/pibot/pibot/internal/prompts"
	"github.com/pibot/pibot/internal/skills"
	openai "github.com/sashabaranov/go-openai"
)

// MaxToolIterations limits the number of tool call iterations to prevent infinite loops
const MaxToolIterations = 10

// MaxPromptToolIterations limits iterations for prompt-based tools
const MaxPromptToolIterations = 5

// ChatSession handles a chat conversation with tool support
type ChatSession struct {
	config   *config.Config
	registry *skills.Registry
	manager  *Manager
}

// NewChatSession creates a new chat session
func NewChatSession(cfg *config.Config, registry *skills.Registry, manager *Manager) *ChatSession {
	return &ChatSession{
		config:   cfg,
		registry: registry,
		manager:  manager,
	}
}

// buildSystemPrompt creates the system prompt with current context
func (s *ChatSession) buildSystemPrompt() Message {
	hostname, _ := os.Hostname()
	workspaceDir := s.getWorkspaceDir()

	prompt := prompts.BuildSystemPromptFromConfig(s.config, workspaceDir, hostname)

	return Message{
		Role:    RoleSystem,
		Content: prompt,
	}
}

// buildPromptBasedToolsSystemPrompt creates a system prompt for prompt-based tool calling
func (s *ChatSession) buildPromptBasedToolsSystemPrompt() Message {
	hostname, _ := os.Hostname()
	workspaceDir := s.getWorkspaceDir()
	botName := "PiBot"

	if s.config != nil {
		promptsCfg := s.config.GetPrompts()
		if promptsCfg.BotName != "" {
			botName = promptsCfg.BotName
		}
	}

	data := prompts.PromptBasedToolsTemplateData{
		BotName:      botName,
		WorkspaceDir: workspaceDir,
		CurrentTime:  time.Now().Format("2006-01-02 15:04:05 MST"),
		Hostname:     hostname,
	}

	tmpl, err := template.New("prompt_tools").Parse(prompts.PromptBasedToolsSystemPrompt)
	if err != nil {
		return Message{
			Role:    RoleSystem,
			Content: "You are " + botName + ", an AI assistant for Raspberry Pi.",
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return Message{
			Role:    RoleSystem,
			Content: "You are " + botName + ", an AI assistant for Raspberry Pi.",
		}
	}

	return Message{
		Role:    RoleSystem,
		Content: buf.String(),
	}
}

// getWorkspaceDir returns the expanded workspace directory
func (s *ChatSession) getWorkspaceDir() string {
	foCfg := s.config.GetFileOps()
	workspaceDir := foCfg.BaseDirectory
	if strings.HasPrefix(workspaceDir, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			workspaceDir = strings.Replace(workspaceDir, "~", home, 1)
		}
	}
	return workspaceDir
}

// PrepareMessages prepares messages with system prompt if not already present
func (s *ChatSession) PrepareMessages(messages []Message) []Message {
	// Check if system message already exists
	hasSystem := false
	for _, msg := range messages {
		if msg.Role == RoleSystem {
			hasSystem = true
			break
		}
	}

	if !hasSystem {
		// Prepend system message
		systemMsg := s.buildSystemPrompt()
		return append([]Message{systemMsg}, messages...)
	}

	return messages
}

// convertToolDefinitions converts skill definitions to OpenAI tool format
func (s *ChatSession) convertToolDefinitions() []openai.Tool {
	skillDefs := s.registry.GetToolDefinitions()
	tools := make([]openai.Tool, len(skillDefs))

	for i, def := range skillDefs {
		tools[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        def.Function.Name,
				Description: def.Function.Description,
				Parameters:  def.Function.Parameters,
			},
		}
	}

	return tools
}

// ChatWithTools sends messages with tool support and handles tool calls
func (s *ChatSession) ChatWithTools(ctx context.Context, providerName string, messages []Message) (string, error) {
	// Check if provider supports native tools (OpenAI/Qwen compatible)
	if s.supportsNativeTools(providerName) {
		// Prepare messages with standard system prompt
		messages = s.PrepareMessages(messages)
		return s.chatWithNativeTools(ctx, providerName, messages)
	}

	// Use prompt-based tools for providers without native tool support
	return s.chatWithPromptBasedTools(ctx, providerName, messages)
}

// chatWithPromptBasedTools handles chat with prompt-based tool calling
func (s *ChatSession) chatWithPromptBasedTools(ctx context.Context, providerName string, messages []Message) (string, error) {
	provider, err := s.manager.GetProvider(providerName)
	if err != nil {
		return "", err
	}

	// Prepare messages with prompt-based tools system prompt
	messages = s.prepareMessagesWithPromptTools(messages)

	var fullResponse strings.Builder

	// Tool execution loop
	for iteration := 0; iteration < MaxPromptToolIterations; iteration++ {
		response, err := provider.Chat(ctx, messages)
		if err != nil {
			return "", err
		}

		fullResponse.WriteString(response)

		// Check for action blocks in the response
		actions := ParseActions(response)
		if len(actions) == 0 {
			// No actions, return the response
			return fullResponse.String(), nil
		}

		// Execute actions and collect results
		var resultBuilder strings.Builder
		for _, action := range actions {
			result, err := ExecuteAction(ctx, s.registry, action)
			if err != nil {
				resultBuilder.WriteString(FormatActionResult(action, "Error: "+err.Error(), true))
			} else {
				resultBuilder.WriteString(FormatActionResult(action, result, false))
			}
		}

		resultsText := resultBuilder.String()
		fullResponse.WriteString(resultsText)

		// Add assistant response and results to messages for next iteration
		messages = append(messages, Message{
			Role:    RoleAssistant,
			Content: response,
		})
		messages = append(messages, Message{
			Role:    RoleUser,
			Content: "Here are the results of your actions:\n" + resultsText + "\n\nPlease provide your response to the user based on these results.",
		})
	}

	return fullResponse.String(), nil
}

// supportsNativeTools checks if a provider supports native OpenAI-style tools
func (s *ChatSession) supportsNativeTools(providerName string) bool {
	switch providerName {
	case "openai", "qwen":
		return true
	default:
		return false
	}
}

// chatWithNativeTools handles chat with OpenAI-compatible tool calling
func (s *ChatSession) chatWithNativeTools(ctx context.Context, providerName string, messages []Message) (string, error) {
	aiCfg := s.config.GetAI()
	tools := s.convertToolDefinitions()

	// Convert messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = openai.ChatCompletionMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	// Get the appropriate client
	var client *openai.Client
	var model string

	switch providerName {
	case "openai":
		if aiCfg.OpenAI.APIKey == "" {
			return "", ErrProviderNotConfigured
		}
		client = openai.NewClient(aiCfg.OpenAI.APIKey)
		model = aiCfg.OpenAI.Model
	case "qwen":
		if aiCfg.Qwen.APIKey == "" {
			return "", ErrProviderNotConfigured
		}
		clientConfig := openai.DefaultConfig(aiCfg.Qwen.APIKey)
		clientConfig.BaseURL = aiCfg.Qwen.BaseURL
		client = openai.NewClientWithConfig(clientConfig)
		model = aiCfg.Qwen.Model
	default:
		return "", errors.New("unsupported provider for native tools")
	}

	// Tool calling loop
	for iteration := 0; iteration < MaxToolIterations; iteration++ {
		resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    model,
			Messages: openaiMessages,
			Tools:    tools,
		})
		if err != nil {
			return "", err
		}

		if len(resp.Choices) == 0 {
			return "", errors.New("no response from AI")
		}

		choice := resp.Choices[0]
		assistantMsg := choice.Message

		// Add assistant message to history
		openaiMessages = append(openaiMessages, assistantMsg)

		// Check if there are tool calls
		if len(assistantMsg.ToolCalls) == 0 {
			// No tool calls, return the content
			return assistantMsg.Content, nil
		}

		// Execute tool calls
		for _, toolCall := range assistantMsg.ToolCalls {
			call := skills.ToolCall{
				ID:        toolCall.ID,
				Name:      toolCall.Function.Name,
				Arguments: json.RawMessage(toolCall.Function.Arguments),
			}

			result := s.registry.ExecuteToolCall(ctx, call)

			// Add tool result to messages
			openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result.Content,
				ToolCallID: toolCall.ID,
			})
		}
	}

	return "", errors.New("maximum tool iterations reached")
}

// StreamChatWithTools streams a chat response with tool support
func (s *ChatSession) StreamChatWithTools(ctx context.Context, providerName string, messages []Message, ch chan<- string) error {
	// For streaming with tools, we need special handling
	if s.supportsNativeTools(providerName) {
		// Prepare messages with standard system prompt
		messages = s.PrepareMessages(messages)
		return s.streamChatWithNativeTools(ctx, providerName, messages, ch)
	}

	// Use prompt-based tools for providers without native tool support
	return s.streamChatWithPromptBasedTools(ctx, providerName, messages, ch)
}

// streamChatWithNativeTools handles streaming chat with tool calling
func (s *ChatSession) streamChatWithNativeTools(ctx context.Context, providerName string, messages []Message, ch chan<- string) error {
	defer close(ch)

	aiCfg := s.config.GetAI()
	tools := s.convertToolDefinitions()

	// Convert messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = openai.ChatCompletionMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	// Get the appropriate client
	var client *openai.Client
	var model string

	switch providerName {
	case "openai":
		if aiCfg.OpenAI.APIKey == "" {
			return ErrProviderNotConfigured
		}
		client = openai.NewClient(aiCfg.OpenAI.APIKey)
		model = aiCfg.OpenAI.Model
	case "qwen":
		if aiCfg.Qwen.APIKey == "" {
			return ErrProviderNotConfigured
		}
		clientConfig := openai.DefaultConfig(aiCfg.Qwen.APIKey)
		clientConfig.BaseURL = aiCfg.Qwen.BaseURL
		client = openai.NewClientWithConfig(clientConfig)
		model = aiCfg.Qwen.Model
	default:
		return errors.New("unsupported provider for native tools")
	}

	// Tool calling loop with streaming
	for iteration := 0; iteration < MaxToolIterations; iteration++ {
		stream, err := client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
			Model:    model,
			Messages: openaiMessages,
			Tools:    tools,
			Stream:   true,
		})
		if err != nil {
			return err
		}

		var contentBuilder strings.Builder
		var toolCalls []openai.ToolCall
		toolCallsMap := make(map[int]*openai.ToolCall)

		// Stream the response
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				stream.Close()
				return err
			}

			if len(response.Choices) > 0 {
				delta := response.Choices[0].Delta

				// Stream content chunks
				if delta.Content != "" {
					ch <- delta.Content
					contentBuilder.WriteString(delta.Content)
				}

				// Accumulate tool calls
				for _, tc := range delta.ToolCalls {
					idx := 0
					if tc.Index != nil {
						idx = *tc.Index
					}

					if _, exists := toolCallsMap[idx]; !exists {
						toolCallsMap[idx] = &openai.ToolCall{
							ID:   tc.ID,
							Type: tc.Type,
							Function: openai.FunctionCall{
								Name:      tc.Function.Name,
								Arguments: tc.Function.Arguments,
							},
						}
					} else {
						// Append to existing tool call
						if tc.ID != "" {
							toolCallsMap[idx].ID = tc.ID
						}
						if tc.Function.Name != "" {
							toolCallsMap[idx].Function.Name = tc.Function.Name
						}
						toolCallsMap[idx].Function.Arguments += tc.Function.Arguments
					}
				}
			}
		}
		stream.Close()

		// Convert map to slice
		for _, tc := range toolCallsMap {
			toolCalls = append(toolCalls, *tc)
		}

		// Add assistant message to history
		assistantMsg := openai.ChatCompletionMessage{
			Role:      openai.ChatMessageRoleAssistant,
			Content:   contentBuilder.String(),
			ToolCalls: toolCalls,
		}
		openaiMessages = append(openaiMessages, assistantMsg)

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
			return nil
		}

		// Execute tool calls and add results
		for _, toolCall := range toolCalls {
			call := skills.ToolCall{
				ID:        toolCall.ID,
				Name:      toolCall.Function.Name,
				Arguments: json.RawMessage(toolCall.Function.Arguments),
			}

			result := s.registry.ExecuteToolCall(ctx, call)

			// Add tool result to messages
			openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result.Content,
				ToolCallID: toolCall.ID,
			})
		}

		// Continue loop to get AI response after tool execution
	}

	return errors.New("maximum tool iterations reached")
}

// streamChatWithPromptBasedTools handles streaming with prompt-based tool calling
// This is used for models that don't support native function calling
func (s *ChatSession) streamChatWithPromptBasedTools(ctx context.Context, providerName string, messages []Message, ch chan<- string) error {
	defer close(ch)

	provider, err := s.manager.GetProvider(providerName)
	if err != nil {
		return err
	}

	// Prepare messages with prompt-based tools system prompt
	messages = s.prepareMessagesWithPromptTools(messages)

	// Tool execution loop
	for iteration := 0; iteration < MaxPromptToolIterations; iteration++ {
		// Collect the full response
		responseCh := make(chan string, 100)
		var responseBuilder strings.Builder

		// Stream from provider
		go func() {
			provider.StreamChat(ctx, messages, responseCh)
		}()

		// Collect and forward chunks
		for chunk := range responseCh {
			responseBuilder.WriteString(chunk)
			ch <- chunk
		}

		fullResponse := responseBuilder.String()

		// Check for action blocks in the response
		actions := ParseActions(fullResponse)
		if len(actions) == 0 {
			// No actions, we're done
			return nil
		}

		// Execute actions and collect results
		var resultBuilder strings.Builder
		for _, action := range actions {
			result, err := ExecuteAction(ctx, s.registry, action)
			if err != nil {
				resultBuilder.WriteString(FormatActionResult(action, "Error: "+err.Error(), true))
			} else {
				resultBuilder.WriteString(FormatActionResult(action, result, false))
			}
		}

		// Send the results to the channel
		resultsText := resultBuilder.String()
		ch <- resultsText

		// Add assistant response and results to messages for next iteration
		messages = append(messages, Message{
			Role:    RoleAssistant,
			Content: fullResponse,
		})
		messages = append(messages, Message{
			Role:    RoleUser,
			Content: "Here are the results of your actions:\n" + resultsText + "\n\nPlease provide your response to the user based on these results.",
		})
	}

	return errors.New("maximum prompt-based tool iterations reached")
}

// prepareMessagesWithPromptTools prepares messages with the prompt-based tools system prompt
func (s *ChatSession) prepareMessagesWithPromptTools(messages []Message) []Message {
	// Check if system message already exists
	hasSystem := false
	for _, msg := range messages {
		if msg.Role == RoleSystem {
			hasSystem = true
			break
		}
	}

	if !hasSystem {
		// Prepend prompt-based tools system message
		systemMsg := s.buildPromptBasedToolsSystemPrompt()
		return append([]Message{systemMsg}, messages...)
	}

	return messages
}
