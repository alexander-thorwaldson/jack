//go:build testing

package msg

import (
	"context"
	"fmt"
	"testing"

	jtesting "github.com/zoobzio/jack/testing"
)

func TestRunRepoPostSuccess(t *testing.T) {
	Homeserver = "http://localhost:8008"
	var sentRoom, sentMsg string
	sender := func(roomID, message string) (string, error) {
		sentRoom = roomID
		sentMsg = message
		return "$evt1", nil
	}
	name, topic, aliasName := repoTarget("vicky")
	err := runBoardPost(name, topic, aliasName, "PR #1 ready", stubResolver("!repo:localhost"), sender, stubCreator("!repo:localhost"))
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, sentRoom, "!repo:localhost")
	jtesting.AssertEqual(t, sentMsg, "PR #1 ready")
}

func TestRunRepoPostCreatesRoom(t *testing.T) {
	Homeserver = "http://localhost:8008"
	var created bool
	creator := func(name, topic, alias string) (*Room, error) {
		created = true
		return &Room{RoomID: "!new:localhost"}, nil
	}
	sender := func(_, _ string) (string, error) { return "$evt1", nil }
	name, topic, aliasName := repoTarget("vicky")
	err := runBoardPost(name, topic, aliasName, "first post", failResolver(), sender, creator)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, created, true)
}

func TestRunRepoReadSuccess(t *testing.T) {
	Homeserver = "http://localhost:8008"
	reader := func(roomID string, limit int) (*Messages, error) {
		return &Messages{
			Chunk: []Message{
				{Sender: "@blue-vicky:localhost", Type: "m.room.message", Content: map[string]interface{}{"body": "issue #5 opened"}},
			},
		}, nil
	}
	name, topic, aliasName := repoTarget("vicky")
	err := runBoardRead(name, topic, aliasName, 10, false, "", stubResolver("!repo:localhost"), reader, stubCreator("!repo:localhost"))
	jtesting.AssertNoError(t, err)
}

func TestRunRepoWatchSuccess(t *testing.T) {
	Homeserver = "http://localhost:8008"
	callCount := 0
	syncer := func(_ context.Context, since string, timeout int, roomID string) (*SyncResponse, error) {
		callCount++
		if callCount == 1 {
			return &SyncResponse{NextBatch: "batch_1"}, nil
		}
		return &SyncResponse{
			NextBatch: "batch_2",
			Rooms: SyncRooms{
				Join: map[string]SyncJoinedRoom{
					"!repo:localhost": {
						Timeline: SyncTimeline{
							Events: []Message{
								{Sender: "@blue-vicky:localhost", Type: "m.room.message", Content: map[string]interface{}{"body": "PR #2 merged"}},
							},
						},
					},
				},
			},
		}, nil
	}
	name, topic, aliasName := repoTarget("vicky")
	err := runBoardWatch(name, topic, aliasName, 5, false, stubResolver("!repo:localhost"), syncer, stubCreator("!repo:localhost"))
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, callCount, 2)
}

func TestRunRepoWatchNoMessages(t *testing.T) {
	Homeserver = "http://localhost:8008"
	syncer := func(_ context.Context, _ string, _ int, _ string) (*SyncResponse, error) {
		return &SyncResponse{NextBatch: "batch_1"}, nil
	}
	name, topic, aliasName := repoTarget("vicky")
	err := runBoardWatch(name, topic, aliasName, 1, false, stubResolver("!repo:localhost"), syncer, stubCreator("!repo:localhost"))
	jtesting.AssertError(t, err)
}

func TestRepoTarget(t *testing.T) {
	Homeserver = "http://localhost:8008"
	name, topic, aliasName := repoTarget("vicky")
	jtesting.AssertEqual(t, name, "repo-vicky")
	jtesting.AssertEqual(t, topic, "Repo channel for vicky")
	jtesting.AssertEqual(t, aliasName, "repo-vicky")
}

func TestRunRepoWatchFollow(t *testing.T) {
	Homeserver = "http://localhost:8008"
	callCount := 0
	syncer := func(_ context.Context, since string, timeout int, roomID string) (*SyncResponse, error) {
		callCount++
		if callCount == 1 {
			return &SyncResponse{NextBatch: "batch_1"}, nil
		}
		if callCount <= 3 {
			return &SyncResponse{NextBatch: fmt.Sprintf("batch_%d", callCount)}, nil
		}
		return nil, fmt.Errorf("done")
	}
	name, topic, aliasName := repoTarget("vicky")
	err := runBoardWatch(name, topic, aliasName, 1, true, stubResolver("!repo:localhost"), syncer, stubCreator("!repo:localhost"))
	jtesting.AssertError(t, err)
	jtesting.AssertEqual(t, callCount, 4)
}
