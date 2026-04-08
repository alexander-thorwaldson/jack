//go:build testing

package jack

import (
	"strings"
	"testing"

	jtesting "github.com/zoobzio/jack/testing"
)

// Shared test helpers used across multiple test files.

func newTestConfig() {
	cfg = Config{
		Profiles: map[string]Profile{
			"blue": {
				Git: GitConfig{Name: "Rockhopper", Email: "rock@example.com"},
			},
		},
	}
}

func noopChecker(string) bool              { return false }
func existsChecker(string) bool            { return true }
func noopCreator(_, _, _ string) error      { return nil }
func noopAdder(_ string) error              { return nil }
func noopAttacher(_ string) error           { return nil }
func noopGHReader(_ string) (string, error) { return "", nil }

func TestBuildShellCmd(t *testing.T) {
	profile := Profile{
		Git: GitConfig{Name: "Test User", Email: "test@example.com"},
	}

	cmd := buildShellCmd(profile, "/home/user/project")

	jtesting.AssertEqual(t, strings.Contains(cmd, `git config user.name "Test User"`), true)
	jtesting.AssertEqual(t, strings.Contains(cmd, `git config user.email "test@example.com"`), true)
	jtesting.AssertEqual(t, strings.Contains(cmd, "claude --dangerously-skip-permissions"), true)
	jtesting.AssertEqual(t, strings.Contains(cmd, " && "), true)
	jtesting.AssertEqual(t, strings.Contains(cmd, ". /home/user/project/.env"), true)
}

func TestBuildShellCmdNoGitConfig(t *testing.T) {
	profile := Profile{}

	cmd := buildShellCmd(profile, "/tmp")

	jtesting.AssertEqual(t, strings.Contains(cmd, "git config"), false)
	jtesting.AssertEqual(t, strings.Contains(cmd, ". /tmp/.env"), true)
	jtesting.AssertEqual(t, strings.Contains(cmd, "exec claude --dangerously-skip-permissions"), true)
}

func TestBuildDotEnv(t *testing.T) {
	content := buildDotEnv("blue", "tok_123", "ghp_abc")
	jtesting.AssertEqual(t, strings.Contains(content, "export JACK_AGENT=blue\n"), true)
	jtesting.AssertEqual(t, strings.Contains(content, "export JACK_MSG_TOKEN=tok_123\n"), true)
	jtesting.AssertEqual(t, strings.Contains(content, "export GH_TOKEN=ghp_abc\n"), true)
}

func TestBuildDotEnvEmpty(t *testing.T) {
	content := buildDotEnv("", "", "")
	jtesting.AssertEqual(t, content, "\n")
}
