//go:build testing

package jack

import (
	"testing"

	jtesting "github.com/zoobzio/jack/testing"
)

func TestSessionName(t *testing.T) {
	jtesting.AssertEqual(t, SessionName("blue", "vicky"), "blue-vicky")
	jtesting.AssertEqual(t, SessionName("red", "flux"), "red-flux")
}

func TestParseSessions(t *testing.T) {
	input := "blue-vicky\t1709900000\t1709900060\t/home/user/vicky\t1\t3\n" +
		"personal\t1709800000\t1709800000\t/home/user\t0\t1\n"

	sessions, err := parseSessions(input)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(sessions), 2)
	jtesting.AssertEqual(t, sessions[0].Name, "blue-vicky")
	jtesting.AssertEqual(t, sessions[0].Attached, true)
	jtesting.AssertEqual(t, sessions[0].Windows, 3)
	jtesting.AssertEqual(t, sessions[1].Name, "personal")
	jtesting.AssertEqual(t, sessions[1].Attached, false)
}

func TestParseSessionsEmpty(t *testing.T) {
	sessions, err := parseSessions("")
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(sessions), 0)
}
