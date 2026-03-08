//go:build testing

package jack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	jtesting "github.com/zoobzio/jack/testing"
)

func stubRegistry(entries ...RegistryEntry) RegistryLoader {
	return func() (*Registry, error) {
		return &Registry{Projects: entries}, nil
	}
}

func stubTeamSelector(team string) TeamSelector {
	return func(_ []string) (string, error) { return team, nil }
}

func stubProjectSelector(project string) ProjectSelector {
	return func(_ string, _ []string) (string, error) { return project, nil }
}

var failSelector TeamSelector = func(_ []string) (string, error) {
	return "", nil
}

var failProjectSelector ProjectSelector = func(_ string, _ []string) (string, error) {
	return "", nil
}

var noopAnnounceRepo RepoAnnouncer = func(_, _, _ string) error { return nil }

func TestRunInEmptyRegistry(t *testing.T) {
	err := runIn("", "", stubRegistry(), failSelector, failProjectSelector,
		noopChecker, noopCreator, noopAttacher, noopAdder, noopDecrypter, noopGHReader, noopAnnounceRepo)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "no projects cloned"), true)
}

func TestRunInNoProjectsForTeam(t *testing.T) {
	reg := stubRegistry(RegistryEntry{Team: "blue", Repo: "vicky"})
	err := runIn("red", "", reg, failSelector, failProjectSelector,
		noopChecker, noopCreator, noopAttacher, noopAdder, noopDecrypter, noopGHReader, noopAnnounceRepo)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "no projects cloned for team"), true)
}

func TestRunInAttachesExistingSession(t *testing.T) {
	newTestConfig()
	env = Env{DataDir: t.TempDir(), ConfigDir: t.TempDir()}

	reg := stubRegistry(RegistryEntry{Team: "blue", Repo: "vicky"})

	var attachedName string
	attacher := func(name string) error {
		attachedName = name
		return nil
	}

	err := runIn("blue", "vicky", reg, failSelector, failProjectSelector,
		existsChecker, noopCreator, attacher, noopAdder, noopDecrypter, noopGHReader, noopAnnounceRepo)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, attachedName, "blue-vicky")
}

func TestRunInCreatesAndAttaches(t *testing.T) {
	newTestConfig()
	env = Env{DataDir: t.TempDir(), ConfigDir: t.TempDir()}

	reg := stubRegistry(RegistryEntry{Team: "blue", Repo: "vicky"})

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
		noopChecker, creator, attacher, noopAdder, noopDecrypter, noopGHReader, noopAnnounceRepo)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, createdName, "blue-vicky")
	jtesting.AssertEqual(t, attachedName, "blue-vicky")
}

func TestRunInAutoSelectsSingleTeam(t *testing.T) {
	newTestConfig()
	env = Env{DataDir: t.TempDir(), ConfigDir: t.TempDir()}

	reg := stubRegistry(RegistryEntry{Team: "blue", Repo: "vicky"})

	var attachedName string
	attacher := func(name string) error {
		attachedName = name
		return nil
	}

	// No team or project specified — should auto-select the only team and project.
	err := runIn("", "", reg, failSelector, failProjectSelector,
		noopChecker, noopCreator, attacher, noopAdder, noopDecrypter, noopGHReader, noopAnnounceRepo)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, attachedName, "blue-vicky")
}

func TestRunInPromptsForTeam(t *testing.T) {
	newTestConfig()
	cfg.Profiles["red"] = Profile{Git: GitConfig{Name: "Red"}}
	env = Env{DataDir: t.TempDir(), ConfigDir: t.TempDir()}

	reg := stubRegistry(
		RegistryEntry{Team: "blue", Repo: "vicky"},
		RegistryEntry{Team: "red", Repo: "flux"},
	)

	var selectedTeam string
	teamSel := func(teams []string) (string, error) {
		selectedTeam = "red"
		return "red", nil
	}

	err := runIn("", "", reg, teamSel, failProjectSelector,
		noopChecker, noopCreator, noopAttacher, noopAdder, noopDecrypter, noopGHReader, noopAnnounceRepo)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, selectedTeam, "red")
}

func TestRunInPromptsForProject(t *testing.T) {
	newTestConfig()
	env = Env{DataDir: t.TempDir(), ConfigDir: t.TempDir()}

	reg := stubRegistry(
		RegistryEntry{Team: "blue", Repo: "vicky"},
		RegistryEntry{Team: "blue", Repo: "flux"},
	)

	var selectedProject string
	projSel := func(_ string, _ []string) (string, error) {
		selectedProject = "flux"
		return "flux", nil
	}

	err := runIn("blue", "", reg, failSelector, projSel,
		noopChecker, noopCreator, noopAttacher, noopAdder, noopDecrypter, noopGHReader, noopAnnounceRepo)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, selectedProject, "flux")
}

func TestRunInDecryptsToken(t *testing.T) {
	newTestConfig()
	dir := t.TempDir()
	env = Env{DataDir: dir, ConfigDir: t.TempDir()}

	// Create project dir with token.
	projDir := filepath.Join(dir, "blue", "vicky")
	jackDir := filepath.Join(projDir, ".jack")
	_ = os.MkdirAll(jackDir, 0o750)
	_ = os.WriteFile(filepath.Join(jackDir, "token.age"), []byte("encrypted"), 0o600)

	reg := stubRegistry(RegistryEntry{Team: "blue", Repo: "vicky"})

	creator := func(_, _, _ string) error { return nil }
	decrypter := func(_, _ string) (string, error) {
		return "tok_decrypted", nil
	}

	err := runIn("blue", "vicky", reg, failSelector, failProjectSelector,
		noopChecker, creator, noopAttacher, noopAdder, decrypter, noopGHReader, noopAnnounceRepo)
	jtesting.AssertNoError(t, err)

	// The .env file is written at the project root — verify the token is in it.
	dotEnvPath := filepath.Join(projDir, ".env")
	dotEnvContent, err := os.ReadFile(dotEnvPath)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, strings.Contains(string(dotEnvContent), "export JACK_MSG_TOKEN=tok_decrypted"), true)
}

func TestRunInReadsGHToken(t *testing.T) {
	newTestConfig()
	dir := t.TempDir()
	configDir := t.TempDir()
	env = Env{DataDir: dir, ConfigDir: configDir}

	// Create project dir.
	projDir := filepath.Join(dir, "blue", "vicky")
	_ = os.MkdirAll(projDir, 0o750)

	reg := stubRegistry(RegistryEntry{Team: "blue", Repo: "vicky"})

	creator := func(_, _, _ string) error { return nil }
	ghReader := func(team string) (string, error) {
		return "ghp_testtoken", nil
	}

	err := runIn("blue", "vicky", reg, failSelector, failProjectSelector,
		noopChecker, creator, noopAttacher, noopAdder, noopDecrypter, ghReader, noopAnnounceRepo)
	jtesting.AssertNoError(t, err)

	// Verify GH_TOKEN is in the .env file.
	dotEnvPath := filepath.Join(projDir, ".env")
	dotEnvContent, err := os.ReadFile(dotEnvPath)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, strings.Contains(string(dotEnvContent), "export GH_TOKEN=ghp_testtoken"), true)
}

func TestRunInUnknownTeamProfile(t *testing.T) {
	newTestConfig()
	env = Env{DataDir: t.TempDir(), ConfigDir: t.TempDir()}

	reg := stubRegistry(RegistryEntry{Team: "unknown", Repo: "vicky"})

	err := runIn("unknown", "vicky", reg, failSelector, failProjectSelector,
		noopChecker, noopCreator, noopAttacher, noopAdder, noopDecrypter, noopGHReader, noopAnnounceRepo)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "unknown team"), true)
}

func TestRunInAnnouncesOnRepoChannel(t *testing.T) {
	newTestConfig()
	dir := t.TempDir()
	env = Env{DataDir: dir, ConfigDir: t.TempDir()}

	projDir := filepath.Join(dir, "blue", "vicky")
	jackDir := filepath.Join(projDir, ".jack")
	_ = os.MkdirAll(jackDir, 0o750)
	_ = os.WriteFile(filepath.Join(jackDir, "token.age"), []byte("encrypted"), 0o600)

	reg := stubRegistry(RegistryEntry{Team: "blue", Repo: "vicky"})

	var announcedToken, announcedRepo, announcedMsg string
	announcer := func(token, repo, message string) error {
		announcedToken = token
		announcedRepo = repo
		announcedMsg = message
		return nil
	}
	decrypter := func(_, _ string) (string, error) {
		return "tok_session", nil
	}

	err := runIn("blue", "vicky", reg, failSelector, failProjectSelector,
		noopChecker, noopCreator, noopAttacher, noopAdder, decrypter, noopGHReader, announcer)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, announcedToken, "tok_session")
	jtesting.AssertEqual(t, announcedRepo, "vicky")
	jtesting.AssertEqual(t, strings.Contains(announcedMsg, "jacked in"), true)
}
