package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Skill defines the interface for a PiBot skill/tool
type Skill interface {
	// Name returns the unique identifier for this skill
	Name() string
	// Description returns a human-readable description for the AI
	Description() string
	// Parameters returns the JSON schema for the skill's parameters
	Parameters() map[string]interface{}
	// Execute runs the skill with the given parameters
	Execute(ctx context.Context, params json.RawMessage) (string, error)
}

// ToolCall represents a tool call request from the AI
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult represents the result of executing a tool
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

// ToolDefinition represents a tool definition for the AI
type ToolDefinition struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef represents a function definition
type FunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// Registry manages available skills
type Registry struct {
	skills map[string]Skill
	mu     sync.RWMutex
}

// NewRegistry creates a new skill registry
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]Skill),
	}
}

// Register adds a skill to the registry
func (r *Registry) Register(skill Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[skill.Name()] = skill
	log.Printf("[skills] registered skill %q", skill.Name())
}

// Get retrieves a skill by name
func (r *Registry) Get(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	skill, ok := r.skills[name]
	return skill, ok
}

// List returns all registered skill names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	return names
}

// GetToolDefinitions returns tool definitions for all registered skills
func (r *Registry) GetToolDefinitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	definitions := make([]ToolDefinition, 0, len(r.skills))
	for _, skill := range r.skills {
		definitions = append(definitions, ToolDefinition{
			Type: "function",
			Function: FunctionDef{
				Name:        skill.Name(),
				Description: skill.Description(),
				Parameters:  skill.Parameters(),
			},
		})
	}
	return definitions
}

// Execute runs a skill by name with the given parameters
func (r *Registry) Execute(ctx context.Context, name string, params json.RawMessage) (string, error) {
	skill, ok := r.Get(name)
	if !ok {
		err := fmt.Errorf("skill %q not found", name)
		log.Printf("[skills] ERROR skill not found: %q", name)
		return "", err
	}

	log.Printf("[skills] executing skill %q params=%s", name, params)
	start := time.Now()
	result, err := skill.Execute(ctx, params)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("[skills] skill %q FAILED in %s: %v", name, elapsed, err)
		return "", err
	}

	log.Printf("[skills] skill %q completed in %s", name, elapsed)
	return result, nil
}

// ExecuteToolCall executes a tool call and returns the result
func (r *Registry) ExecuteToolCall(ctx context.Context, call ToolCall) ToolResult {
	result, err := r.Execute(ctx, call.Name, call.Arguments)
	if err != nil {
		log.Printf("[skills] tool call %q (id=%s) returned error: %v", call.Name, call.ID, err)
		return ToolResult{
			ToolCallID: call.ID,
			Content:    fmt.Sprintf("Error: %v", err),
			IsError:    true,
		}
	}
	return ToolResult{
		ToolCallID: call.ID,
		Content:    result,
		IsError:    false,
	}
}
