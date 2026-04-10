//go:build testing

package jack

import (
	"os"
	"path/filepath"
	"testing"

	jtesting "jack.dev/jack/testing"
)

func TestContainerName(t *testing.T) {
	jtesting.AssertEqual(t, ContainerName("blue", "vicky"), "jack-blue-vicky")
	jtesting.AssertEqual(t, ContainerName("red", "flux"), "jack-red-flux")
}

func TestSessionMountsBasic(t *testing.T) {
	c := Config{}
	mounts := SessionMounts(c, "blue", "/home/user/.jack/blue/vicky")

	jtesting.AssertEqual(t, len(mounts), 3)
	jtesting.AssertEqual(t, mounts[0].Target, "/home/jack/.claude")
	jtesting.AssertEqual(t, mounts[0].ReadOnly, false)
	jtesting.AssertEqual(t, mounts[1].Target, "/home/jack/.claude.json")
	jtesting.AssertEqual(t, mounts[1].ReadOnly, false)
	jtesting.AssertEqual(t, mounts[2].Source, "/home/user/.jack/blue/vicky")
	jtesting.AssertEqual(t, mounts[2].Target, "/workspace")
	jtesting.AssertEqual(t, mounts[2].ReadOnly, false)
}

func TestSessionMountsWithSupportingRepos(t *testing.T) {
	dataDir := t.TempDir()
	env = Env{DataDir: dataDir, ConfigDir: t.TempDir()}

	wikiDir := filepath.Join(dataDir, "blue", "wiki")
	_ = os.MkdirAll(wikiDir, 0o750)

	c := Config{
		Agents: map[string]AgentConfig{
			"blue": {Repos: []string{"git@github.com:jackdev/wiki.git"}},
		},
	}
	mounts := SessionMounts(c, "blue", filepath.Join(dataDir, "blue", "vicky"))

	jtesting.AssertEqual(t, len(mounts), 4)
	jtesting.AssertEqual(t, mounts[3].Source, wikiDir)
	jtesting.AssertEqual(t, mounts[3].Target, "/repos/wiki")
	jtesting.AssertEqual(t, mounts[3].ReadOnly, false)
}

func TestSessionEnv(t *testing.T) {
	c := Config{
		Git:    GitConfig{Name: "Blue Bot", Email: "blue@example.com"},
		Matrix: MatrixConfig{Homeserver: "https://matrix.example.com"},
	}
	e := SessionEnv(c, "blue", "tok_123", "ghp_abc")

	jtesting.AssertEqual(t, e["JACK_AGENT"], "blue")
	jtesting.AssertEqual(t, e["JACK_MSG_TOKEN"], "tok_123")
	jtesting.AssertEqual(t, e["GH_TOKEN"], "ghp_abc")
	jtesting.AssertEqual(t, e["GIT_AUTHOR_NAME"], "Blue Bot")
	jtesting.AssertEqual(t, e["GIT_COMMITTER_NAME"], "Blue Bot")
	jtesting.AssertEqual(t, e["GIT_AUTHOR_EMAIL"], "blue@example.com")
	jtesting.AssertEqual(t, e["GIT_COMMITTER_EMAIL"], "blue@example.com")
	jtesting.AssertEqual(t, e["JACK_HOMESERVER"], "https://matrix.example.com")
}

func TestSessionEnvEmpty(t *testing.T) {
	e := SessionEnv(Config{}, "", "", "")
	jtesting.AssertEqual(t, len(e), 0)
}

func TestDockerExecCmd(t *testing.T) {
	cmd := DockerExecCmd("jack-blue-vicky", "exec claude --dangerously-skip-permissions")
	jtesting.AssertEqual(t, cmd, `docker exec -it jack-blue-vicky sh -c "exec claude --dangerously-skip-permissions"`)
}
