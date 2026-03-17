package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pibot/pibot/internal/fileops"
)

// WriteFileParams represents parameters for the write_file tool.
type WriteFileParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// WriteFileTool writes content to a file.
type WriteFileTool struct {
	fileOps *fileops.FileOps
}

// NewWriteFileTool creates a new write_file tool.
func NewWriteFileTool(fops *fileops.FileOps) *WriteFileTool {
	return &WriteFileTool{
		fileOps: fops,
	}
}

func (t *WriteFileTool) Name() string { return "write_file" }

func (t *WriteFileTool) Description() string {
	return "Write content to a file. Creates the file if it doesn't exist, or overwrites if it does. The path can be relative to the workspace base directory or an absolute path within allowed directories."
}

func (t *WriteFileTool) Parameters() map[string]interface{} {
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

func (t *WriteFileTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p WriteFileParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	if err := t.fileOps.Write(p.Path, p.Content); err != nil {
		return "", err
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(p.Content), p.Path), nil
}
