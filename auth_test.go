//go:build testing

package jack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	jtesting "github.com/zoobzio/jack/testing"
)

func TestRunAuthUnknownTeam(t *testing.T) {
	newTestConfig()
	err := runAuth("bogus")
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "unknown team"), true)
}

func TestGhTokenPathCreatesDir(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir}

	path := ghTokenPath("blue")
	expected := filepath.Join(configDir, "teams", "blue", ".github-token")
	jtesting.AssertEqual(t, path, expected)
}

func TestGhTokenWriteAndRead(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir}

	// Create team dir and write token.
	teamDir := filepath.Join(configDir, "teams", "blue")
	_ = os.MkdirAll(teamDir, 0o750)
	path := ghTokenPath("blue")
	_ = os.WriteFile(path, []byte("ghp_testtoken"), 0o600)

	// Read it back.
	token, err := readGHToken("blue")
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, token, "ghp_testtoken")
}
