package tools

import (
	"github.com/pibot/pibot/internal/capabilities"
	"github.com/pibot/pibot/internal/config"
	"github.com/pibot/pibot/internal/executor"
	"github.com/pibot/pibot/internal/fileops"
	"github.com/pibot/pibot/internal/reboot"
)

// Tool defines the interface for a built-in Go tool.
// Tools are pure Go functions registered at server startup and called directly
// by the Agent — no external scripts or interpretation needed.
type Tool interface {
	capabilities.Capability
}

// RegisterAll registers every built-in tool into the capabilities registry.
func RegisterAll(reg *capabilities.Registry, exec *executor.Executor, fops *fileops.FileOps, reboter *reboot.Reboter, cfg *config.Config) {
	for _, t := range []Tool{
		NewExecuteCommandTool(exec),
		NewReadFileTool(fops),
		NewWriteFileTool(fops),
		NewListDirectoryTool(fops),
		NewSystemInfoTool(fops),
		NewWebSearchTool(cfg.GetWebSearch()),
		NewRebootTool(reboter),
	} {
		reg.Register(t, capabilities.KindTool)
	}
}
