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

func TestHttpsURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"https passthrough", "https://github.com/org/repo.git", "https://github.com/org/repo.git"},
		{"scp", "git@github.com:org/repo.git", "https://github.com/org/repo.git"},
		{"ssh", "ssh://git@github.com/org/repo.git", "https://github.com/org/repo.git"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jtesting.AssertEqual(t, httpsURL(tt.url), tt.want)
		})
	}
}

func noopCloner(_, _, _ string) error                   { return nil }
func noopKiller(_ string) error                         { return nil }
func noopTokenWriter(_, _ string) error                 { return nil }
func noopRepoProvisioner(_, _ string, _ []string) error { return nil }
func noopImageBuilder(_ context.Context) error          { return nil }
func noopTokenPrompter(_ string) (string, error)        { return "ghp_stub", nil }
func noopGHTokenReader(_ string) (string, error)        { return "ghp_stub", nil }

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

	for name := range cfg.Agents {
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
		noopCloner, noopTokenPrompter, noopGHTokenReader, noopChecker, noopKiller,
		noopRegisterer, noopLogin, noopTokenWriter,
		noopRegLoader, noopRegSaver, noopRepoProvisioner, noopImageBuilder)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "agent directory not found"), true)
}

func TestRunCloneSuccess(t *testing.T) {
	newTestConfig()
	setupAgentFixtures(t, []string{"commit", "pr"})

	var clonedDirs []string
	cloner := func(_, dir, _ string) error {
		clonedDirs = append(clonedDirs, dir)
		return nil
	}

	var savedReg *Registry
	saver := func(r *Registry) error {
		savedReg = r
		return nil
	}

	err := runClone(context.Background(), "git@github.com:jackdev/vicky.git", []string{"blue"}, false,
		cloner, noopTokenPrompter, noopGHTokenReader, noopChecker, noopKiller,
		noopRegisterer, noopLogin, noopTokenWriter,
		noopRegLoader, saver, noopRepoProvisioner, noopImageBuilder)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(clonedDirs), 1)
	jtesting.AssertEqual(t, strings.HasSuffix(clonedDirs[0], "blue/vicky"), true)
	jtesting.AssertEqual(t, savedReg != nil, true)
	jtesting.AssertEqual(t, savedReg.Find("blue", "vicky") != nil, true)
}

func TestRunCloneMultipleAgents(t *testing.T) {
	cfg = Config{
		Agents: map[string]AgentConfig{
			"blue": {},
			"red":  {},
		},
	}
	setupAgentFixtures(t, []string{"commit"})

	var savedReg *Registry
	saver := func(r *Registry) error {
		savedReg = r
		return nil
	}

	err := runClone(context.Background(), "git@github.com:jackdev/vicky.git", []string{"blue", "red"}, false,
		noopCloner, noopTokenPrompter, noopGHTokenReader, noopChecker, noopKiller,
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

	var storedTokens []string
	writer := func(token, _ string) error {
		storedTokens = append(storedTokens, token)
		return nil
	}

	err := runClone(context.Background(), "git@github.com:jackdev/vicky.git", []string{"blue"}, false,
		noopCloner, noopTokenPrompter, noopGHTokenReader, noopChecker, noopKiller,
		registerer, noopLogin, writer,
		noopRegLoader, noopRegSaver, noopRepoProvisioner, noopImageBuilder)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, registeredUsername, "blue-vicky")
	// First stored token is the GH PAT (from prompter fallback), second is the Matrix token.
	jtesting.AssertEqual(t, len(storedTokens) >= 1, true)
}

func TestRunCloneValidationFailsMissingAgent(t *testing.T) {
	newTestConfig()
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir, DataDir: t.TempDir()}

	err := runClone(context.Background(), "git@github.com:jackdev/vicky.git", []string{"blue"}, false,
		noopCloner, noopTokenPrompter, noopGHTokenReader, noopChecker, noopKiller,
		noopRegisterer, noopLogin, noopTokenWriter,
		noopRegLoader, noopRegSaver, noopRepoProvisioner, noopImageBuilder)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "agent directory not found"), true)
}

func TestRunCloneSkipsExisting(t *testing.T) {
	newTestConfig()
	setupAgentFixtures(t, []string{"commit"})

	dir := filepath.Join(env.dataDir(), "blue", "vicky")
	_ = os.MkdirAll(dir, 0o750)

	var cloned bool
	cloner := func(_, _, _ string) error {
		cloned = true
		return nil
	}

	err := runClone(context.Background(), "git@github.com:jackdev/vicky.git", []string{"blue"}, false,
		cloner, noopTokenPrompter, noopGHTokenReader, noopChecker, noopKiller,
		noopRegisterer, noopLogin, noopTokenWriter,
		noopRegLoader, noopRegSaver, noopRepoProvisioner, noopImageBuilder)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, cloned, false)
}

func TestRunCloneForceReplacesExisting(t *testing.T) {
	newTestConfig()
	setupAgentFixtures(t, []string{"commit"})

	dir := filepath.Join(env.dataDir(), "blue", "vicky")
	_ = os.MkdirAll(dir, 0o750)

	var cloned bool
	cloner := func(_, _, _ string) error {
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
		cloner, noopTokenPrompter, noopGHTokenReader, hasSession, killer,
		noopRegisterer, noopLogin, noopTokenWriter,
		noopRegLoader, noopRegSaver, noopRepoProvisioner, noopImageBuilder)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, cloned, true)
	jtesting.AssertEqual(t, killed, true)
}

func TestRunClonePromptsForPATWhenMissing(t *testing.T) {
	newTestConfig()
	setupAgentFixtures(t, []string{"commit"})

	var prompted bool
	prompter := func(repo string) (string, error) {
		prompted = true
		jtesting.AssertEqual(t, repo, "vicky")
		return "ghp_prompted", nil
	}

	// GH reader returns empty — triggers prompt.
	noGH := func(_ string) (string, error) { return "", nil }

	var storedPaths []string
	writer := func(_, path string) error {
		storedPaths = append(storedPaths, path)
		return nil
	}

	err := runClone(context.Background(), "git@github.com:jackdev/vicky.git", []string{"blue"}, false,
		noopCloner, prompter, noGH, noopChecker, noopKiller,
		noopRegisterer, noopLogin, writer,
		noopRegLoader, noopRegSaver, noopRepoProvisioner, noopImageBuilder)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, prompted, true)
	// First store is the PAT, second is the Matrix token.
	jtesting.AssertEqual(t, len(storedPaths) >= 1, true)
	jtesting.AssertEqual(t, strings.Contains(storedPaths[0], ".github-token"), true)
}
