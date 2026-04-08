//go:build testing

package jack

import (
	"strings"
	"testing"

	jtesting "github.com/zoobzio/jack/testing"
)

var noopTokenReader TokenReader = func(_, _ string) string { return "" }
var noopAnnouncer DepartureAnnouncer = func(_, _, _ string) error { return nil }

func TestRunOutNotFound(t *testing.T) {
	err := runOut("blue-vicky", "", "", noopChecker, func(_ string) error { return nil }, noopTokenReader, noopAnnouncer)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "not found"), true)
}

func TestRunOutSuccess(t *testing.T) {
	var killed string
	err := runOut("blue-vicky", "", "", existsChecker, func(name string) error {
		killed = name
		return nil
	}, noopTokenReader, noopAnnouncer)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, killed, "blue-vicky")
}

func TestRunOutWithFlags(t *testing.T) {
	var killed string
	err := runOut("", "blue", "vicky", existsChecker, func(name string) error {
		killed = name
		return nil
	}, noopTokenReader, noopAnnouncer)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, killed, "blue-vicky")
}

func TestRunOutMissingArgs(t *testing.T) {
	err := runOut("", "", "", noopChecker, noopKiller, noopTokenReader, noopAnnouncer)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "specify a session name"), true)
}

func TestRunOutPartialFlags(t *testing.T) {
	err := runOut("", "blue", "", noopChecker, noopKiller, noopTokenReader, noopAnnouncer)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "specify a session name"), true)
}

func TestRunOutAnnouncesWithToken(t *testing.T) {
	var announcedToken, announcedRepo, announcedMsg string
	reader := func(agent, project string) string {
		jtesting.AssertEqual(t, agent, "blue")
		jtesting.AssertEqual(t, project, "vicky")
		return "tok_session"
	}
	announcer := func(token, repo, message string) error {
		announcedToken = token
		announcedRepo = repo
		announcedMsg = message
		return nil
	}
	err := runOut("blue-vicky", "", "", existsChecker, func(_ string) error { return nil }, reader, announcer)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, announcedToken, "tok_session")
	jtesting.AssertEqual(t, announcedRepo, "vicky")
	jtesting.AssertEqual(t, strings.Contains(announcedMsg, "jacked out"), true)
}

func TestParseSessionName(t *testing.T) {
	agent, project := parseSessionName("blue-vicky")
	jtesting.AssertEqual(t, agent, "blue")
	jtesting.AssertEqual(t, project, "vicky")

	agent, project = parseSessionName("rockhopper-sentinel")
	jtesting.AssertEqual(t, agent, "rockhopper")
	jtesting.AssertEqual(t, project, "sentinel")
}
