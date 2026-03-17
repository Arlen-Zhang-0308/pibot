package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pibot/pibot/internal/executor"
)

// ExecuteCommandParams represents parameters for the execute_command tool.
type ExecuteCommandParams struct {
	Command string `json:"command"`
}

// ExecuteCommandTool executes shell commands on the Raspberry Pi.
type ExecuteCommandTool struct {
	executor *executor.Executor
}

// NewExecuteCommandTool creates a new execute_command tool.
func NewExecuteCommandTool(exec *executor.Executor) *ExecuteCommandTool {
	return &ExecuteCommandTool{
		executor: exec,
	}
}

func (t *ExecuteCommandTool) Name() string { return "execute_command" }

func (t *ExecuteCommandTool) Description() string {
	return "Execute a shell command on the Raspberry Pi. Use this to run system commands like ls, pwd, cat, grep, etc. Commands are sandboxed for security - safe commands execute immediately, while dangerous commands require user confirmation."
}

func (t *ExecuteCommandTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The shell command to execute (e.g., 'ls -la', 'pwd', 'cat file.txt')",
			},
		},
		"required": []string{"command"},
	}
}

func (t *ExecuteCommandTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p ExecuteCommandParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	result, err := t.executor.Execute(ctx, p.Command)
	if err != nil {
		// Surface denial as a clear tool result so the AI understands the user
		// explicitly refused — not a transient failure to retry.
		if executor.IsDeniedError(err) {
			return "USER DENIED EXECUTION: The user explicitly declined to run this command. Do NOT retry or ask again. Inform the user that the command was not executed because they denied it.", nil
		}
		return "", err
	}

	if result.Pending {
		return fmt.Sprintf("Command requires user confirmation (security level: %s). Pending ID: %s\nCommand: %s",
			result.Level, result.PendingID, result.Command), nil
	}

	output := result.Output
	if result.Error != "" {
		output += "\nError: " + result.Error
	}
	if result.ExitCode != 0 {
		output += fmt.Sprintf("\nExit code: %d", result.ExitCode)
	}

	if output == "" {
		output = "(command completed with no output)"
	}

	return output, nil
}
