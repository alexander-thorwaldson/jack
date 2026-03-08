//go:build testing

package jack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	jtesting "github.com/zoobzio/jack/testing"
)

func TestApplyAgentUnknownTeam(t *testing.T) {
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

	teamDir := filepath.Join(configDir, "teams", "blue")

	// CLAUDE.md
	_ = os.MkdirAll(teamDir, 0o750)
	_ = os.WriteFile(filepath.Join(teamDir, "CLAUDE.md"), []byte("instructions"), 0o600)

	// Skills.
	teamSkillsDir := filepath.Join(teamDir, "skills")
	_ = os.MkdirAll(filepath.Join(teamSkillsDir, "commit"), 0o750)
	_ = os.WriteFile(filepath.Join(teamSkillsDir, "commit", "SKILL.md"), []byte("commit"), 0o600)
	_ = os.MkdirAll(filepath.Join(teamSkillsDir, "pr"), 0o750)
	_ = os.WriteFile(filepath.Join(teamSkillsDir, "pr", "SKILL.md"), []byte("pr"), 0o600)

	dir := t.TempDir()
	err := applyAgent("blue", dir)
	jtesting.AssertNoError(t, err)

	// CLAUDE.md symlinked.
	claudeLink := filepath.Join(dir, ".claude", "CLAUDE.md")
	target, err := os.Readlink(claudeLink)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, target, filepath.Join(teamDir, "CLAUDE.md"))

	// Skills symlinked.
	commitLink := filepath.Join(dir, ".claude", "commands", "commit")
	target, err = os.Readlink(commitLink)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, target, filepath.Join(teamSkillsDir, "commit"))

	prLink := filepath.Join(dir, ".claude", "commands", "pr")
	target, err = os.Readlink(prLink)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, target, filepath.Join(teamSkillsDir, "pr"))
}

func TestApplyAgentNoSkills(t *testing.T) {
	configDir, _ := filepath.EvalSymlinks(t.TempDir())
	env = Env{ConfigDir: configDir, DataDir: "/tmp/jack"}

	teamDir := filepath.Join(configDir, "teams", "solo")
	_ = os.MkdirAll(filepath.Join(teamDir, "skills"), 0o750)
	_ = os.WriteFile(filepath.Join(teamDir, "CLAUDE.md"), []byte("instructions"), 0o600)

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

func TestValidateTeamMissingDir(t *testing.T) {
	configDir := t.TempDir()
	err := validateTeam(configDir, "blue")
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "team directory not found"), true)
}

func TestValidateTeamMissingClaudeMD(t *testing.T) {
	configDir := t.TempDir()
	teamDir := filepath.Join(configDir, "teams", "blue")
	_ = os.MkdirAll(teamDir, 0o750)
	_ = os.MkdirAll(filepath.Join(teamDir, "skills"), 0o750)

	err := validateTeam(configDir, "blue")
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "CLAUDE.md not found"), true)
}

func TestValidateTeamMissingSkillsDir(t *testing.T) {
	configDir := t.TempDir()
	teamDir := filepath.Join(configDir, "teams", "blue")
	_ = os.MkdirAll(teamDir, 0o750)
	_ = os.WriteFile(filepath.Join(teamDir, "CLAUDE.md"), []byte("x"), 0o600)

	err := validateTeam(configDir, "blue")
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "skills directory not found"), true)
}

func TestValidateTeamSuccess(t *testing.T) {
	configDir := t.TempDir()
	teamDir := filepath.Join(configDir, "teams", "blue")
	_ = os.MkdirAll(filepath.Join(teamDir, "skills"), 0o750)
	_ = os.WriteFile(filepath.Join(teamDir, "CLAUDE.md"), []byte("x"), 0o600)

	err := validateTeam(configDir, "blue")
	jtesting.AssertNoError(t, err)
}
