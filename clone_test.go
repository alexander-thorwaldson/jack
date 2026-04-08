//go:build testing

package jack

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jack.dev/jack/msg"
	jtesting "jack.dev/jack/testing"
)

func TestRepoName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"scp with .git", "git@github.com:jackdev/vicky.git", "vicky"},
		{"scp without .git", "git@github.com:jackdev/vicky", "vicky"},
		{"https with .git", "https://github.com/jackdev/vicky.git", "vicky"},
		{"https without .git", "https://github.com/jackdev/vicky", "vicky"},
		{"ssh protocol", "ssh://git@github.com/jackdev/vicky.git", "vicky"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jtesting.AssertEqual(t, repoName(tt.input), tt.want)
		})
	}
}

func noopCloner(_, _ string) error                     { return nil }
func noopKiller(_ string) error                         { return nil }
func noopTokenWriter(_, _ string) error                 { return nil }
func noopRepoProvisioner(_, _ string, _ []string) error { return nil }
func noopImageBuilder(_ context.Context) error          { return nil }

func noopRegisterer(_, _, _ string) (*msg.Registration, error) {
	return &msg.Registration{AccessToken: "tok_test"}, nil
}

func noopLogin(_, _ string) (*msg.Registration, error) {
	return &msg.Registration{AccessToken: "tok_test"}, nil
}

func noopRegLoader() (*Registry, error) { return &Registry{}, nil }
func noopRegSaver(_ *Registry) error    { return nil }

// setupAgentFixtures creates the agent directories needed for clone validation
// and agent application to pass.
func setupAgentFixtures(t *testing.T, skills []string) {
	t.Helper()
	configDir := t.TempDir()
	dataDir := t.TempDir()
	env = Env{ConfigDir: configDir, DataDir: dataDir}

	for name := range cfg.Profiles {
		agentDir := filepath.Join(configDir, "agents", name)
		agentSkillsDir := filepath.Join(agentDir, "skills")
		_ = os.MkdirAll(agentSkillsDir, 0o750)
		_ = os.WriteFile(filepath.Join(agentDir, "CLAUDE.md"), []byte("x"), 0o600)
		for _, skill := range skills {
			skillDir := filepath.Join(agentSkillsDir, skill)
			_ = os.MkdirAll(skillDir, 0o750)
			_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("x"), 0o600)
		}
	}
}

func TestRunCloneUnknownAgent(t *testing.T) {
	newTestConfig()
	setupAgentFixtures(t, []string{"commit", "pr"})
	err := runClone(context.Background(), "git@github.com:jackdev/vicky.git", []string{"bogus"}, false,
		noopCloner, noopChecker, noopKiller,
		noopRegisterer, noopLogin, noopTokenWriter,
		noopRegLoader, noopRegSaver, noopRepoProvisioner, noopImageBuilder)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "agent directory not found"), true)
}

func TestRunCloneSuccess(t *testing.T) {
	newTestConfig()
	setupAgentFixtures(t, []string{"commit", "pr"})

	var clonedURLs, clonedDirs []string
	cloner := func(url, dir string) error {
		clonedURLs = append(clonedURLs, url)
		clonedDirs = append(clonedDirs, dir)
		return nil
	}

	var savedReg *Registry
	saver := func(r *Registry) error {
		savedReg = r
		return nil
	}

	err := runClone(context.Background(), "git@github.com:jackdev/vicky.git", []string{"blue"}, false,
		cloner, noopChecker, noopKiller,
		noopRegisterer, noopLogin, noopTokenWriter,
		noopRegLoader, saver, noopRepoProvisioner, noopImageBuilder)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(clonedURLs), 1)
	jtesting.AssertEqual(t, clonedURLs[0], "git@github.com:jackdev/vicky.git")
	jtesting.AssertEqual(t, strings.HasSuffix(clonedDirs[0], "blue/vicky"), true)
	jtesting.AssertEqual(t, savedReg != nil, true)
	jtesting.AssertEqual(t, savedReg.Find("blue", "vicky") != nil, true)
}

func TestRunCloneMultipleAgents(t *testing.T) {
	cfg = Config{
		Profiles: map[string]Profile{
			"blue": {Git: GitConfig{Name: "Rockhopper", Email: "rock@example.com"}},
			"red":  {Git: GitConfig{Name: "Mother", Email: "mother@example.com"}},
		},
	}
	setupAgentFixtures(t, []string{"commit"})

	var savedReg *Registry
	saver := func(r *Registry) error {
		savedReg = r
		return nil
	}

	err := runClone(context.Background(), "git@github.com:jackdev/vicky.git", []string{"blue", "red"}, false,
		noopCloner, noopChecker, noopKiller,
		noopRegisterer, noopLogin, noopTokenWriter,
		noopRegLoader, saver, noopRepoProvisioner, noopImageBuilder)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(savedReg.Projects), 2)
	jtesting.AssertEqual(t, savedReg.Find("blue", "vicky") != nil, true)
	jtesting.AssertEqual(t, savedReg.Find("red", "vicky") != nil, true)
}

func TestRunCloneRegistersAndStoresToken(t *testing.T) {
	newTestConfig()
	setupAgentFixtures(t, []string{"commit", "pr"})

	var registeredUsername string
	registerer := func(user, pass, token string) (*msg.Registration, error) {
		registeredUsername = user
		return &msg.Registration{AccessToken: "tok_new"}, nil
	}

	var storedToken, storedPath string
	writer := func(token, outPath string) error {
		storedToken = token
		storedPath = outPath
		return nil
	}

	err := runClone(context.Background(), "git@github.com:jackdev/vicky.git", []string{"blue"}, false,
		noopCloner, noopChecker, noopKiller,
		registerer, noopLogin, writer,
		noopRegLoader, noopRegSaver, noopRepoProvisioner, noopImageBuilder)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, registeredUsername, "blue-vicky")
	jtesting.AssertEqual(t, storedToken, "tok_new")
	jtesting.AssertEqual(t, strings.HasSuffix(storedPath, ".jack/token"), true)
}

func TestRunCloneValidationFailsMissingAgent(t *testing.T) {
	newTestConfig()
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir, DataDir: t.TempDir()}

	err := runClone(context.Background(), "git@github.com:jackdev/vicky.git", []string{"blue"}, false,
		noopCloner, noopChecker, noopKiller,
		noopRegisterer, noopLogin, noopTokenWriter,
		noopRegLoader, noopRegSaver, noopRepoProvisioner, noopImageBuilder)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "agent directory not found"), true)
}

func TestRunCloneSkipsExisting(t *testing.T) {
	newTestConfig()
	setupAgentFixtures(t, []string{"commit"})

	// Pre-create the repo directory to simulate a previous clone.
	dir := filepath.Join(env.dataDir(), "blue", "vicky")
	_ = os.MkdirAll(dir, 0o750)

	var cloned bool
	cloner := func(_, _ string) error {
		cloned = true
		return nil
	}

	err := runClone(context.Background(), "git@github.com:jackdev/vicky.git", []string{"blue"}, false,
		cloner, noopChecker, noopKiller,
		noopRegisterer, noopLogin, noopTokenWriter,
		noopRegLoader, noopRegSaver, noopRepoProvisioner, noopImageBuilder)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, cloned, false)
}

func TestRunCloneForceReplacesExisting(t *testing.T) {
	newTestConfig()
	setupAgentFixtures(t, []string{"commit"})

	// Pre-create the repo directory to simulate a previous clone.
	dir := filepath.Join(env.dataDir(), "blue", "vicky")
	_ = os.MkdirAll(dir, 0o750)

	var cloned bool
	cloner := func(_, _ string) error {
		cloned = true
		return nil
	}

	var killed bool
	killer := func(_ string) error {
		killed = true
		return nil
	}

	hasSession := func(_ string) bool { return true }

	err := runClone(context.Background(), "git@github.com:jackdev/vicky.git", []string{"blue"}, true,
		cloner, hasSession, killer,
		noopRegisterer, noopLogin, noopTokenWriter,
		noopRegLoader, noopRegSaver, noopRepoProvisioner, noopImageBuilder)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, cloned, true)
	jtesting.AssertEqual(t, killed, true)
}
