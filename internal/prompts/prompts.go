package prompts

import (
	"bytes"
	"log"
	"text/template"
	"time"

	"github.com/pibot/pibot/internal/config"
)

// TemplateData contains data for rendering the system prompt.
type TemplateData struct {
	BotName      string
	WorkspaceDir string
	CurrentTime  string
	Hostname     string
	UserName     string
}

// systemPromptTemplate returns the parsed system prompt template from the
// embedded files/system_prompt.md file.
func systemPromptTemplate() (*template.Template, error) {
	raw, err := readPromptFile("system_prompt.md")
	if err != nil {
		return nil, err
	}
	return template.New("system_prompt").Parse(string(raw))
}

// BuildSystemPrompt renders the system prompt template with the given data.
func BuildSystemPrompt(data TemplateData) (string, error) {
	tmpl, err := systemPromptTemplate()
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// BuildSystemPromptWithDefaults builds a system prompt with default values.
func BuildSystemPromptWithDefaults(workspaceDir, hostname string) string {
	return BuildSystemPromptFromConfig(nil, workspaceDir, hostname)
}

// BuildSystemPromptFromConfig builds a system prompt using config values.
// Resolution order:
//  1. system_prompt field in config (inline override string)
//  2. Embedded files/system_prompt.md template
func BuildSystemPromptFromConfig(cfg *config.Config, workspaceDir, hostname string) string {
	botName := "PiBot"
	customPrompt := ""

	if cfg != nil {
		promptsCfg := cfg.GetPrompts()
		if promptsCfg.BotName != "" {
			botName = promptsCfg.BotName
		}
		customPrompt = promptsCfg.SystemPrompt
	}

	// Inline config value takes highest priority.
	if customPrompt != "" {
		return customPrompt
	}

	data := TemplateData{
		BotName:      botName,
		WorkspaceDir: workspaceDir,
		CurrentTime:  time.Now().Format("2006-01-02 15:04:05 MST"),
		Hostname:     hostname,
	}

	prompt, err := BuildSystemPrompt(data)
	if err != nil {
		log.Printf("prompts: failed to render system_prompt.md: %v", err)
		return "You are " + botName + ", an AI assistant for Raspberry Pi. Use your available tools to help users manage their system."
	}

	return prompt
}

// ToolCallInstructions returns instructions for how the AI should format tool
// calls when the provider does not have native tool support.
const ToolCallInstructions = `
## Tool Call Format
When you need to use a tool, respond with a JSON tool call in this exact format:
` + "```json" + `
{
  "tool_calls": [
    {
      "id": "call_1",
      "name": "tool_name",
      "arguments": {
        "param1": "value1"
      }
    }
  ]
}
` + "```" + `

After receiving tool results, incorporate them into your response to the user.
`
