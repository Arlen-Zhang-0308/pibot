package prompts

import (
	"bytes"
	"log"
	"text/template"
	"time"
)

// PromptBasedToolsTemplateData contains data for the prompt-based tools system prompt.
type PromptBasedToolsTemplateData struct {
	BotName      string
	WorkspaceDir string
	CurrentTime  string
	Hostname     string
}

// BuildPromptBasedToolsSystemPrompt renders the prompt_tools.md template with
// the given data. It falls back to a minimal inline prompt on error.
func BuildPromptBasedToolsSystemPrompt(data PromptBasedToolsTemplateData) string {
	raw, err := readPromptFile("prompt_tools.md")
	if err != nil {
		log.Printf("prompts: failed to read prompt_tools.md: %v", err)
		return "You are " + data.BotName + ", an AI assistant for Raspberry Pi."
	}

	tmpl, err := template.New("prompt_tools").Parse(string(raw))
	if err != nil {
		log.Printf("prompts: failed to parse prompt_tools.md: %v", err)
		return "You are " + data.BotName + ", an AI assistant for Raspberry Pi."
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		log.Printf("prompts: failed to render prompt_tools.md: %v", err)
		return "You are " + data.BotName + ", an AI assistant for Raspberry Pi."
	}

	return buf.String()
}

// PromptBasedToolsSystemPromptNow is a convenience wrapper that builds the
// prompt-based tools system prompt with the current timestamp.
func PromptBasedToolsSystemPromptNow(botName, workspaceDir, hostname string) string {
	return BuildPromptBasedToolsSystemPrompt(PromptBasedToolsTemplateData{
		BotName:      botName,
		WorkspaceDir: workspaceDir,
		CurrentTime:  time.Now().Format("2006-01-02 15:04:05 MST"),
		Hostname:     hostname,
	})
}
