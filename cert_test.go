//go:build testing

package jack

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	jtesting "jack.dev/jack/testing"
)

// writeTestCert creates a self-signed cert at the given path with the given expiry.
func writeTestCert(t *testing.T, certFile string, expiry time.Time) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     expiry,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	dir := filepath.Dir(certFile)
	_ = os.MkdirAll(dir, 0o750)
	_ = os.WriteFile(certFile, certPEM, 0o600)
}

func TestCertPath(t *testing.T) {
	env = Env{ConfigDir: "/home/user/.config/jack"}
	jtesting.AssertEqual(t, certPath("blue"), "/home/user/.config/jack/agents/blue/cert.pem")
	jtesting.AssertEqual(t, keyPath("blue"), "/home/user/.config/jack/agents/blue/key.pem")
}

func TestHasCert(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir}

	jtesting.AssertEqual(t, hasCert("blue"), false)

	agentDir := filepath.Join(configDir, "agents", "blue")
	_ = os.MkdirAll(agentDir, 0o750)
	_ = os.WriteFile(filepath.Join(agentDir, "cert.pem"), []byte("x"), 0o600)

	jtesting.AssertEqual(t, hasCert("blue"), true)
}

func TestCertExpiry(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir}

	expiry := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	writeTestCert(t, certPath("blue"), expiry)

	got, err := certExpiry("blue")
	jtesting.AssertNoError(t, err)
	// Allow 1 second tolerance for rounding.
	jtesting.AssertEqual(t, got.Sub(expiry) < time.Second, true)
}

func TestCertNeedsRenewalExpired(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir}

	// Cert that expired an hour ago.
	writeTestCert(t, certPath("blue"), time.Now().Add(-time.Hour))

	jtesting.AssertEqual(t, certNeedsRenewal("blue", time.Hour), true)
}

func TestCertNeedsRenewalValid(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir}

	// Cert valid for another 12 hours.
	writeTestCert(t, certPath("blue"), time.Now().Add(12*time.Hour))

	jtesting.AssertEqual(t, certNeedsRenewal("blue", time.Hour), false)
}

func TestCertNeedsRenewalMissing(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir}

	jtesting.AssertEqual(t, certNeedsRenewal("blue", time.Hour), true)
}

func TestRunRenewSkipsValidCerts(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir}
	cfg = Config{
		Agents: map[string]AgentConfig{"blue": {}},
		CA:     CAConfig{URL: "https://ca.example.com", Provisioner: "jack"},
	}

	// Write a valid cert.
	writeTestCert(t, certPath("blue"), time.Now().Add(12*time.Hour))

	var renewed bool
	renewer := func(_ context.Context, _ string) error {
		renewed = true
		return nil
	}

	err := runRenew(context.Background(), nil, renewer)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, renewed, false)
}

func TestRunRenewRenewsExpiring(t *testing.T) {
	configDir := t.TempDir()
	env = Env{ConfigDir: configDir}
	cfg = Config{
		Agents: map[string]AgentConfig{"blue": {}},
		CA:     CAConfig{URL: "https://ca.example.com", Provisioner: "jack"},
	}

	// Write a cert expiring in 30 minutes (within 1h threshold).
	writeTestCert(t, certPath("blue"), time.Now().Add(30*time.Minute))

	var renewedAgent string
	renewer := func(_ context.Context, agent string) error {
		renewedAgent = agent
		return nil
	}

	err := runRenew(context.Background(), nil, renewer)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, renewedAgent, "blue")
}

func TestRunRenewNoCA(t *testing.T) {
	cfg = Config{
		Agents: map[string]AgentConfig{"blue": {}},
	}

	err := runRenew(context.Background(), nil, nil)
	jtesting.AssertError(t, err)
}
