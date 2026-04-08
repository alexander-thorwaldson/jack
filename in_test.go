//go:build testing

package jack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jtesting "jack.dev/jack/testing"
)

func stubRegistry(entries ...RegistryEntry) RegistryLoader {
	return func() (*Registry, error) {
		return &Registry{Projects: entries}, nil
	}
}

func stubAgentSelector(agent string) AgentSelector {
	return func(_ []string) (string, error) { return agent, nil }
}

func stubProjectSelector(project string) ProjectSelector {
	return func(_ string, _ []string) (string, error) { return project, nil }
}

var failSelector AgentSelector = func(_ []string) (string, error) {
	return "", nil
}

var failProjectSelector ProjectSelector = func(_ string, _ []string) (string, error) {
	return "", nil
}

var noopAnnounceRepo RepoAnnouncer = func(_, _, _ string) error { return nil }
var noopContainerRunner ContainerRunner = func(_ string, _ []Mount, _ map[string]string) error { return nil }
var noopContainerExecer ContainerExecer = func(_ string, _ []string) error { return nil }
var noopContainerStopper ContainerStopper = func(_ string) error { return nil }

func TestRunInEmptyRegistry(t *testing.T) {
	err := runIn("", "", stubRegistry(), failSelector, failProjectSelector,
		noopChecker, noopCreator, noopAttacher, noopAdder, noopGHReader, noopAnnounceRepo,
		noopContainerRunner, noopContainerExecer, noopContainerStopper)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "no projects cloned"), true)
}

func TestRunInNoProjectsForAgent(t *testing.T) {
	reg := stubRegistry(RegistryEntry{Agent: "blue", Repo: "vicky"})
	err := runIn("red", "", reg, failSelector, failProjectSelector,
		noopChecker, noopCreator, noopAttacher, noopAdder, noopGHReader, noopAnnounceRepo,
		noopContainerRunner, noopContainerExecer, noopContainerStopper)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "no projects cloned for agent"), true)
}

func TestRunInAttachesExistingSession(t *testing.T) {
	newTestConfig()
	env = Env{DataDir: t.TempDir(), ConfigDir: t.TempDir()}

	reg := stubRegistry(RegistryEntry{Agent: "blue", Repo: "vicky"})

	var attachedName string
	attacher := func(name string) error {
		attachedName = name
		return nil
	}

	err := runIn("blue", "vicky", reg, failSelector, failProjectSelector,
		existsChecker, noopCreator, attacher, noopAdder, noopGHReader, noopAnnounceRepo,
		noopContainerRunner, noopContainerExecer, noopContainerStopper)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, attachedName, "blue-vicky")
}

func TestRunInCreatesAndAttaches(t *testing.T) {
	newTestConfig()
	env = Env{DataDir: t.TempDir(), ConfigDir: t.TempDir()}

	reg := stubRegistry(RegistryEntry{Agent: "blue", Repo: "vicky"})

	var createdName, attachedName string
	creator := func(name, dir, shellCmd string) error {
		createdName = name
		return nil
	}
	attacher := func(name string) error {
		attachedName = name
		return nil
	}

	err := runIn("blue", "vicky", reg, failSelector, failProjectSelector,
		noopChecker, creator, attacher, noopAdder, noopGHReader, noopAnnounceRepo,
		noopContainerRunner, noopContainerExecer, noopContainerStopper)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, createdName, "blue-vicky")
	jtesting.AssertEqual(t, attachedName, "blue-vicky")
}

func TestRunInAutoSelectsSingleAgent(t *testing.T) {
	newTestConfig()
	env = Env{DataDir: t.TempDir(), ConfigDir: t.TempDir()}

	reg := stubRegistry(RegistryEntry{Agent: "blue", Repo: "vicky"})

	var attachedName string
	attacher := func(name string) error {
		attachedName = name
		return nil
	}

	// No agent or project specified — should auto-select the only agent and project.
	err := runIn("", "", reg, failSelector, failProjectSelector,
		noopChecker, noopCreator, attacher, noopAdder, noopGHReader, noopAnnounceRepo,
		noopContainerRunner, noopContainerExecer, noopContainerStopper)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, attachedName, "blue-vicky")
}

func TestRunInPromptsForAgent(t *testing.T) {
	newTestConfig()
	cfg.Profiles["red"] = Profile{Git: GitConfig{Name: "Red"}}
	env = Env{DataDir: t.TempDir(), ConfigDir: t.TempDir()}

	reg := stubRegistry(
		RegistryEntry{Agent: "blue", Repo: "vicky"},
		RegistryEntry{Agent: "red", Repo: "flux"},
	)

	var selectedAgent string
	agentSel := func(agents []string) (string, error) {
		selectedAgent = "red"
		return "red", nil
	}

	err := runIn("", "", reg, agentSel, failProjectSelector,
		noopChecker, noopCreator, noopAttacher, noopAdder, noopGHReader, noopAnnounceRepo,
		noopContainerRunner, noopContainerExecer, noopContainerStopper)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, selectedAgent, "red")
}

func TestRunInPromptsForProject(t *testing.T) {
	newTestConfig()
	env = Env{DataDir: t.TempDir(), ConfigDir: t.TempDir()}

	reg := stubRegistry(
		RegistryEntry{Agent: "blue", Repo: "vicky"},
		RegistryEntry{Agent: "blue", Repo: "flux"},
	)

	var selectedProject string
	projSel := func(_ string, _ []string) (string, error) {
		selectedProject = "flux"
		return "flux", nil
	}

	err := runIn("blue", "", reg, failSelector, projSel,
		noopChecker, noopCreator, noopAttacher, noopAdder, noopGHReader, noopAnnounceRepo,
		noopContainerRunner, noopContainerExecer, noopContainerStopper)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, selectedProject, "flux")
}

func TestRunInReadsToken(t *testing.T) {
	newTestConfig()
	dir := t.TempDir()
	env = Env{DataDir: dir, ConfigDir: t.TempDir()}

	// Create project dir with token.
	projDir := filepath.Join(dir, "blue", "vicky")
	jackDir := filepath.Join(projDir, ".jack")
	_ = os.MkdirAll(jackDir, 0o750)
	_ = os.WriteFile(filepath.Join(jackDir, "token"), []byte("tok_plaintext"), 0o600)

	reg := stubRegistry(RegistryEntry{Agent: "blue", Repo: "vicky"})

	creator := func(_, _, _ string) error { return nil }

	var containerEnv map[string]string
	runner := func(_ string, _ []Mount, env map[string]string) error {
		containerEnv = env
		return nil
	}

	err := runIn("blue", "vicky", reg, failSelector, failProjectSelector,
		noopChecker, creator, noopAttacher, noopAdder, noopGHReader, noopAnnounceRepo,
		runner, noopContainerExecer, noopContainerStopper)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, containerEnv["JACK_MSG_TOKEN"], "tok_plaintext")
}

func TestRunInReadsGHToken(t *testing.T) {
	newTestConfig()
	dir := t.TempDir()
	configDir := t.TempDir()
	env = Env{DataDir: dir, ConfigDir: configDir}

	// Create project dir.
	projDir := filepath.Join(dir, "blue", "vicky")
	_ = os.MkdirAll(projDir, 0o750)

	reg := stubRegistry(RegistryEntry{Agent: "blue", Repo: "vicky"})

	creator := func(_, _, _ string) error { return nil }
	ghReader := func(agent string) (string, error) {
		return "ghp_testtoken", nil
	}

	var containerEnv map[string]string
	runner := func(_ string, _ []Mount, env map[string]string) error {
		containerEnv = env
		return nil
	}

	err := runIn("blue", "vicky", reg, failSelector, failProjectSelector,
		noopChecker, creator, noopAttacher, noopAdder, ghReader, noopAnnounceRepo,
		runner, noopContainerExecer, noopContainerStopper)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, containerEnv["GH_TOKEN"], "ghp_testtoken")
}

func TestRunInUnknownAgentProfile(t *testing.T) {
	newTestConfig()
	env = Env{DataDir: t.TempDir(), ConfigDir: t.TempDir()}

	reg := stubRegistry(RegistryEntry{Agent: "unknown", Repo: "vicky"})

	err := runIn("unknown", "vicky", reg, failSelector, failProjectSelector,
		noopChecker, noopCreator, noopAttacher, noopAdder, noopGHReader, noopAnnounceRepo,
		noopContainerRunner, noopContainerExecer, noopContainerStopper)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "unknown agent"), true)
}

func TestRunInAnnouncesOnRepoChannel(t *testing.T) {
	newTestConfig()
	dir := t.TempDir()
	env = Env{DataDir: dir, ConfigDir: t.TempDir()}

	projDir := filepath.Join(dir, "blue", "vicky")
	jackDir := filepath.Join(projDir, ".jack")
	_ = os.MkdirAll(jackDir, 0o750)
	_ = os.WriteFile(filepath.Join(jackDir, "token"), []byte("tok_session"), 0o600)

	reg := stubRegistry(RegistryEntry{Agent: "blue", Repo: "vicky"})

	var announcedToken, announcedRepo, announcedMsg string
	announcer := func(token, repo, message string) error {
		announcedToken = token
		announcedRepo = repo
		announcedMsg = message
		return nil
	}

	err := runIn("blue", "vicky", reg, failSelector, failProjectSelector,
		noopChecker, noopCreator, noopAttacher, noopAdder, noopGHReader, announcer,
		noopContainerRunner, noopContainerExecer, noopContainerStopper)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, announcedToken, "tok_session")
	jtesting.AssertEqual(t, announcedRepo, "vicky")
	jtesting.AssertEqual(t, strings.Contains(announcedMsg, "jacked in"), true)
}

func TestRunInStartsContainer(t *testing.T) {
	newTestConfig()
	env = Env{DataDir: t.TempDir(), ConfigDir: t.TempDir()}

	reg := stubRegistry(RegistryEntry{Agent: "blue", Repo: "vicky"})

	var containerName string
	runner := func(name string, _ []Mount, _ map[string]string) error {
		containerName = name
		return nil
	}

	var tmuxCmd string
	creator := func(_, _, cmd string) error {
		tmuxCmd = cmd
		return nil
	}

	err := runIn("blue", "vicky", reg, failSelector, failProjectSelector,
		noopChecker, creator, noopAttacher, noopAdder, noopGHReader, noopAnnounceRepo,
		runner, noopContainerExecer, noopContainerStopper)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, containerName, "jack-blue-vicky")
	jtesting.AssertEqual(t, strings.Contains(tmuxCmd, "docker exec -it jack-blue-vicky"), true)
	jtesting.AssertEqual(t, strings.Contains(tmuxCmd, "claude --dangerously-skip-permissions"), true)
}

func TestRunInCleansUpContainerOnTmuxFailure(t *testing.T) {
	newTestConfig()
	env = Env{DataDir: t.TempDir(), ConfigDir: t.TempDir()}

	reg := stubRegistry(RegistryEntry{Agent: "blue", Repo: "vicky"})

	creator := func(_, _, _ string) error {
		return fmt.Errorf("tmux failed")
	}

	var stoppedContainer string
	stopper := func(name string) error {
		stoppedContainer = name
		return nil
	}

	err := runIn("blue", "vicky", reg, failSelector, failProjectSelector,
		noopChecker, creator, noopAttacher, noopAdder, noopGHReader, noopAnnounceRepo,
		noopContainerRunner, noopContainerExecer, stopper)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, stoppedContainer, "jack-blue-vicky")
}
