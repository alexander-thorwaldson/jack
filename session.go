package jack

// SessionChecker reports whether a tmux session exists.
type SessionChecker func(string) bool

// SessionCreator creates a detached tmux session.
type SessionCreator func(name, dir, shellCmd string) error

// SessionAttacher attaches to a tmux session.
type SessionAttacher func(name string) error

// SessionKiller terminates a tmux session.
type SessionKiller func(name string) error

// buildContainerShellCmd builds the command to run inside a Docker container.
// Git identity is set via environment variables, so only claude is needed.
func buildContainerShellCmd() string {
	return "exec claude --dangerously-skip-permissions"
}
