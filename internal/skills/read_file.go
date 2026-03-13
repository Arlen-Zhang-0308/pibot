package skills

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pibot/pibot/internal/fileops"
)

// ReadFileParams represents parameters for the read_file skill
type ReadFileParams struct {
	Path string `json:"path"`
}

// ReadFileSkill reads file contents
type ReadFileSkill struct {
	fileOps *fileops.FileOps
}

// NewReadFileSkill creates a new read_file skill
func NewReadFileSkill(fops *fileops.FileOps) *ReadFileSkill {
	return &ReadFileSkill{
		fileOps: fops,
	}
}

// Name returns the skill name
func (s *ReadFileSkill) Name() string {
	return "read_file"
}

// Description returns the skill description
func (s *ReadFileSkill) Description() string {
	return "Read the contents of a file. The path can be relative to the workspace base directory or an absolute path within allowed directories."
}

// Parameters returns the JSON schema for the skill parameters
func (s *ReadFileSkill) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The path to the file to read (relative to workspace or absolute)",
			},
		},
		"required": []string{"path"},
	}
}

// Execute reads the file
func (s *ReadFileSkill) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p ReadFileParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	content, err := s.fileOps.Read(p.Path)
	if err != nil {
		return "", err
	}

	return content, nil
}
