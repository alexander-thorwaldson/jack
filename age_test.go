//go:build testing

package jack

import (
	"os"
	"path/filepath"
	"testing"

	jtesting "jack.dev/jack/testing"
)

func TestTokenPath(t *testing.T) {
	jtesting.AssertEqual(t, tokenPath("/home/user/.jack/blue/vicky"), "/home/user/.jack/blue/vicky/.jack/token")
}

func TestWriteAndReadToken(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, ".jack", "token")

	err := writeToken("tok_test123", outPath)
	jtesting.AssertNoError(t, err)

	token, err := readToken(dir)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, token, "tok_test123")
}

func TestReadTokenMissing(t *testing.T) {
	dir := t.TempDir()
	token, err := readToken(dir)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, token, "")
}

func TestGhTokenPath(t *testing.T) {
	env = Env{ConfigDir: "/home/user/.config/jack"}
	jtesting.AssertEqual(t, ghTokenPath("blue"), "/home/user/.config/jack/agents/blue/.github-token")
}

func TestReadGHToken(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir}

	agentDir := filepath.Join(configDir, "agents", "blue")
	_ = os.MkdirAll(agentDir, 0o750)
	_ = os.WriteFile(filepath.Join(agentDir, ".github-token"), []byte("ghp_test123\n"), 0o600)

	token, err := readGHToken("blue")
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, token, "ghp_test123")
}

func TestReadGHTokenMissing(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir}

	token, err := readGHToken("blue")
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, token, "")
}
