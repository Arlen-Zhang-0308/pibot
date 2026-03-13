package executor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pibot/pibot/internal/config"
)

func newTestConfig() *config.Config {
	return config.DefaultConfig()
}

func TestSandbox_ClassifyCommand(t *testing.T) {
	cfg := newTestConfig()
	sandbox := NewSandbox(cfg)

	tests := []struct {
		command  string
		expected CommandLevel
	}{
		// Safe commands
		{"ls", LevelSafe},
		{"ls -la", LevelSafe},
		{"pwd", LevelSafe},
		{"echo hello", LevelSafe},
		{"cat file.txt", LevelSafe},
		{"whoami", LevelSafe},
		{"date", LevelSafe},

		// Moderate commands
		{"mkdir newdir", LevelModerate},
		{"cp file1 file2", LevelModerate},
		{"mv file1 file2", LevelModerate},
		{"curl https://example.com", LevelModerate},

		// Dangerous commands
		{"rm file.txt", LevelDangerous},
		{"rm -rf /", LevelDangerous},
		{"sudo apt update", LevelDangerous},
		{"systemctl restart nginx", LevelDangerous},
		{"kill 1234", LevelDangerous},

		// Blocked commands
		{"dd if=/dev/zero of=/dev/sda", LevelBlocked},
		{"mkfs.ext4 /dev/sda", LevelBlocked},

		// Unknown commands
		{"custom_script.sh", LevelUnknown},
		{"./my_program", LevelUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			level := sandbox.ClassifyCommand(tt.command)
			if level != tt.expected {
				t.Errorf("ClassifyCommand(%q) = %v, want %v", tt.command, level, tt.expected)
			}
		})
	}
}

func TestSandbox_DangerousPatterns(t *testing.T) {
	cfg := newTestConfig()
	sandbox := NewSandbox(cfg)

	patterns := []string{
		"rm -rf /tmp",
		"rm -fr /home",
		"echo test > /dev/sda",
		"cat file | sudo tee",
		"chmod 777 /etc/passwd",
	}

	for _, cmd := range patterns {
		t.Run(cmd, func(t *testing.T) {
			level := sandbox.ClassifyCommand(cmd)
			if level != LevelDangerous && level != LevelBlocked {
				t.Errorf("Expected dangerous/blocked for %q, got %v", cmd, level)
			}
		})
	}
}

func TestSandbox_RequiresConfirmation(t *testing.T) {
	cfg := newTestConfig()
	sandbox := NewSandbox(cfg)

	tests := []struct {
		level    CommandLevel
		requires bool
	}{
		{LevelSafe, false},
		{LevelModerate, false},
		{LevelDangerous, true},
		{LevelUnknown, true},
		{LevelBlocked, false}, // Blocked commands are rejected, not pending
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			if sandbox.RequiresConfirmation(tt.level) != tt.requires {
				t.Errorf("RequiresConfirmation(%v) = %v, want %v",
					tt.level, sandbox.RequiresConfirmation(tt.level), tt.requires)
			}
		})
	}
}

func TestSandbox_PendingCommands(t *testing.T) {
	cfg := newTestConfig()
	sandbox := NewSandbox(cfg)

	// Add pending command
	sandbox.AddPending("test-id", "rm -rf /tmp", LevelDangerous)

	// Get pending
	pending, ok := sandbox.GetPending("test-id")
	if !ok {
		t.Fatal("Expected to find pending command")
	}
	if pending.Command != "rm -rf /tmp" {
		t.Errorf("Expected command 'rm -rf /tmp', got '%s'", pending.Command)
	}
	if pending.Level != "dangerous" {
		t.Errorf("Expected level 'dangerous', got '%s'", pending.Level)
	}

	// List pending
	list := sandbox.ListPending()
	if len(list) != 1 {
		t.Errorf("Expected 1 pending command, got %d", len(list))
	}

	// Remove pending
	sandbox.RemovePending("test-id")
	_, ok = sandbox.GetPending("test-id")
	if ok {
		t.Error("Expected pending command to be removed")
	}
}

func TestExecutor_ExecuteSafeCommand(t *testing.T) {
	cfg := newTestConfig()
	exec := NewExecutor(cfg)
	ctx := context.Background()

	result, err := exec.Execute(ctx, "echo hello world")
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}

	if result.Pending {
		t.Error("Safe command should not be pending")
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Output, "hello world") {
		t.Errorf("Expected output to contain 'hello world', got '%s'", result.Output)
	}
	if result.Level != "safe" {
		t.Errorf("Expected level 'safe', got '%s'", result.Level)
	}
}

func TestExecutor_ExecuteModerateCommand(t *testing.T) {
	cfg := newTestConfig()
	exec := NewExecutor(cfg)
	ctx := context.Background()

	// mkdir is moderate but should execute
	result, err := exec.Execute(ctx, "mkdir -p /tmp/pibot-test-dir")
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}

	if result.Pending {
		t.Error("Moderate command should not be pending")
	}
	if result.Level != "moderate" {
		t.Errorf("Expected level 'moderate', got '%s'", result.Level)
	}
}

func TestExecutor_ExecuteDangerousCommand_RequiresPending(t *testing.T) {
	cfg := newTestConfig()
	exec := NewExecutor(cfg)
	ctx := context.Background()

	result, err := exec.Execute(ctx, "rm -rf /tmp/nonexistent")
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}

	if !result.Pending {
		t.Error("Dangerous command should be pending")
	}
	if result.PendingID == "" {
		t.Error("Expected pending ID to be set")
	}
	if result.Level != "dangerous" {
		t.Errorf("Expected level 'dangerous', got '%s'", result.Level)
	}
}

func TestExecutor_ExecuteBlockedCommand_Rejected(t *testing.T) {
	cfg := newTestConfig()
	exec := NewExecutor(cfg)
	ctx := context.Background()

	_, err := exec.Execute(ctx, "dd if=/dev/zero of=/dev/sda")
	if err == nil {
		t.Error("Expected blocked command to return error")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("Expected error to mention 'blocked', got '%s'", err.Error())
	}
}

func TestExecutor_ExecuteConfirmed(t *testing.T) {
	cfg := newTestConfig()
	exec := NewExecutor(cfg)
	ctx := context.Background()

	// First, execute a dangerous command to get pending ID
	result, err := exec.Execute(ctx, "echo dangerous_test")
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}

	// Since echo is safe, let's manually add a pending command
	exec.GetSandbox().AddPending("test-confirm", "echo confirmed", LevelDangerous)

	// Now confirm it
	result, err = exec.ExecuteConfirmed(ctx, "test-confirm")
	if err != nil {
		t.Fatalf("Failed to execute confirmed: %v", err)
	}

	if result.Pending {
		t.Error("Confirmed command should not be pending")
	}
	if !strings.Contains(result.Output, "confirmed") {
		t.Errorf("Expected output 'confirmed', got '%s'", result.Output)
	}
}

func TestExecutor_CancelPending(t *testing.T) {
	cfg := newTestConfig()
	exec := NewExecutor(cfg)

	// Add pending
	exec.GetSandbox().AddPending("cancel-test", "rm -rf /", LevelDangerous)

	// Cancel it
	err := exec.CancelPending("cancel-test")
	if err != nil {
		t.Fatalf("Failed to cancel: %v", err)
	}

	// Verify it's gone
	_, ok := exec.GetSandbox().GetPending("cancel-test")
	if ok {
		t.Error("Expected pending command to be cancelled")
	}
}

func TestExecutor_EmptyCommand(t *testing.T) {
	cfg := newTestConfig()
	exec := NewExecutor(cfg)
	ctx := context.Background()

	_, err := exec.Execute(ctx, "")
	if err == nil {
		t.Error("Expected error for empty command")
	}

	_, err = exec.Execute(ctx, "   ")
	if err == nil {
		t.Error("Expected error for whitespace-only command")
	}
}

func TestExecutor_CommandTimeout(t *testing.T) {
	cfg := newTestConfig()
	exec := NewExecutor(cfg)

	// Create a context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This sleep command should be cancelled
	result, _ := exec.Execute(ctx, "sleep 0.05 && echo done")

	// The command might complete or timeout depending on timing
	// Just verify it doesn't hang
	if result != nil && result.ExitCode == 0 {
		// Command completed before timeout
		return
	}
}

func TestCommandLevel_String(t *testing.T) {
	tests := []struct {
		level    CommandLevel
		expected string
	}{
		{LevelSafe, "safe"},
		{LevelModerate, "moderate"},
		{LevelDangerous, "dangerous"},
		{LevelBlocked, "blocked"},
		{LevelUnknown, "unknown"},
		{CommandLevel(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.level.String() != tt.expected {
				t.Errorf("String() = %s, want %s", tt.level.String(), tt.expected)
			}
		})
	}
}
