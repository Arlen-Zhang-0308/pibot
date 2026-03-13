package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
		return nil, fmt.Errorf("command is blocked for security reasons: %s", command)
	
	case LevelDangerous, LevelUnknown:
		// Requires confirmation
		pendingID := uuid.New().String()
		e.sandbox.AddPending(pendingID, command, level)
		result.Pending = true
		result.PendingID = pendingID
		return result, nil
	
	case LevelSafe, LevelModerate:
		// Execute directly
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
	
	level := e.sandbox.ClassifyCommand(pending.Command)
	return e.executeCommand(ctx, pending.Command, level)
}

// CancelPending removes a pending command without executing
func (e *Executor) CancelPending(pendingID string) error {
	_, ok := e.sandbox.GetPending(pendingID)
	if !ok {
		return fmt.Errorf("pending command not found: %s", pendingID)
	}
	e.sandbox.RemovePending(pendingID)
	return nil
}

// ListPending returns all commands awaiting confirmation
func (e *Executor) ListPending() []*PendingCommand {
	return e.sandbox.ListPending()
}

// executeCommand actually runs the command
func (e *Executor) executeCommand(ctx context.Context, command string, level CommandLevel) (*ExecutionResult, error) {
	start := time.Now()

	// Create command with context for timeout
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
	} else {
		result.ExitCode = 0
	}

	return result, nil
}

// GetSandbox returns the sandbox for direct access
func (e *Executor) GetSandbox() *Sandbox {
	return e.sandbox
}
