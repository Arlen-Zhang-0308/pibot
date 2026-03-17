package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/pibot/pibot/internal/fileops"
)

// SystemInfoParams represents parameters for the system_info tool.
type SystemInfoParams struct{}

// SystemInfoTool returns system information.
type SystemInfoTool struct {
	fileOps *fileops.FileOps
}

// NewSystemInfoTool creates a new system_info tool.
func NewSystemInfoTool(fops *fileops.FileOps) *SystemInfoTool {
	return &SystemInfoTool{
		fileOps: fops,
	}
}

func (t *SystemInfoTool) Name() string { return "system_info" }

func (t *SystemInfoTool) Description() string {
	return "Get information about the system including current working directory, hostname, OS, architecture, and workspace base directory."
}

func (t *SystemInfoTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}
}

func (t *SystemInfoTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var info []string

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "(unable to determine)"
	}
	info = append(info, fmt.Sprintf("Current Directory: %s", cwd))

	info = append(info, fmt.Sprintf("Workspace Base: %s", t.fileOps.GetBaseDirectory()))

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "(unable to determine)"
	}
	info = append(info, fmt.Sprintf("Hostname: %s", hostname))

	info = append(info, fmt.Sprintf("OS: %s", runtime.GOOS))
	info = append(info, fmt.Sprintf("Architecture: %s", runtime.GOARCH))

	info = append(info, fmt.Sprintf("Current Time: %s", time.Now().Format("2006-01-02 15:04:05 MST")))

	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME")
	}
	if user == "" {
		user = "(unable to determine)"
	}
	info = append(info, fmt.Sprintf("User: %s", user))

	return strings.Join(info, "\n"), nil
}
