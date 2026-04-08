//go:build testing

package jack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	jtesting "github.com/zoobzio/jack/testing"
)

func TestApplyAgentUnknownAgent(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir, DataDir: "/tmp/jack"}
	err := applyAgent("bogus", t.TempDir())
	jtesting.AssertError(t, err)
}

func TestApplyAgentSuccess(t *testing.T) {
	// Resolve symlinks so paths match EvalSymlinks in applyAgent (e.g. macOS
	// /var -> /private/var).
	configDir, _ := filepath.EvalSymlinks(t.TempDir())
	env = Env{ConfigDir: configDir, DataDir: "/tmp/jack"}

	agentDir := filepath.Join(configDir, "agents", "blue")

	// CLAUDE.md
	_ = os.MkdirAll(agentDir, 0o750)
	_ = os.WriteFile(filepath.Join(agentDir, "CLAUDE.md"), []byte("instructions"), 0o600)

	// Skills.
	agentSkillsDir := filepath.Join(agentDir, "skills")
	_ = os.MkdirAll(filepath.Join(agentSkillsDir, "commit"), 0o750)
	_ = os.WriteFile(filepath.Join(agentSkillsDir, "commit", "SKILL.md"), []byte("commit"), 0o600)
	_ = os.MkdirAll(filepath.Join(agentSkillsDir, "pr"), 0o750)
	_ = os.WriteFile(filepath.Join(agentSkillsDir, "pr", "SKILL.md"), []byte("pr"), 0o600)

	dir := t.TempDir()
	err := applyAgent("blue", dir)
	jtesting.AssertNoError(t, err)

	// CLAUDE.md symlinked.
	claudeLink := filepath.Join(dir, ".claude", "CLAUDE.md")
	target, err := os.Readlink(claudeLink)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, target, filepath.Join(agentDir, "CLAUDE.md"))

	// Skills symlinked.
	commitLink := filepath.Join(dir, ".claude", "commands", "commit")
	target, err = os.Readlink(commitLink)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, target, filepath.Join(agentSkillsDir, "commit"))

	prLink := filepath.Join(dir, ".claude", "commands", "pr")
	target, err = os.Readlink(prLink)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, target, filepath.Join(agentSkillsDir, "pr"))
}

func TestApplyAgentNoSkills(t *testing.T) {
	configDir, _ := filepath.EvalSymlinks(t.TempDir())
	env = Env{ConfigDir: configDir, DataDir: "/tmp/jack"}

	agentDir := filepath.Join(configDir, "agents", "solo")
	_ = os.MkdirAll(filepath.Join(agentDir, "skills"), 0o750)
	_ = os.WriteFile(filepath.Join(agentDir, "CLAUDE.md"), []byte("instructions"), 0o600)

	dir := t.TempDir()
	err := applyAgent("solo", dir)
	jtesting.AssertNoError(t, err)

	// CLAUDE.md exists.
	claudeLink := filepath.Join(dir, ".claude", "CLAUDE.md")
	_, err = os.Readlink(claudeLink)
	jtesting.AssertNoError(t, err)

	// No commands dir created.
	_, err = os.Stat(filepath.Join(dir, ".claude", "commands"))
	jtesting.AssertEqual(t, os.IsNotExist(err), true)
}

func TestValidateAgentMissingDir(t *testing.T) {
	configDir := t.TempDir()
	err := validateAgent(configDir, "blue")
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "agent directory not found"), true)
}

func TestValidateAgentMissingClaudeMD(t *testing.T) {
	configDir := t.TempDir()
	agentDir := filepath.Join(configDir, "agents", "blue")
	_ = os.MkdirAll(agentDir, 0o750)
	_ = os.MkdirAll(filepath.Join(agentDir, "skills"), 0o750)

	err := validateAgent(configDir, "blue")
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "CLAUDE.md not found"), true)
}

func TestValidateAgentMissingSkillsDir(t *testing.T) {
	configDir := t.TempDir()
	agentDir := filepath.Join(configDir, "agents", "blue")
	_ = os.MkdirAll(agentDir, 0o750)
	_ = os.WriteFile(filepath.Join(agentDir, "CLAUDE.md"), []byte("x"), 0o600)

	err := validateAgent(configDir, "blue")
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "skills directory not found"), true)
}

func TestValidateAgentSuccess(t *testing.T) {
	configDir := t.TempDir()
	agentDir := filepath.Join(configDir, "agents", "blue")
	_ = os.MkdirAll(filepath.Join(agentDir, "skills"), 0o750)
	_ = os.WriteFile(filepath.Join(agentDir, "CLAUDE.md"), []byte("x"), 0o600)

	err := validateAgent(configDir, "blue")
	jtesting.AssertNoError(t, err)
}
