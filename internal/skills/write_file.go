package skills

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pibot/pibot/internal/fileops"
)

// WriteFileParams represents parameters for the write_file skill
type WriteFileParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// WriteFileSkill writes content to a file
type WriteFileSkill struct {
	fileOps *fileops.FileOps
}

// NewWriteFileSkill creates a new write_file skill
func NewWriteFileSkill(fops *fileops.FileOps) *WriteFileSkill {
	return &WriteFileSkill{
		fileOps: fops,
	}
}

// Name returns the skill name
func (s *WriteFileSkill) Name() string {
	return "write_file"
}

// Description returns the skill description
func (s *WriteFileSkill) Description() string {
	return "Write content to a file. Creates the file if it doesn't exist, or overwrites if it does. The path can be relative to the workspace base directory or an absolute path within allowed directories."
}

// Parameters returns the JSON schema for the skill parameters
func (s *WriteFileSkill) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The path to the file to write (relative to workspace or absolute)",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to write to the file",
			},
		},
		"required": []string{"path", "content"},
	}
}

// Execute writes to the file
func (s *WriteFileSkill) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p WriteFileParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	if err := s.fileOps.Write(p.Path, p.Content); err != nil {
		return "", err
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(p.Content), p.Path), nil
}
