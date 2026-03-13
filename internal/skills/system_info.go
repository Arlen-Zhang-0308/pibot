package skills

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

// SystemInfoParams represents parameters for the system_info skill
type SystemInfoParams struct {
	// No parameters needed
}

// SystemInfoSkill returns system information
type SystemInfoSkill struct {
	fileOps *fileops.FileOps
}

// NewSystemInfoSkill creates a new system_info skill
func NewSystemInfoSkill(fops *fileops.FileOps) *SystemInfoSkill {
	return &SystemInfoSkill{
		fileOps: fops,
	}
}

// Name returns the skill name
func (s *SystemInfoSkill) Name() string {
	return "system_info"
}

// Description returns the skill description
func (s *SystemInfoSkill) Description() string {
	return "Get information about the system including current working directory, hostname, OS, architecture, and workspace base directory."
}

// Parameters returns the JSON schema for the skill parameters
func (s *SystemInfoSkill) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}
}

// Execute returns system information
func (s *SystemInfoSkill) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var info []string

	// Current working directory
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "(unable to determine)"
	}
	info = append(info, fmt.Sprintf("Current Directory: %s", cwd))

	// Workspace base directory
	info = append(info, fmt.Sprintf("Workspace Base: %s", s.fileOps.GetBaseDirectory()))

	// Hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "(unable to determine)"
	}
	info = append(info, fmt.Sprintf("Hostname: %s", hostname))

	// OS and architecture
	info = append(info, fmt.Sprintf("OS: %s", runtime.GOOS))
	info = append(info, fmt.Sprintf("Architecture: %s", runtime.GOARCH))

	// Current time
	info = append(info, fmt.Sprintf("Current Time: %s", time.Now().Format("2006-01-02 15:04:05 MST")))

	// User
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
