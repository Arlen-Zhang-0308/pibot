// Package reboot provides controlled self-restart for the PiBot server.
//
// The Reboter executes a plan-specific strategy to stop the current process
// and relaunch the server in the same environment (e.g. inside a screen session).
// New plans can be added by extending the Execute method.
package reboot

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/pibot/pibot/internal/config"
)

// Reboter performs controlled server restarts.
type Reboter struct {
	cfg *config.Config
}

// New creates a new Reboter backed by the given configuration.
func New(cfg *config.Config) *Reboter {
	return &Reboter{cfg: cfg}
}

// Execute triggers a server reboot according to the configured plan.
// It schedules the restart asynchronously so the caller can still send a
// response before the process exits.
func (r *Reboter) Execute(reason string) error {
	rc := r.cfg.GetReboot()

	log.Printf("[reboot] reboot requested (plan=%q reason=%q)", rc.Plan, reason)

	switch rc.Plan {
	case "screen", "":
		return r.scheduleScreenReboot(rc.Screen)
	default:
		return fmt.Errorf("unknown reboot plan %q", rc.Plan)
	}
}

// scheduleScreenReboot sends the restart command to the screen session that
// hosts the server, then exits the current process.
//
// The approach:
//  1. Stuff a shell command into the screen session that waits briefly (to let
//     the current HTTP response flush), then re-runs the server.
//  2. Exit this process so the session is free to launch the new one.
func (r *Reboter) scheduleScreenReboot(sc config.ScreenRebootConfig) error {
	sessionName := sc.SessionName
	if sessionName == "" {
		sessionName = "pibot"
	}
	workDir := sc.WorkDir
	if workDir == "" {
		workDir = "/home/orangepi/workspace/pibot"
	}
	startCmd := sc.StartCommand
	if startCmd == "" {
		startCmd = "go run cmd/server/main.go"
	}

	// Build a shell snippet that:
	//   • sleeps 1 s (allow HTTP response to flush)
	//   • cd to workDir
	//   • runs the server
	shellSnippet := fmt.Sprintf(
		"sleep 1 && cd %s && %s",
		shellQuote(workDir),
		startCmd,
	)

	// Stuff the command into the screen session (non-blocking).
	screenCmd := exec.Command(
		"screen", "-S", sessionName,
		"-X", "stuff", shellSnippet+"\n",
	)
	if out, err := screenCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("screen stuff failed: %w (output: %s)", err, out)
	}

	log.Printf("[reboot] restart command sent to screen session %q; exiting in 500ms", sessionName)

	// Exit after a short delay so any pending HTTP writes can complete.
	go func() {
		time.Sleep(500 * time.Millisecond)
		log.Printf("[reboot] exiting process now")
		os.Exit(0)
	}()

	return nil
}

// shellQuote wraps s in single quotes, escaping any existing single quotes.
func shellQuote(s string) string {
	out := "'"
	for _, ch := range s {
		if ch == '\'' {
			out += "'\\''"
		} else {
			out += string(ch)
		}
	}
	return out + "'"
}
