package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pibot/pibot/internal/fileops"
)

// ReadFileParams represents parameters for the read_file tool.
type ReadFileParams struct {
	Path string `json:"path"`
}

// ReadFileTool reads file contents.
type ReadFileTool struct {
	fileOps *fileops.FileOps
}

// NewReadFileTool creates a new read_file tool.
func NewReadFileTool(fops *fileops.FileOps) *ReadFileTool {
	return &ReadFileTool{
		fileOps: fops,
	}
}

func (t *ReadFileTool) Name() string { return "read_file" }

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file. The path can be relative to the workspace base directory or an absolute path within allowed directories."
}

func (t *ReadFileTool) Parameters() map[string]interface{} {
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

func (t *ReadFileTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p ReadFileParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	content, err := t.fileOps.Read(p.Path)
	if err != nil {
		return "", err
	}

	return content, nil
}
