package jack

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

func buildShellCmd(profile Profile, dir string) string {
	var parts []string

	// Set git identity.
	if profile.Git.Name != "" {
		parts = append(parts, fmt.Sprintf("git config user.name %q", profile.Git.Name))
	}
	if profile.Git.Email != "" {
		parts = append(parts, fmt.Sprintf("git config user.email %q", profile.Git.Email))
	}

	// Source the .env file for session variables.
	parts = append(parts, fmt.Sprintf("set -a && . %s/.env && set +a", dir))

	parts = append(parts, "exec claude --dangerously-skip-permissions --teammate-mode in-process")
	return strings.Join(parts, " && ")
}

// buildBwrapShellCmd builds a shell command that launches Claude inside a
// bwrap sandbox. Kept for future use once bwrap integration is debugged.
//
//nolint:unused // intentionally kept for future use
func buildBwrapShellCmd(profile Profile, dir string) string {
	var parts []string

	// Set git identity before entering the sandbox.
	if profile.Git.Name != "" {
		parts = append(parts, fmt.Sprintf("git config user.name %q", profile.Git.Name))
	}
	if profile.Git.Email != "" {
		parts = append(parts, fmt.Sprintf("git config user.email %q", profile.Git.Email))
	}

	// Build bwrap command.
	home, _ := os.UserHomeDir()

	bwrap := []string{"exec bwrap"}

	// Read-only base filesystem.
	bwrap = append(bwrap, "--ro-bind / /")

	// Proper /dev, /proc, writable /tmp.
	bwrap = append(bwrap, "--dev /dev", "--proc /proc", "--tmpfs /tmp")

	// Working directory read-write.
	bwrap = append(bwrap, fmt.Sprintf("--bind %s %s", dir, dir))

	// Jack config directory read-only (symlink targets resolve here).
	configDir := env.configDir()
	bwrap = append(bwrap, fmt.Sprintf("--ro-bind %s %s", configDir, configDir))

	// Claude local state (session data, caches).
	claudeLocal := fmt.Sprintf("%s/.claude/local", home)
	if _, err := os.Stat(claudeLocal); err == nil {
		bwrap = append(bwrap, fmt.Sprintf("--bind %s %s", claudeLocal, claudeLocal))
	}

	// SSH agent socket for git operations.
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		sockDir := sock[:strings.LastIndex(sock, "/")]
		bwrap = append(bwrap, fmt.Sprintf("--ro-bind %s %s", sockDir, sockDir))
	}

	bwrap = append(bwrap, "-- claude --dangerously-skip-permissions --teammate-mode in-process")

	parts = append(parts, strings.Join(bwrap, " "))
	return strings.Join(parts, " && ")
}

// buildDotEnv creates the content for a .env file containing session
// environment variables.
func buildDotEnv(team, token, ghToken string) string {
	var lines []string
	if team != "" {
		lines = append(lines, "export JACK_TEAM="+team)
	}
	if token != "" {
		lines = append(lines, "export JACK_MSG_TOKEN="+token)
	}
	if ghToken != "" {
		lines = append(lines, "export GH_TOKEN="+ghToken)
	}
	return strings.Join(lines, "\n") + "\n"
}
