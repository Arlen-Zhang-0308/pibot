package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pibot/pibot/internal/reboot"
)

// RebootParams represents parameters for the reboot_server tool.
type RebootParams struct {
	Reason string `json:"reason,omitempty"`
}

// RebootTool triggers a controlled server restart.
type RebootTool struct {
	reboter *reboot.Reboter
}

// NewRebootTool creates a new reboot_server tool backed by r.
func NewRebootTool(r *reboot.Reboter) *RebootTool {
	return &RebootTool{reboter: r}
}

func (t *RebootTool) Name() string { return "reboot_server" }

func (t *RebootTool) Description() string {
	return "Reboot (restart) the PiBot server. Use this when the user asks to restart, reboot, or reload the bot. The server will shut down and immediately relaunch in the same screen session."
}

func (t *RebootTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"reason": map[string]interface{}{
				"type":        "string",
				"description": "Optional reason for the reboot (logged for diagnostics).",
			},
		},
		"required": []string{},
	}
}

func (t *RebootTool) Execute(_ context.Context, params json.RawMessage) (string, error) {
	var p RebootParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return "", fmt.Errorf("invalid parameters: %w", err)
		}
	}

	if err := t.reboter.Execute(p.Reason); err != nil {
		return "", fmt.Errorf("reboot failed: %w", err)
	}

	return "Reboot initiated. The server will restart momentarily.", nil
}
