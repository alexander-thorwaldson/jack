package jack

import (
	"os"
	"path/filepath"
	"strings"
)

// GHTokenReader reads a plaintext GitHub token for a project.
type GHTokenReader func(project string) (string, error)

// TokenWriter stores a plaintext token to disk.
type TokenWriter func(token, outPath string) error

func tokenPath(repoDir string) string {
	return filepath.Join(repoDir, ".jack", "token")
}

func writeToken(token, outPath string) error {
	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return os.WriteFile(outPath, []byte(token), 0o600)
}

func readToken(repoDir string) (string, error) {
	data, err := os.ReadFile(tokenPath(repoDir))
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(data)), nil
}

func ghTokenPath(project string) string {
	return filepath.Join(env.configDir(), "repos", project, ".github-token")
}

func readGHToken(project string) (string, error) {
	data, err := os.ReadFile(ghTokenPath(project))
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(data)), nil
}
