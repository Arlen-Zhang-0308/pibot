package scheduler

import (
	"context"
	"fmt"
	"strings"

	"github.com/pibot/pibot/internal/ai"
	"github.com/pibot/pibot/internal/executor"
)

// runAction executes a task's action and returns the output string and any error.
func runAction(ctx context.Context, t *Task, exec *executor.Executor, chat *ai.ChatSession) (string, error) {
	switch t.ActionType {
	case ActionShell:
		return runShell(ctx, t.Command, exec)
	case ActionAI:
		return runAI(ctx, t.Command, chat)
	default:
		return "", fmt.Errorf("unknown action type: %s", t.ActionType)
	}
}

// runShell executes a shell command and returns combined stdout/stderr output.
func runShell(ctx context.Context, command string, exec *executor.Executor) (string, error) {
	result, err := exec.Execute(ctx, command)
	if err != nil {
		return "", fmt.Errorf("execution error: %w", err)
	}

	if result.Pending {
		return "", fmt.Errorf("command requires confirmation and cannot be run as a scheduled task (level=dangerous/unknown): %s", command)
	}

	var sb strings.Builder
	if result.Output != "" {
		sb.WriteString(result.Output)
	}
	if result.Error != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(result.Error)
	}

	if result.ExitCode != 0 {
		return sb.String(), fmt.Errorf("command exited with code %d", result.ExitCode)
	}
	return sb.String(), nil
}

// runAI sends a prompt to the AI chat session and returns the response.
func runAI(ctx context.Context, prompt string, chat *ai.ChatSession) (string, error) {
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: prompt},
	}
	messages = chat.PrepareMessages(messages)
	return chat.ChatWithTools(ctx, "", messages)
}
