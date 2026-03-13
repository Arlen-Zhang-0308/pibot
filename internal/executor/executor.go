package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pibot/pibot/internal/config"
)

// ExecutionResult represents the result of a command execution
type ExecutionResult struct {
	Command    string `json:"command"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
	ExitCode   int    `json:"exit_code"`
	Duration   string `json:"duration"`
	Level      string `json:"level"`
	Pending    bool   `json:"pending"`
	PendingID  string `json:"pending_id,omitempty"`
}

// Executor handles command execution with sandboxing
type Executor struct {
	config  *config.Config
	sandbox *Sandbox
}

// NewExecutor creates a new command executor
func NewExecutor(cfg *config.Config) *Executor {
	return &Executor{
		config:  cfg,
		sandbox: NewSandbox(cfg),
	}
}

// Execute runs a command based on its classification
func (e *Executor) Execute(ctx context.Context, command string) (*ExecutionResult, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, errors.New("empty command")
	}

	level := e.sandbox.ClassifyCommand(command)

	result := &ExecutionResult{
		Command: command,
		Level:   level.String(),
	}

	switch level {
	case LevelBlocked:
		log.Printf("[exec] BLOCKED command (level=%s): %s", level, command)
		return nil, fmt.Errorf("command is blocked for security reasons: %s", command)

	case LevelDangerous, LevelUnknown:
		pendingID := uuid.New().String()
		e.sandbox.AddPending(pendingID, command, level)
		result.Pending = true
		result.PendingID = pendingID
		log.Printf("[exec] command requires confirmation (level=%s, pending_id=%s): %s", level, pendingID, command)
		return result, nil

	case LevelSafe, LevelModerate:
		log.Printf("[exec] running command (level=%s): %s", level, command)
		return e.executeCommand(ctx, command, level)
	}

	return result, nil
}

// ExecuteConfirmed runs a previously pending command after confirmation
func (e *Executor) ExecuteConfirmed(ctx context.Context, pendingID string) (*ExecutionResult, error) {
	pending, ok := e.sandbox.GetPending(pendingID)
	if !ok {
		return nil, fmt.Errorf("pending command not found: %s", pendingID)
	}

	e.sandbox.RemovePending(pendingID)
	log.Printf("[exec] confirmed pending command (pending_id=%s): %s", pendingID, pending.Command)

	level := e.sandbox.ClassifyCommand(pending.Command)
	return e.executeCommand(ctx, pending.Command, level)
}

// CancelPending removes a pending command without executing
func (e *Executor) CancelPending(pendingID string) error {
	pending, ok := e.sandbox.GetPending(pendingID)
	if !ok {
		return fmt.Errorf("pending command not found: %s", pendingID)
	}
	e.sandbox.RemovePending(pendingID)
	log.Printf("[exec] cancelled pending command (pending_id=%s): %s", pendingID, pending.Command)
	return nil
}

// ListPending returns all commands awaiting confirmation
func (e *Executor) ListPending() []*PendingCommand {
	return e.sandbox.ListPending()
}

// executeCommand actually runs the command
func (e *Executor) executeCommand(ctx context.Context, command string, level CommandLevel) (*ExecutionResult, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	result := &ExecutionResult{
		Command:  command,
		Output:   stdout.String(),
		Duration: duration.String(),
		Level:    level.String(),
		Pending:  false,
	}

	if err != nil {
		result.Error = stderr.String()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			if result.Error == "" {
				result.Error = err.Error()
			}
		}
		log.Printf("[exec] command FAILED (exit=%d, duration=%s): %s\n  stdout: %s\n  stderr: %s",
			result.ExitCode, duration, command,
			truncateLog(stdout.String(), 512),
			truncateLog(result.Error, 512))
	} else {
		result.ExitCode = 0
		log.Printf("[exec] command succeeded (exit=0, duration=%s): %s\n  stdout: %s",
			duration, command, truncateLog(stdout.String(), 512))
	}

	return result, nil
}

// truncateLog truncates s to maxLen characters for log output, appending "…" if trimmed.
func truncateLog(s string, maxLen int) string {
	s = strings.TrimRight(s, "\n")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

// GetSandbox returns the sandbox for direct access
func (e *Executor) GetSandbox() *Sandbox {
	return e.sandbox
}
