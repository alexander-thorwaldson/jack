//go:build testing

package jack

import (
	"fmt"
	"strings"
	"testing"

	"jack.dev/jack/msg"
	jtesting "jack.dev/jack/testing"
)

func TestRunOpenInvitesAllAgents(t *testing.T) {
	msg.Homeserver = "https://matrix.example.com"

	reg := stubRegistry(
		RegistryEntry{Agent: "blue", Repo: "vicky"},
		RegistryEntry{Agent: "red", Repo: "vicky"},
	)

	var provisionedRepo, provisionedName string
	var invitedIDs []string
	provisioner := func(repo, name string, ids []string) error {
		provisionedRepo = repo
		provisionedName = name
		invitedIDs = ids
		return nil
	}

	err := runOpen("vicky", "review-pr-1", nil, reg, provisioner)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, provisionedRepo, "vicky")
	jtesting.AssertEqual(t, provisionedName, "review-pr-1")
	jtesting.AssertEqual(t, len(invitedIDs), 2)
	// Both agents should be invited.
	joined := strings.Join(invitedIDs, ",")
	jtesting.AssertEqual(t, strings.Contains(joined, "blue-vicky"), true)
	jtesting.AssertEqual(t, strings.Contains(joined, "red-vicky"), true)
}

func TestRunOpenInvitesSpecificAgents(t *testing.T) {
	msg.Homeserver = "https://matrix.example.com"

	reg := stubRegistry(
		RegistryEntry{Agent: "blue", Repo: "vicky"},
		RegistryEntry{Agent: "red", Repo: "vicky"},
	)

	var invitedIDs []string
	provisioner := func(_, _ string, ids []string) error {
		invitedIDs = ids
		return nil
	}

	err := runOpen("vicky", "review-pr-1", []string{"blue"}, reg, provisioner)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(invitedIDs), 1)
	jtesting.AssertEqual(t, strings.Contains(invitedIDs[0], "blue-vicky"), true)
}

func TestRunOpenNoAgents(t *testing.T) {
	reg := stubRegistry()
	provisioner := func(_, _ string, _ []string) error { return nil }

	err := runOpen("vicky", "review-pr-1", nil, reg, provisioner)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "no agents found"), true)
}

func TestRunCloseKicksMembers(t *testing.T) {
	var kickedUsers []string
	var readOnlyRoom string

	resolve := func(_ string) (*msg.AliasResponse, error) {
		return &msg.AliasResponse{RoomID: "!task:localhost"}, nil
	}
	getMembers := func(_ string) ([]string, error) {
		return []string{"@operator:localhost", "@blue-vicky:localhost", "@red-vicky:localhost"}, nil
	}
	whoami := func() (*msg.WhoAmIResponse, error) {
		return &msg.WhoAmIResponse{UserID: "@operator:localhost"}, nil
	}
	kick := func(_, userID, _ string) error {
		kickedUsers = append(kickedUsers, userID)
		return nil
	}
	setReadOnly := func(roomID string) error {
		readOnlyRoom = roomID
		return nil
	}

	err := runClose("vicky", "review-pr-1", resolve, getMembers, whoami, kick, setReadOnly)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(kickedUsers), 2)
	jtesting.AssertEqual(t, kickedUsers[0], "@blue-vicky:localhost")
	jtesting.AssertEqual(t, kickedUsers[1], "@red-vicky:localhost")
	jtesting.AssertEqual(t, readOnlyRoom, "!task:localhost")
}

func TestRunCloseDoesNotKickSelf(t *testing.T) {
	var kickedUsers []string

	resolve := func(_ string) (*msg.AliasResponse, error) {
		return &msg.AliasResponse{RoomID: "!task:localhost"}, nil
	}
	getMembers := func(_ string) ([]string, error) {
		return []string{"@operator:localhost"}, nil
	}
	whoami := func() (*msg.WhoAmIResponse, error) {
		return &msg.WhoAmIResponse{UserID: "@operator:localhost"}, nil
	}
	kick := func(_, userID, _ string) error {
		kickedUsers = append(kickedUsers, userID)
		return nil
	}
	setReadOnly := func(_ string) error { return nil }

	err := runClose("vicky", "review-pr-1", resolve, getMembers, whoami, kick, setReadOnly)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(kickedUsers), 0)
}

func TestRunCloseRoomNotFound(t *testing.T) {
	resolve := func(_ string) (*msg.AliasResponse, error) {
		return nil, fmt.Errorf("not found")
	}
	getMembers := func(_ string) ([]string, error) { return nil, nil }
	whoami := func() (*msg.WhoAmIResponse, error) { return nil, nil }
	kick := func(_, _, _ string) error { return nil }
	setReadOnly := func(_ string) error { return nil }

	err := runClose("vicky", "review-pr-1", resolve, getMembers, whoami, kick, setReadOnly)
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, strings.Contains(err.Error(), "not found"), true)
}

func TestOperatorTokenFromEnv(t *testing.T) {
	t.Setenv("JACK_MSG_TOKEN", "tok_from_env")
	token := operatorToken("vicky")
	jtesting.AssertEqual(t, token, "tok_from_env")
}

func TestOperatorTokenFromRegistry(t *testing.T) {
	t.Setenv("JACK_MSG_TOKEN", "")
	dir := t.TempDir()
	env = Env{DataDir: dir, ConfigDir: t.TempDir()}

	// Set up a registry entry and token file.
	cfg = Config{Agents: map[string]AgentConfig{"blue": {}}}
	reg := &Registry{}
	reg.Add("blue", "vicky", "url")
	_ = saveRegistry(reg)

	projDir := fmt.Sprintf("%s/blue/vicky/.jack", dir)
	_ = writeToken("tok_from_file", projDir+"/token")

	token := operatorToken("vicky")
	jtesting.AssertEqual(t, token, "tok_from_file")
}
