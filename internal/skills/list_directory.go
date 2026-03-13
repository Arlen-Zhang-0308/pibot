package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pibot/pibot/internal/fileops"
)

// ListDirectoryParams represents parameters for the list_directory skill
type ListDirectoryParams struct {
	Path string `json:"path"`
}

// ListDirectorySkill lists directory contents
type ListDirectorySkill struct {
	fileOps *fileops.FileOps
}

// NewListDirectorySkill creates a new list_directory skill
func NewListDirectorySkill(fops *fileops.FileOps) *ListDirectorySkill {
	return &ListDirectorySkill{
		fileOps: fops,
	}
}

// Name returns the skill name
func (s *ListDirectorySkill) Name() string {
	return "list_directory"
}

// Description returns the skill description
func (s *ListDirectorySkill) Description() string {
	return "List the contents of a directory. Returns file and directory names with their sizes and modification times. If no path is provided, lists the workspace base directory."
}

// Parameters returns the JSON schema for the skill parameters
func (s *ListDirectorySkill) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "The path to the directory to list (optional, defaults to workspace root)",
			},
		},
		"required": []string{},
	}
}

// Execute lists the directory
func (s *ListDirectorySkill) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p ListDirectoryParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	files, err := s.fileOps.List(p.Path)
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "Directory is empty", nil
	}

	var lines []string
	for _, f := range files {
		typeStr := "FILE"
		if f.IsDir {
			typeStr = "DIR "
		}
		lines = append(lines, fmt.Sprintf("[%s] %s  (%s, %s)", typeStr, f.Name, formatSize(f.Size), f.ModTime))
	}

	return strings.Join(lines, "\n"), nil
}

// formatSize formats a file size in human-readable format
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
