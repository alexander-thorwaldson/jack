package jack

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const keychainService = "Claude Code-credentials"

// syncClaudeCredentials reads Claude OAuth credentials from the macOS
// keychain and writes them to ~/.claude/.credentials.json so that
// containers (which lack keychain access) can authenticate.
// On non-macOS platforms this is a no-op.
func syncClaudeCredentials() error {
	if runtime.GOOS != "darwin" {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home dir: %w", err)
	}

	credPath := filepath.Join(home, ".claude", ".credentials.json")

	// Read from keychain.
	cmd := exec.CommandContext(context.Background(), "security", "find-generic-password", "-s", keychainService, "-w") // #nosec G204 -- constant service name
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reading keychain: %w: %s", err, stderr.String())
	}

	creds := bytes.TrimSpace(stdout.Bytes())
	if len(creds) == 0 {
		return fmt.Errorf("empty credentials in keychain")
	}

	if err := os.WriteFile(credPath, creds, 0o600); err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}

	return nil
}

// credSyncInterval controls how often the credential sidecar re-syncs.
const credSyncInterval = 30 * time.Minute

// startCredentialSync periodically re-syncs Claude OAuth credentials from
// the macOS keychain to disk. The returned cancel function stops the loop.
func startCredentialSync(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(credSyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := syncClaudeCredentials(); err != nil {
					fmt.Fprintf(os.Stderr, "warning: credential sync failed: %v\n", err)
				}
			}
		}
	}()
	return cancel
}
