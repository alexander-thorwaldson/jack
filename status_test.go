//go:build testing

package jack

import (
	"bytes"
	"strings"
	"testing"
	"time"

	jtesting "jack.dev/jack/testing"
)

var noopContainerChecker ContainerChecker = func(_ string) (bool, bool) { return false, false }

func TestRunStatusEmptyRegistry(t *testing.T) {
	var buf bytes.Buffer
	err := runStatus(&buf, stubRegistry(), func() ([]TmuxSession, error) {
		return nil, nil
	}, noopContainerChecker)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, strings.Contains(buf.String(), "no projects cloned"), true)
}

func TestRunStatusNoSessions(t *testing.T) {
	reg := stubRegistry(
		RegistryEntry{Agent: "blue", Repo: "vicky"},
		RegistryEntry{Agent: "red", Repo: "flux"},
	)

	var buf bytes.Buffer
	err := runStatus(&buf, reg, func() ([]TmuxSession, error) {
		return nil, nil
	}, noopContainerChecker)
	jtesting.AssertNoError(t, err)

	output := buf.String()
	jtesting.AssertEqual(t, strings.Contains(output, "blue"), true)
	jtesting.AssertEqual(t, strings.Contains(output, "red"), true)
	jtesting.AssertEqual(t, strings.Contains(output, "not running"), true)
}

func TestRunStatusWithSessions(t *testing.T) {
	reg := stubRegistry(
		RegistryEntry{Agent: "blue", Repo: "vicky"},
		RegistryEntry{Agent: "blue", Repo: "flux"},
		RegistryEntry{Agent: "red", Repo: "sentinel"},
	)

	containerChecker := func(name string) (bool, bool) {
		if name == "jack-blue-vicky" {
			return true, true
		}
		if name == "jack-blue-flux" {
			return false, true
		}
		return false, false
	}

	var buf bytes.Buffer
	err := runStatus(&buf, reg, func() ([]TmuxSession, error) {
		return []TmuxSession{
			{
				Name:     "blue-vicky",
				Created:  time.Now().Add(-time.Hour),
				Activity: time.Now(),
				Path:     "/home/user/vicky",
				Attached: true,
				Windows:  1,
			},
			{
				Name:     "blue-flux",
				Created:  time.Now(),
				Activity: time.Now(),
				Path:     "/home/user/flux",
				Attached: false,
				Windows:  1,
			},
			{
				Name:     "personal",
				Created:  time.Now(),
				Activity: time.Now(),
				Path:     "/home/user",
				Attached: false,
				Windows:  1,
			},
		}, nil
	}, containerChecker)
	jtesting.AssertNoError(t, err)

	output := buf.String()
	// Blue agent projects.
	jtesting.AssertEqual(t, strings.Contains(output, "blue-vicky"), true)
	jtesting.AssertEqual(t, strings.Contains(output, "blue-flux"), true)
	jtesting.AssertEqual(t, strings.Contains(output, "attached"), true)
	jtesting.AssertEqual(t, strings.Contains(output, "active"), true)
	// Container states.
	jtesting.AssertEqual(t, strings.Contains(output, "running"), true)
	jtesting.AssertEqual(t, strings.Contains(output, "stopped"), true)
	// Red agent project not running.
	jtesting.AssertEqual(t, strings.Contains(output, "red"), true)
	jtesting.AssertEqual(t, strings.Contains(output, "not running"), true)
	// Non-jack session filtered out.
	jtesting.AssertEqual(t, strings.Contains(output, "personal"), false)
}
