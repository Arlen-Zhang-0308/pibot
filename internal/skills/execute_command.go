package skills

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pibot/pibot/internal/executor"
)

// ExecuteCommandParams represents parameters for the execute_command skill
type ExecuteCommandParams struct {
	Command string `json:"command"`
}

// ExecuteCommandSkill executes shell commands on the Raspberry Pi
type ExecuteCommandSkill struct {
	executor *executor.Executor
}

// NewExecuteCommandSkill creates a new execute_command skill
func NewExecuteCommandSkill(exec *executor.Executor) *ExecuteCommandSkill {
	return &ExecuteCommandSkill{
		executor: exec,
	}
}

// Name returns the skill name
func (s *ExecuteCommandSkill) Name() string {
	return "execute_command"
}

// Description returns the skill description
func (s *ExecuteCommandSkill) Description() string {
	return "Execute a shell command on the Raspberry Pi. Use this to run system commands like ls, pwd, cat, grep, etc. Commands are sandboxed for security - safe commands execute immediately, while dangerous commands require user confirmation."
}

// Parameters returns the JSON schema for the skill parameters
func (s *ExecuteCommandSkill) Parameters() map[string]interface{} {
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

// Execute runs the command
func (s *ExecuteCommandSkill) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p ExecuteCommandParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	result, err := s.executor.Execute(ctx, p.Command)
	if err != nil {
		return "", err
	}

	// Format the result
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
