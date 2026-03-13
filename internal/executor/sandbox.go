package executor

import (
	"strings"
	"sync"

	"github.com/pibot/pibot/internal/config"
)

// CommandLevel represents the danger level of a command
type CommandLevel int

const (
	LevelSafe CommandLevel = iota
	LevelModerate
	LevelDangerous
	LevelBlocked
	LevelUnknown
)

// String returns a string representation of the command level
func (l CommandLevel) String() string {
	switch l {
	case LevelSafe:
		return "safe"
	case LevelModerate:
		return "moderate"
	case LevelDangerous:
		return "dangerous"
	case LevelBlocked:
		return "blocked"
	default:
		return "unknown"
	}
}

// PendingCommand represents a command awaiting confirmation
type PendingCommand struct {
	ID      string `json:"id"`
	Command string `json:"command"`
	Level   string `json:"level"`
}

// Sandbox manages command classification and pending confirmations
type Sandbox struct {
	config   *config.Config
	pending  map[string]*PendingCommand
	mu       sync.RWMutex
}

// NewSandbox creates a new command sandbox
func NewSandbox(cfg *config.Config) *Sandbox {
	return &Sandbox{
		config:  cfg,
		pending: make(map[string]*PendingCommand),
	}
}

// ClassifyCommand determines the danger level of a command
func (s *Sandbox) ClassifyCommand(command string) CommandLevel {
	execCfg := s.config.GetExecutor()
	
	// Extract the base command (first word)
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return LevelUnknown
	}
	baseCmd := parts[0]
	
	// Check for blocked commands first
	for _, blocked := range execCfg.BlockedCommands {
		if baseCmd == blocked || strings.Contains(command, blocked) {
			return LevelBlocked
		}
	}
	
	// Check dangerous commands
	for _, dangerous := range execCfg.DangerousCommands {
		if baseCmd == dangerous {
			return LevelDangerous
		}
	}
	
	// Check for dangerous patterns
	if containsDangerousPattern(command) {
		return LevelDangerous
	}
	
	// Check moderate commands
	for _, moderate := range execCfg.ModerateCommands {
		if baseCmd == moderate {
			return LevelModerate
		}
	}
	
	// Check safe commands
	for _, safe := range execCfg.SafeCommands {
		if baseCmd == safe {
			return LevelSafe
		}
	}
	
	return LevelUnknown
}

// containsDangerousPattern checks for dangerous command patterns
func containsDangerousPattern(command string) bool {
	patterns := []string{
		"rm -rf",
		"rm -fr",
		"> /dev/",
		"| sudo",
		"&& sudo",
		"; sudo",
		"chmod 777",
		":(){ :|:& };:",
	}
	
	for _, pattern := range patterns {
		if strings.Contains(command, pattern) {
			return true
		}
	}
	return false
}

// AddPending adds a command to the pending list
func (s *Sandbox) AddPending(id, command string, level CommandLevel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[id] = &PendingCommand{
		ID:      id,
		Command: command,
		Level:   level.String(),
	}
}

// GetPending returns a pending command by ID
func (s *Sandbox) GetPending(id string) (*PendingCommand, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cmd, ok := s.pending[id]
	return cmd, ok
}

// RemovePending removes a command from the pending list
func (s *Sandbox) RemovePending(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, id)
}

// ListPending returns all pending commands
func (s *Sandbox) ListPending() []*PendingCommand {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make([]*PendingCommand, 0, len(s.pending))
	for _, cmd := range s.pending {
		result = append(result, cmd)
	}
	return result
}

// RequiresConfirmation returns true if the command requires user confirmation
func (s *Sandbox) RequiresConfirmation(level CommandLevel) bool {
	return level == LevelDangerous || level == LevelUnknown
}
