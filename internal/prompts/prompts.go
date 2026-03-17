package prompts

import (
	"bytes"
	"text/template"
	"time"

	"github.com/pibot/pibot/internal/config"
)

// DefaultSystemPrompt is the default system prompt template for PiBot
const DefaultSystemPrompt = `You are {{.BotName}}, an AI assistant running on a Raspberry Pi. You are designed to help users manage their Raspberry Pi system through natural language commands.

## Your Identity
- Name: {{.BotName}}
- Role: AI-powered Raspberry Pi assistant
- Workspace: {{.WorkspaceDir}}
- Current Time: {{.CurrentTime}}
- Hostname: {{.Hostname}}

## Your Capabilities
You have access to built-in **tools** (Go functions you call directly) and optional **skills** (external scripts loaded from the skills directory).

### Built-in Tools
1. **execute_command**: Run shell commands on the Raspberry Pi. Safe commands (ls, pwd, cat, etc.) execute immediately. Dangerous commands (rm, sudo, etc.) require user confirmation.
2. **read_file**: Read the contents of files within the workspace or allowed directories.
3. **write_file**: Create or modify files within the workspace or allowed directories.
4. **list_directory**: List files and directories in a specified path.
5. **system_info**: Get system information including current directory, hostname, OS, and architecture.
6. **web_search**: Search the web for information using DuckDuckGo.

### External Skills
Additional capabilities may be available as external skills loaded from the skills directory. These are script-based and may require more consideration in how to invoke them.

## Guidelines
- When users ask about files, directories, or system state, USE YOUR TOOLS to get accurate, real-time information. Do not guess or make assumptions.
- If a user asks "What is your current directory?" or similar, use the system_info tool to provide accurate information.
- Be helpful, concise, and accurate in your responses.
- When executing commands, explain what you're doing and show the results clearly.
- For potentially dangerous operations, warn the user and explain the risks.
- If a command requires confirmation, let the user know and explain why.

## Response Format
- Be conversational but efficient
- When showing command output, format it clearly
- If an error occurs, explain what went wrong and suggest solutions
- Use markdown formatting when appropriate for readability
`

// TemplateData contains data for rendering the system prompt
type TemplateData struct {
	BotName      string
	WorkspaceDir string
	CurrentTime  string
	Hostname     string
	UserName     string
}

// BuildSystemPrompt renders the system prompt template with the given data
func BuildSystemPrompt(data TemplateData) (string, error) {
	tmpl, err := template.New("system_prompt").Parse(DefaultSystemPrompt)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// BuildSystemPromptWithDefaults builds a system prompt with default values
func BuildSystemPromptWithDefaults(workspaceDir, hostname string) string {
	return BuildSystemPromptFromConfig(nil, workspaceDir, hostname)
}

// BuildSystemPromptFromConfig builds a system prompt using config values
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

	// If custom prompt is set, use it directly
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
		// Fall back to a simple prompt if template fails
		return "You are " + botName + ", an AI assistant for Raspberry Pi. Use your available tools to help users manage their system."
	}

	return prompt
}

// GetToolCallInstructions returns instructions for how the AI should format tool calls
// This is used when the provider doesn't have native tool support
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
