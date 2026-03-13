package ai

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/pibot/pibot/internal/skills"
)

// ActionBlock represents a parsed action from the AI response
type ActionBlock struct {
	Type       string
	Parameters map[string]string
	RawContent string
	StartIndex int
	EndIndex   int
}

// actionRegex matches <action type="...">...</action> blocks
var actionRegex = regexp.MustCompile(`(?s)<action\s+type="([^"]+)">\s*(.*?)\s*</action>`)

// ParseActions extracts action blocks from AI response text
func ParseActions(text string) []ActionBlock {
	matches := actionRegex.FindAllStringSubmatchIndex(text, -1)
	var actions []ActionBlock

	for _, match := range matches {
		if len(match) >= 6 {
			actionType := text[match[2]:match[3]]
			content := text[match[4]:match[5]]

			params := parseActionParams(content)

			actions = append(actions, ActionBlock{
				Type:       actionType,
				Parameters: params,
				RawContent: content,
				StartIndex: match[0],
				EndIndex:   match[1],
			})
		}
	}

	return actions
}

// parseActionParams parses YAML-like key: value pairs from action content
func parseActionParams(content string) map[string]string {
	params := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentKey string
	var currentValue strings.Builder
	inMultiline := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" && !inMultiline {
			continue
		}

		// Check for key: value pattern
		if !inMultiline && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			key := strings.TrimSpace(parts[0])
			value := ""
			if len(parts) > 1 {
				value = strings.TrimSpace(parts[1])
			}

			// Check if this starts a multiline value (with |)
			if value == "|" {
				currentKey = key
				currentValue.Reset()
				inMultiline = true
				continue
			}

			// Regular key: value
			params[key] = value
			currentKey = ""
		} else if inMultiline {
			// Continue multiline value
			if currentValue.Len() > 0 {
				currentValue.WriteString("\n")
			}
			// Remove leading indentation (2 spaces typically)
			trimmedLine := strings.TrimPrefix(line, "  ")
			currentValue.WriteString(trimmedLine)
		}
	}

	// Save any pending multiline value
	if inMultiline && currentKey != "" {
		params[currentKey] = currentValue.String()
	}

	return params
}

// HasActions checks if the text contains any action blocks
func HasActions(text string) bool {
	return actionRegex.MatchString(text)
}

// RemoveActions removes action blocks from text, returning the cleaned text
func RemoveActions(text string) string {
	return actionRegex.ReplaceAllString(text, "")
}

// ExecuteAction executes a single action using the skill registry
func ExecuteAction(ctx context.Context, registry *skills.Registry, action ActionBlock) (string, error) {
	// Convert parameters to JSON for the skill
	var args json.RawMessage
	var err error

	switch action.Type {
	case "execute_command":
		args, err = json.Marshal(map[string]string{
			"command": action.Parameters["command"],
		})
	case "system_info":
		args = json.RawMessage("{}")
	case "read_file":
		args, err = json.Marshal(map[string]string{
			"path": action.Parameters["path"],
		})
	case "write_file":
		args, err = json.Marshal(map[string]string{
			"path":    action.Parameters["path"],
			"content": action.Parameters["content"],
		})
	case "list_directory":
		path := action.Parameters["path"]
		if path == "" {
			path = ""
		}
		args, err = json.Marshal(map[string]string{
			"path": path,
		})
	default:
		return "", nil // Unknown action, skip
	}

	if err != nil {
		return "", err
	}

	return registry.Execute(ctx, action.Type, args)
}

// FormatActionResult formats an action result for inclusion in the conversation
func FormatActionResult(action ActionBlock, result string, isError bool) string {
	var sb strings.Builder
	sb.WriteString("\n<result type=\"")
	sb.WriteString(action.Type)
	sb.WriteString("\"")
	if isError {
		sb.WriteString(" error=\"true\"")
	}
	sb.WriteString(">\n")
	sb.WriteString(result)
	sb.WriteString("\n</result>\n")
	return sb.String()
}
