//go:build testing

package jack

import (
	"bytes"
	"strings"
	"testing"
	"time"

	jtesting "github.com/zoobzio/jack/testing"
)

func TestRunStatusEmptyRegistry(t *testing.T) {
	var buf bytes.Buffer
	err := runStatus(&buf, stubRegistry(), func() ([]TmuxSession, error) {
		return nil, nil
	})
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, strings.Contains(buf.String(), "no projects cloned"), true)
}

func TestRunStatusNoSessions(t *testing.T) {
	reg := stubRegistry(
		RegistryEntry{Team: "blue", Repo: "vicky"},
		RegistryEntry{Team: "red", Repo: "flux"},
	)

	var buf bytes.Buffer
	err := runStatus(&buf, reg, func() ([]TmuxSession, error) {
		return nil, nil
	})
	jtesting.AssertNoError(t, err)

	output := buf.String()
	jtesting.AssertEqual(t, strings.Contains(output, "blue"), true)
	jtesting.AssertEqual(t, strings.Contains(output, "red"), true)
	jtesting.AssertEqual(t, strings.Contains(output, "not running"), true)
}

func TestRunStatusWithSessions(t *testing.T) {
	reg := stubRegistry(
		RegistryEntry{Team: "blue", Repo: "vicky"},
		RegistryEntry{Team: "blue", Repo: "flux"},
		RegistryEntry{Team: "red", Repo: "sentinel"},
	)

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
	})
	jtesting.AssertNoError(t, err)

	output := buf.String()
	// Blue team projects.
	jtesting.AssertEqual(t, strings.Contains(output, "blue-vicky"), true)
	jtesting.AssertEqual(t, strings.Contains(output, "blue-flux"), true)
	jtesting.AssertEqual(t, strings.Contains(output, "attached"), true)
	jtesting.AssertEqual(t, strings.Contains(output, "active"), true)
	// Red team project not running.
	jtesting.AssertEqual(t, strings.Contains(output, "red"), true)
	jtesting.AssertEqual(t, strings.Contains(output, "not running"), true)
	// Non-jack session filtered out.
	jtesting.AssertEqual(t, strings.Contains(output, "personal"), false)
}
