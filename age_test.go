//go:build testing

package jack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	jtesting "github.com/zoobzio/jack/testing"
)

func TestTokenAgePath(t *testing.T) {
	jtesting.AssertEqual(t, tokenAgePath("/home/user/.jack/blue/vicky"), "/home/user/.jack/blue/vicky/.jack/token.age")
}



func TestAgeKeyPath(t *testing.T) {
	jtesting.AssertEqual(t, ageKeyPath("/home/user/.jack/blue/vicky"), "/home/user/.jack/blue/vicky/.jack/age.key")
}

func TestGhTokenPath(t *testing.T) {
	env = Env{ConfigDir: "/home/user/.config/jack"}
	jtesting.AssertEqual(t, ghTokenPath("blue"), "/home/user/.config/jack/teams/blue/.github-token")
}

func TestReadAgePublicKey(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "age.key")
	content := "# created: 2024-01-01T00:00:00Z\n# public key: age1testpublickey123\nAGE-SECRET-KEY-1FAKE\n"
	_ = os.WriteFile(keyFile, []byte(content), 0o600)

	pubKey, err := readAgePublicKey(keyFile)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, pubKey, "age1testpublickey123")
}

func TestReadAgePublicKeyMissing(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "age.key")
	content := "# created: 2024-01-01T00:00:00Z\nAGE-SECRET-KEY-1FAKE\n"
	_ = os.WriteFile(keyFile, []byte(content), 0o600)

	_, err := readAgePublicKey(keyFile)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "no public key found"), true)
}

func TestReadGHToken(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir}

	teamDir := filepath.Join(configDir, "teams", "blue")
	_ = os.MkdirAll(teamDir, 0o750)
	_ = os.WriteFile(filepath.Join(teamDir, ".github-token"), []byte("ghp_test123\n"), 0o600)

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
