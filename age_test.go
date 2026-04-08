//go:build testing

package jack

import (
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
