package jack

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AgeKeypair holds a generated age identity.
type AgeKeypair struct {
	PrivKeyPath string // path to the private key file
	PublicKey   string // the age1... public key string
}

// KeypairGenerator generates an age keypair and writes it to disk.
type KeypairGenerator func(outPath string) (*AgeKeypair, error)

// TokenEncrypterByRecipient encrypts a plaintext token using an age public key
// string and writes the ciphertext to the given path.
type TokenEncrypterByRecipient func(token, recipient, outPath string) error

// TokenDecrypter decrypts an age-encrypted file using a private key and
// returns the plaintext token.
type TokenDecrypter func(privKeyPath, agePath string) (string, error)

// GHTokenReader reads a plaintext GitHub token for an agent.
type GHTokenReader func(agent string) (string, error)


func ageKeygen(outPath string) (*AgeKeypair, error) {
	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("creating directory %s: %w", dir, err)
	}
	cmd := exec.CommandContext(context.Background(), "age-keygen", "-o", outPath) // #nosec G204 -- path from internal helper
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("age-keygen: %w", err)
	}
	pubKey, err := readAgePublicKey(outPath)
	if err != nil {
		return nil, err
	}
	return &AgeKeypair{PrivKeyPath: outPath, PublicKey: pubKey}, nil
}

// readAgePublicKey extracts the public key from an age private key file.
// The file contains a comment line like: # public key: age1...
func readAgePublicKey(path string) (string, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("opening age key: %w", err)
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# public key: ") {
			return strings.TrimPrefix(line, "# public key: "), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading age key: %w", err)
	}
	return "", fmt.Errorf("no public key found in %s", path)
}

func ageEncryptToRecipient(token, recipient, outPath string) error {
	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	cmd := exec.CommandContext(context.Background(), "age", "-r", recipient, "-o", outPath) // #nosec G204 -- args from internal paths
	cmd.Stdin = strings.NewReader(token)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ageDecrypt(privKeyPath, agePath string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "age", "-d", "-i", privKeyPath, agePath) // #nosec G204 -- args from internal paths
	var stdout bytes.Buffer
	cmd.Stdin = os.Stdin
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("age decrypt: %w", err)
	}
	return strings.TrimSpace(stdout.String()), nil
}


func tokenAgePath(repoDir string) string {
	return filepath.Join(repoDir, ".jack", "token.age")
}

func ageKeyPath(repoDir string) string {
	return filepath.Join(repoDir, ".jack", "age.key")
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
