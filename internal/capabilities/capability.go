package capabilities

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Kind distinguishes between built-in Go tools and external script-based skills.
type Kind string

const (
	KindTool  Kind = "tool"
	KindSkill Kind = "skill"
)

func (k Kind) logTag() string {
	switch k {
	case KindTool:
		return "[tools]"
	default:
		return "[skills]"
	}
}

// Capability is the common interface satisfied by both tools (Go functions)
// and skills (external scripts). The Agent calls Execute with structured
// JSON parameters and receives a string result.
type Capability interface {
	Name() string
	Description() string
	Parameters() map[string]interface{}
	Execute(ctx context.Context, params json.RawMessage) (string, error)
}

// ToolCall represents a tool/skill call request from the AI.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult represents the result of executing a tool/skill.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

// ToolDefinition represents a tool/skill definition sent to the AI.
type ToolDefinition struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef represents a function definition within a ToolDefinition.
type FunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// entry pairs a capability with its kind for log-routing purposes.
type entry struct {
	cap  Capability
	kind Kind
}

// Registry manages available capabilities (tools and skills).
type Registry struct {
	entries map[string]entry
	mu      sync.RWMutex
}

// NewRegistry creates a new capability registry.
func NewRegistry() *Registry {
	return &Registry{
		entries: make(map[string]entry),
	}
}

// Register adds a capability to the registry with the given kind.
func (r *Registry) Register(cap Capability, kind Kind) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[cap.Name()] = entry{cap: cap, kind: kind}
	log.Printf("%s registered %s %q", kind.logTag(), kind, cap.Name())
}

// Get retrieves a capability by name.
func (r *Registry) Get(name string) (Capability, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[name]
	if !ok {
		return nil, false
	}
	return e.cap, true
}

// List returns all registered capability names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.entries))
	for name := range r.entries {
		names = append(names, name)
	}
	return names
}

// GetToolDefinitions returns tool definitions for all registered capabilities.
func (r *Registry) GetToolDefinitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]ToolDefinition, 0, len(r.entries))
	for _, e := range r.entries {
		defs = append(defs, ToolDefinition{
			Type: "function",
			Function: FunctionDef{
				Name:        e.cap.Name(),
				Description: e.cap.Description(),
				Parameters:  e.cap.Parameters(),
			},
		})
	}
	return defs
}

// Execute runs a capability by name with the given parameters.
func (r *Registry) Execute(ctx context.Context, name string, params json.RawMessage) (string, error) {
	r.mu.RLock()
	e, ok := r.entries[name]
	r.mu.RUnlock()

	if !ok {
		log.Printf("[capabilities] ERROR not found: %q", name)
		return "", fmt.Errorf("capability %q not found", name)
	}

	tag := e.kind.logTag()
	label := string(e.kind)

	log.Printf("%s executing %s %q params=%s", tag, label, name, params)
	start := time.Now()
	result, err := e.cap.Execute(ctx, params)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("%s %s %q FAILED in %s: %v", tag, label, name, elapsed, err)
		return "", err
	}

	log.Printf("%s %s %q completed in %s", tag, label, name, elapsed)
	return result, nil
}

// ExecuteToolCall executes a tool call and returns the result.
func (r *Registry) ExecuteToolCall(ctx context.Context, call ToolCall) ToolResult {
	result, err := r.Execute(ctx, call.Name, call.Arguments)
	if err != nil {
		log.Printf("[tools] tool call %q (id=%s) returned error: %v", call.Name, call.ID, err)
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
