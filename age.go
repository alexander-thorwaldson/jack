package jack

import (
	"os"
	"path/filepath"
	"strings"
)

// GHTokenReader reads a plaintext GitHub token for an agent.
type GHTokenReader func(agent string) (string, error)

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

func ghTokenPath(agentName string) string {
	return filepath.Join(env.configDir(), "agents", agentName, ".github-token")
}

func readGHToken(agent string) (string, error) {
	data, err := os.ReadFile(ghTokenPath(agent))
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(data)), nil
}
