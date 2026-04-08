package jack

import (
	"context"
	"os"
	"os/exec"
)

// SessionChecker reports whether a tmux session exists.
type SessionChecker func(string) bool

// SessionCreator creates a detached tmux session.
type SessionCreator func(name, dir, shellCmd string) error

// SessionAttacher attaches to a tmux session.
type SessionAttacher func(name string) error

// SessionKiller terminates a tmux session.
type SessionKiller func(name string) error

// KeyAdder adds an SSH key to the agent.
type KeyAdder func(key string) error

func sshAdd(key string) error {
	cmd := exec.CommandContext(context.Background(), "ssh-add", key) // #nosec G204 -- key path from config
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// buildContainerShellCmd builds the command to run inside a Docker container.
// Git identity is set via environment variables, so only claude is needed.
func buildContainerShellCmd() string {
	return "exec claude --dangerously-skip-permissions"
}
