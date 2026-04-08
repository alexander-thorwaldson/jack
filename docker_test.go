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
	profile := Profile{}
	mounts := SessionMounts(profile, "blue", "/home/user/.jack/blue/vicky")

	jtesting.AssertEqual(t, len(mounts), 2)
	jtesting.AssertEqual(t, mounts[0].Target, "/root/.claude")
	jtesting.AssertEqual(t, mounts[0].ReadOnly, false)
	jtesting.AssertEqual(t, mounts[1].Source, "/home/user/.jack/blue/vicky")
	jtesting.AssertEqual(t, mounts[1].Target, "/workspace")
	jtesting.AssertEqual(t, mounts[1].ReadOnly, false)
}

func TestSessionMountsWithSSH(t *testing.T) {
	dir := t.TempDir()
	keyPath := dir + "/id_ed25519"
	_ = os.WriteFile(keyPath, []byte("key"), 0o600)
	_ = os.WriteFile(keyPath+".pub", []byte("pub"), 0o600)

	profile := Profile{SSH: SSHConfig{Key: keyPath}}
	mounts := SessionMounts(profile, "blue", "/workspace")

	jtesting.AssertEqual(t, len(mounts), 4)
	jtesting.AssertEqual(t, mounts[2].Source, keyPath)
	jtesting.AssertEqual(t, mounts[2].Target, "/root/.ssh/id_ed25519")
	jtesting.AssertEqual(t, mounts[2].ReadOnly, true)
	jtesting.AssertEqual(t, mounts[3].Source, keyPath+".pub")
	jtesting.AssertEqual(t, mounts[3].Target, "/root/.ssh/id_ed25519.pub")
	jtesting.AssertEqual(t, mounts[3].ReadOnly, true)
}

func TestSessionMountsSSHNoPub(t *testing.T) {
	dir := t.TempDir()
	keyPath := dir + "/id_ed25519"
	_ = os.WriteFile(keyPath, []byte("key"), 0o600)

	profile := Profile{SSH: SSHConfig{Key: keyPath}}
	mounts := SessionMounts(profile, "blue", "/workspace")

	jtesting.AssertEqual(t, len(mounts), 3)
}

func TestSessionMountsWithSupportingRepos(t *testing.T) {
	dataDir := t.TempDir()
	env = Env{DataDir: dataDir, ConfigDir: t.TempDir()}

	// Create a supporting repo directory.
	wikiDir := filepath.Join(dataDir, "blue", "wiki")
	_ = os.MkdirAll(wikiDir, 0o750)

	profile := Profile{
		Repos: []string{"git@github.com:jackdev/wiki.git"},
	}
	mounts := SessionMounts(profile, "blue", filepath.Join(dataDir, "blue", "vicky"))

	// 2 base mounts + 1 supporting repo.
	jtesting.AssertEqual(t, len(mounts), 3)
	jtesting.AssertEqual(t, mounts[2].Source, wikiDir)
	jtesting.AssertEqual(t, mounts[2].Target, "/repos/wiki")
	jtesting.AssertEqual(t, mounts[2].ReadOnly, false)
}

func TestSessionEnv(t *testing.T) {
	profile := Profile{
		Git: GitConfig{Name: "Blue Bot", Email: "blue@example.com"},
	}
	env := SessionEnv("blue", "tok_123", "ghp_abc", "https://matrix.example.com", profile)

	jtesting.AssertEqual(t, env["JACK_AGENT"], "blue")
	jtesting.AssertEqual(t, env["JACK_MSG_TOKEN"], "tok_123")
	jtesting.AssertEqual(t, env["GH_TOKEN"], "ghp_abc")
	jtesting.AssertEqual(t, env["GIT_AUTHOR_NAME"], "Blue Bot")
	jtesting.AssertEqual(t, env["GIT_COMMITTER_NAME"], "Blue Bot")
	jtesting.AssertEqual(t, env["GIT_AUTHOR_EMAIL"], "blue@example.com")
	jtesting.AssertEqual(t, env["GIT_COMMITTER_EMAIL"], "blue@example.com")
	jtesting.AssertEqual(t, env["JACK_HOMESERVER"], "https://matrix.example.com")
}

func TestSessionEnvEmpty(t *testing.T) {
	env := SessionEnv("", "", "", "", Profile{})
	jtesting.AssertEqual(t, len(env), 0)
}

func TestDockerExecCmd(t *testing.T) {
	cmd := DockerExecCmd("jack-blue-vicky", "exec claude --dangerously-skip-permissions")
	jtesting.AssertEqual(t, cmd, `docker exec -it jack-blue-vicky sh -c "exec claude --dangerously-skip-permissions"`)
}
