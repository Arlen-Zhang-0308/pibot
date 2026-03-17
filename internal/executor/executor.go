package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pibot/pibot/internal/config"
)

// ErrDenied is returned when the user explicitly denies a pending command.
var ErrDenied = errors.New("command execution denied by user")

// IsDeniedError reports whether err is (or wraps) ErrDenied.
func IsDeniedError(err error) bool {
	return errors.Is(err, ErrDenied)
}

// contextKey is an unexported type for context keys in this package.
type contextKey int

const (
	// AlwaysAllowKey is the context key for bypassing the pending confirmation flow.
	AlwaysAllowKey contextKey = iota
	// NotifyPendingKey holds a func(result *ExecutionResult) to push a pending WS message.
	NotifyPendingKey
)

// ExecutionResult represents the result of a command execution
type ExecutionResult struct {
	Command   string `json:"command"`
	Output    string `json:"output"`
	Error     string `json:"error,omitempty"`
	ExitCode  int    `json:"exit_code"`
	Duration  string `json:"duration"`
	Level     string `json:"level"`
	Pending   bool   `json:"pending"`
	PendingID string `json:"pending_id,omitempty"`
}

// Executor handles command execution with sandboxing
type Executor struct {
	config  *config.Config
	sandbox *Sandbox

	// gateMu protects gates
	gateMu sync.Mutex
	// gates maps pendingID -> channel that receives true (confirm) or false (deny)
	gates map[string]chan bool
}

// NewExecutor creates a new command executor
func NewExecutor(cfg *config.Config) *Executor {
	return &Executor{
		config:  cfg,
		sandbox: NewSandbox(cfg),
		gates:   make(map[string]chan bool),
	}
}

// Execute runs a command based on its classification.
//
// For dangerous/unknown commands that are not always-allowed it does one of two things:
//   - If a NotifyPendingKey function is in ctx (AI tool-call path): registers the pending
//     entry, calls the notify func so the WS client shows the panel, then BLOCKS until the
//     user confirms or denies via ExecuteConfirmed / CancelPending.
//   - Otherwise (terminal direct-exec path): returns immediately with result.Pending=true
//     as before, so the WS handler can send the pending message itself.
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

	alwaysAllow, _ := ctx.Value(AlwaysAllowKey).(bool)

	switch level {
	case LevelBlocked:
		log.Printf("[exec] BLOCKED command (level=%s): %s", level, command)
		return nil, fmt.Errorf("command is blocked for security reasons: %s", command)

	case LevelDangerous, LevelUnknown:
		if alwaysAllow {
			log.Printf("[exec] always-allow: running command without confirmation (level=%s): %s", level, command)
			return e.executeCommand(ctx, command, level)
		}

		pendingID := uuid.New().String()
		e.sandbox.AddPending(pendingID, command, level)
		result.Pending = true
		result.PendingID = pendingID
		log.Printf("[exec] command requires confirmation (level=%s, pending_id=%s): %s", level, pendingID, command)

		// If a notify function is present in ctx (AI tool-call path), block and wait.
		if notifyFn, ok := ctx.Value(NotifyPendingKey).(func(*ExecutionResult)); ok && notifyFn != nil {
			// Create a gate channel before notifying so the response handler can send on it.
			gateCh := make(chan bool, 1)
			e.gateMu.Lock()
			e.gates[pendingID] = gateCh
			e.gateMu.Unlock()

			// Push the pending notification to the WS client.
			notifyFn(result)

			// Block until confirmed, denied, or context cancelled.
			select {
			case confirmed := <-gateCh:
				if !confirmed {
					log.Printf("[exec] pending command denied by user (pending_id=%s)", pendingID)
					return nil, fmt.Errorf("%w: %s", ErrDenied, command)
				}
				log.Printf("[exec] pending command confirmed via gate (pending_id=%s): %s", pendingID, command)
				return e.executeCommand(ctx, command, level)
			case <-ctx.Done():
				e.sandbox.RemovePending(pendingID)
				e.removeGate(pendingID)
				return nil, ctx.Err()
			}
		}

		// No notify function — return pending result immediately (terminal path).
		return result, nil

	case LevelSafe, LevelModerate:
		log.Printf("[exec] running command (level=%s): %s", level, command)
		return e.executeCommand(ctx, command, level)
	}

	return result, nil
}

// ExecuteConfirmed runs a previously pending command after user confirmation.
// If a gate channel exists for the pendingID (AI tool-call path), it signals
// the waiting goroutine rather than running the command itself.
func (e *Executor) ExecuteConfirmed(ctx context.Context, pendingID string) (*ExecutionResult, error) {
	pending, ok := e.sandbox.GetPending(pendingID)
	if !ok {
		return nil, fmt.Errorf("pending command not found: %s", pendingID)
	}

	e.sandbox.RemovePending(pendingID)
	log.Printf("[exec] confirmed pending command (pending_id=%s): %s", pendingID, pending.Command)

	// If there's a gate, signal confirm and return a minimal result.
	// The actual command output will flow back through the AI streaming channel.
	if ch := e.takeGate(pendingID); ch != nil {
		ch <- true
		return &ExecutionResult{
			Command: pending.Command,
			Level:   pending.Level,
			Pending: false,
		}, nil
	}

	// No gate — direct terminal path: run the command now.
	level := e.sandbox.ClassifyCommand(pending.Command)
	return e.executeCommand(ctx, pending.Command, level)
}

// CancelPending removes a pending command without executing.
// If a gate channel exists (AI tool-call path) it signals denial.
func (e *Executor) CancelPending(pendingID string) error {
	pending, ok := e.sandbox.GetPending(pendingID)
	if !ok {
		return fmt.Errorf("pending command not found: %s", pendingID)
	}
	e.sandbox.RemovePending(pendingID)
	log.Printf("[exec] cancelled pending command (pending_id=%s): %s", pendingID, pending.Command)

	if ch := e.takeGate(pendingID); ch != nil {
		ch <- false
	}
	return nil
}

// ListPending returns all commands awaiting confirmation
func (e *Executor) ListPending() []*PendingCommand {
	return e.sandbox.ListPending()
}

// removeGate deletes a gate channel without sending on it.
func (e *Executor) removeGate(pendingID string) {
	e.gateMu.Lock()
	defer e.gateMu.Unlock()
	delete(e.gates, pendingID)
}

// takeGate atomically retrieves and removes a gate channel.
func (e *Executor) takeGate(pendingID string) chan bool {
	e.gateMu.Lock()
	defer e.gateMu.Unlock()
	ch, ok := e.gates[pendingID]
	if !ok {
		return nil
	}
	delete(e.gates, pendingID)
	return ch
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
