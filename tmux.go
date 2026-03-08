package jack

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// TmuxSession represents a single tmux session as reported by tmux.
type TmuxSession struct {
	Name     string
	Created  time.Time
	Activity time.Time
	Path     string
	Attached bool
	Windows  int
}

// SessionInfo is a parsed jack-managed session with team and repo extracted.
type SessionInfo struct {
	Team string
	Repo string
	TmuxSession
}

// SessionName builds the canonical tmux session name for a team and repo.
func SessionName(team, repo string) string {
	return team + "-" + repo
}

// ListSessions calls tmux list-sessions and returns all tmux sessions.
// Returns an empty slice (not an error) if the tmux server is not running.
func ListSessions() ([]TmuxSession, error) {
	format := "#{session_name}\t#{session_created}\t#{session_activity}\t#{session_path}\t#{session_attached}\t#{session_windows}"
	cmd := exec.CommandContext(context.Background(), "tmux", "list-sessions", "-F", format)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if strings.Contains(msg, "no server running") ||
			strings.Contains(msg, "no sessions") ||
			strings.Contains(msg, "error connecting") {
			return nil, nil
		}
		return nil, fmt.Errorf("tmux list-sessions: %w: %s", err, msg)
	}

	return parseSessions(stdout.String())
}

// HasSession checks whether a tmux session with the given name exists.
func HasSession(name string) bool {
	cmd := exec.CommandContext(context.Background(), "tmux", "has-session", "-t", name) // #nosec G204 -- args from internal session name
	return cmd.Run() == nil
}

// AttachSession attaches to an existing tmux session, taking over the terminal.
func AttachSession(name string) error {
	cmd := exec.CommandContext(context.Background(), "tmux", "attach-session", "-t", name) // #nosec G204 -- args from internal session name
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// KillSession terminates a tmux session.
func KillSession(name string) error {
	cmd := exec.CommandContext(context.Background(), "tmux", "kill-session", "-t", name) // #nosec G204 -- args from internal session name
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux kill-session: %w: %s", err, stderr.String())
	}
	return nil
}

// CreateSession creates a detached tmux session running the given shell command
// in the given directory.
func CreateSession(name, dir, shellCmd string) error {
	cmd := exec.CommandContext(context.Background(), "tmux", "new-session", "-d", "-s", name, "-c", dir, shellCmd) // #nosec G204 -- args from internal paths
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux new-session: %w: %s", err, stderr.String())
	}
	return nil
}

func parseSessions(output string) ([]TmuxSession, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	sessions := make([]TmuxSession, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 6)
		if len(parts) != 6 {
			return nil, fmt.Errorf("unexpected tmux output: %q", line)
		}
		s, err := parseTmuxSession(parts)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func parseTmuxSession(parts []string) (TmuxSession, error) {
	created, err := parseUnixTimestamp(parts[1])
	if err != nil {
		return TmuxSession{}, fmt.Errorf("parsing created time: %w", err)
	}
	activity, err := parseUnixTimestamp(parts[2])
	if err != nil {
		return TmuxSession{}, fmt.Errorf("parsing activity time: %w", err)
	}
	return TmuxSession{
		Name:     parts[0],
		Created:  created,
		Activity: activity,
		Path:     parts[3],
		Attached: parts[4] != "0",
		Windows:  parseIntOr(parts[5], 1),
	}, nil
}

func parseUnixTimestamp(s string) (time.Time, error) {
	sec, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(sec, 0), nil
}

func parseIntOr(s string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return fallback
	}
	return n
}
