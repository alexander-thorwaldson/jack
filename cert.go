package jack

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// CertIssuer issues a certificate for an agent from the CA.
type CertIssuer func(ctx context.Context, agent string) error

// CertRenewer renews an agent's certificate.
type CertRenewer func(ctx context.Context, agent string) error

// certPath returns the path to an agent's certificate.
func certPath(agent string) string {
	return filepath.Join(env.configDir(), "agents", agent, "cert.pem")
}

// keyPath returns the path to an agent's private key.
func keyPath(agent string) string {
	return filepath.Join(env.configDir(), "agents", agent, "key.pem")
}

// hasCert reports whether an agent has a certificate.
func hasCert(agent string) bool {
	_, err := os.Stat(certPath(agent))
	return err == nil
}

// certExpiry returns the expiry time of an agent's certificate.
func certExpiry(agent string) (time.Time, error) {
	data, err := os.ReadFile(filepath.Clean(certPath(agent))) // #nosec G304 -- path from internal config
	if err != nil {
		return time.Time{}, fmt.Errorf("reading cert: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return time.Time{}, fmt.Errorf("no PEM block found in cert")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing cert: %w", err)
	}
	return cert.NotAfter, nil
}

// certNeedsRenewal reports whether an agent's cert is missing or expires
// within the given threshold.
func certNeedsRenewal(agent string, threshold time.Duration) bool {
	expiry, err := certExpiry(agent)
	if err != nil {
		return true
	}
	return time.Until(expiry) < threshold
}

// issueCert issues a new certificate for an agent using the step CLI.
func issueCert(ctx context.Context, agent string) error {
	if cfg.CA.URL == "" {
		return fmt.Errorf("ca.url not configured")
	}

	certFile := certPath(agent)
	keyFile := keyPath(agent)

	dir := filepath.Dir(certFile)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating agent dir: %w", err)
	}

	args := []string{
		"ca", "certificate",
		agent,
		certFile,
		keyFile,
		"--ca-url", cfg.CA.URL,
		"--provisioner", cfg.CA.Provisioner,
		"--force",
	}
	if cfg.CA.Root != "" {
		args = append(args, "--root", expandHome(cfg.CA.Root))
	}

	cmd := exec.CommandContext(ctx, "step", args...) // #nosec G204 -- args from internal config
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("step ca certificate: %w", err)
	}
	return nil
}

// renewCert renews an agent's certificate using the step CLI.
func renewCert(ctx context.Context, agent string) error {
	if cfg.CA.URL == "" {
		return fmt.Errorf("ca.url not configured")
	}

	certFile := certPath(agent)
	keyFile := keyPath(agent)

	if !hasCert(agent) {
		return issueCert(ctx, agent)
	}

	args := []string{
		"ca", "renew",
		certFile,
		keyFile,
		"--ca-url", cfg.CA.URL,
		"--force",
	}
	if cfg.CA.Root != "" {
		args = append(args, "--root", expandHome(cfg.CA.Root))
	}

	cmd := exec.CommandContext(ctx, "step", args...) // #nosec G204 -- args from internal config
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// If renewal fails (e.g. cert already expired), fall back to issuing a new one.
		return issueCert(ctx, agent)
	}
	return nil
}
