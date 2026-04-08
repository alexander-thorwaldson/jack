//go:build testing

package jack

import (
	"testing"

	jtesting "jack.dev/jack/testing"
)

// Shared test helpers used across multiple test files.

func newTestConfig() {
	cfg = Config{
		Profiles: map[string]Profile{
			"blue": {
				Git: GitConfig{Name: "Rockhopper", Email: "rock@example.com"},
			},
		},
	}
}

func noopChecker(string) bool              { return false }
func existsChecker(string) bool            { return true }
func noopCreator(_, _, _ string) error      { return nil }
func noopAdder(_ string) error              { return nil }
func noopAttacher(_ string) error           { return nil }
func noopGHReader(_ string) (string, error) { return "", nil }

func TestBuildContainerShellCmd(t *testing.T) {
	cmd := buildContainerShellCmd()
	jtesting.AssertEqual(t, cmd, "exec claude --dangerously-skip-permissions")
}
