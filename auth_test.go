//go:build testing

package jack

import (
	"os"
	"path/filepath"
	"testing"

	jtesting "jack.dev/jack/testing"
)

func TestGhTokenPathUsesRepoDir(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir}

	path := ghTokenPath("vicky")
	expected := filepath.Join(configDir, "repos", "vicky", ".github-token")
	jtesting.AssertEqual(t, path, expected)
}

func TestGhTokenWriteAndRead(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir}

	// Create repo dir and write token.
	repoDir := filepath.Join(configDir, "repos", "vicky")
	_ = os.MkdirAll(repoDir, 0o750)
	path := ghTokenPath("vicky")
	_ = os.WriteFile(path, []byte("ghp_testtoken"), 0o600)

	// Read it back.
	token, err := readGHToken("vicky")
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, token, "ghp_testtoken")
}

func TestReadGHTokenMissing(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir}

	token, err := readGHToken("nonexistent")
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, token, "")
}
