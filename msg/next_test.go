//go:build testing

package msg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	jtesting "jack.dev/jack/testing"
)

func TestRunNextFromQueue(t *testing.T) {
	queue := []watchMessage{
		{Type: "message", RoomID: "!r1:localhost", RoomName: "repo", Sender: "@alice:localhost", Body: "first"},
		{Type: "message", RoomID: "!r1:localhost", RoomName: "repo", Sender: "@bob:localhost", Body: "second"},
	}
	var saved []watchMessage
	loadQ := func() []watchMessage { return queue }
	saveQ := func(msgs []watchMessage) error { saved = msgs; return nil }

	// Sync should never be called when queue has messages.
	syncer := func(_ context.Context, _ string, _ int, _ string) (*SyncResponse, error) {
		t.Fatal("sync should not be called")
		return nil, nil
	}

	err := runNext(0, false, syncer, nil, func() string { return "" }, func(_ string) error { return nil }, loadQ, saveQ)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(saved), 1)
	jtesting.AssertEqual(t, saved[0].Body, "second")
}

func TestRunNextFromQueueLastMessage(t *testing.T) {
	queue := []watchMessage{
		{Type: "message", RoomID: "!r1:localhost", RoomName: "repo", Sender: "@alice:localhost", Body: "only"},
	}
	var saved []watchMessage
	loadQ := func() []watchMessage { return queue }
	saveQ := func(msgs []watchMessage) error { saved = msgs; return nil }

	syncer := func(_ context.Context, _ string, _ int, _ string) (*SyncResponse, error) {
		t.Fatal("sync should not be called")
		return nil, nil
	}

	err := runNext(0, false, syncer, nil, func() string { return "" }, func(_ string) error { return nil }, loadQ, saveQ)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(saved), 0)
}

func TestRunNextSyncPending(t *testing.T) {
	syncer := func(_ context.Context, since string, timeout int, _ string) (*SyncResponse, error) {
		jtesting.AssertEqual(t, since, "saved_batch")
		jtesting.AssertEqual(t, timeout, 0)
		return &SyncResponse{
			NextBatch: "batch_2",
			Rooms: SyncRooms{
				Join: map[string]SyncJoinedRoom{
					"!room:localhost": {
						Timeline: SyncTimeline{
							Events: []Message{
								{Sender: "@alice:localhost", Type: "m.room.message", Content: map[string]interface{}{"body": "msg1"}, EventID: "$e1"},
								{Sender: "@bob:localhost", Type: "m.room.message", Content: map[string]interface{}{"body": "msg2"}, EventID: "$e2"},
								{Sender: "@carol:localhost", Type: "m.room.message", Content: map[string]interface{}{"body": "msg3"}, EventID: "$e3"},
							},
						},
					},
				},
			},
		}, nil
	}
	getInfo := func(roomID string) (*RoomInfo, error) {
		return &RoomInfo{Name: "test-room"}, nil
	}
	var savedQueue []watchMessage
	loadQ := func() []watchMessage { return nil }
	saveQ := func(msgs []watchMessage) error { savedQueue = msgs; return nil }
	var savedToken string
	loadToken := func() string { return "saved_batch" }
	saveToken := func(token string) error { savedToken = token; return nil }

	err := runNext(0, false, syncer, getInfo, loadToken, saveToken, loadQ, saveQ)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, savedToken, "batch_2")
	// First message printed, remaining 2 queued.
	jtesting.AssertEqual(t, len(savedQueue), 2)
	jtesting.AssertEqual(t, savedQueue[0].Body, "msg2")
	jtesting.AssertEqual(t, savedQueue[1].Body, "msg3")
}

func TestRunNextSyncSingleMessage(t *testing.T) {
	syncer := func(_ context.Context, since string, _ int, _ string) (*SyncResponse, error) {
		return &SyncResponse{
			NextBatch: "batch_2",
			Rooms: SyncRooms{
				Join: map[string]SyncJoinedRoom{
					"!room:localhost": {
						Timeline: SyncTimeline{
							Events: []Message{
								{Sender: "@alice:localhost", Type: "m.room.message", Content: map[string]interface{}{"body": "only msg"}, EventID: "$e1"},
							},
						},
					},
				},
			},
		}, nil
	}
	var savedQueue []watchMessage
	loadQ := func() []watchMessage { return nil }
	saveQ := func(msgs []watchMessage) error { savedQueue = msgs; return nil }

	err := runNext(0, false, syncer, nil, func() string { return "saved" }, func(_ string) error { return nil }, loadQ, saveQ)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(savedQueue), 0)
}

func TestRunNextNoPendingEntersWatch(t *testing.T) {
	callCount := 0
	syncer := func(_ context.Context, since string, timeout int, _ string) (*SyncResponse, error) {
		callCount++
		if callCount == 1 {
			jtesting.AssertEqual(t, timeout, 0)
			return &SyncResponse{NextBatch: "batch_2"}, nil
		}
		jtesting.AssertEqual(t, timeout, pollInterval)
		return &SyncResponse{
			NextBatch: "batch_3",
			Rooms: SyncRooms{
				Join: map[string]SyncJoinedRoom{
					"!room:localhost": {
						Timeline: SyncTimeline{
							Events: []Message{
								{Sender: "@alice:localhost", Type: "m.room.message", Content: map[string]interface{}{"body": "watched"}},
							},
						},
					},
				},
			},
		}, nil
	}
	loadQ := func() []watchMessage { return nil }
	saveQ := func(_ []watchMessage) error { return nil }

	err := runNext(0, false, syncer, nil, func() string { return "saved" }, func(_ string) error { return nil }, loadQ, saveQ)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, callCount, 2)
}

func TestRunNextStaleTokenFallback(t *testing.T) {
	callCount := 0
	syncer := func(_ context.Context, since string, _ int, _ string) (*SyncResponse, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("M_UNKNOWN_TOKEN")
		}
		if callCount == 2 {
			jtesting.AssertEqual(t, since, "")
			return &SyncResponse{NextBatch: "fresh"}, nil
		}
		return &SyncResponse{
			NextBatch: "batch_2",
			Rooms: SyncRooms{
				Join: map[string]SyncJoinedRoom{
					"!room:localhost": {
						Timeline: SyncTimeline{
							Events: []Message{
								{Sender: "@alice:localhost", Type: "m.room.message", Content: map[string]interface{}{"body": "hi"}},
							},
						},
					},
				},
			},
		}, nil
	}
	loadQ := func() []watchMessage { return nil }
	saveQ := func(_ []watchMessage) error { return nil }

	err := runNext(0, false, syncer, nil, func() string { return "stale" }, func(_ string) error { return nil }, loadQ, saveQ)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, callCount, 3)
}

func TestRunNextInvite(t *testing.T) {
	syncer := func(_ context.Context, _ string, _ int, _ string) (*SyncResponse, error) {
		return &SyncResponse{
			NextBatch: "batch_2",
			Rooms: SyncRooms{
				Invite: map[string]SyncInvitedRoom{
					"!newroom:localhost": {
						InviteState: SyncInviteState{
							Events: []Message{
								{Type: "m.room.name", Content: map[string]interface{}{"name": "planning"}},
								{Type: "m.room.member", Sender: "@alice:localhost", Content: map[string]interface{}{"membership": "invite"}},
							},
						},
					},
				},
			},
		}, nil
	}
	var savedQueue []watchMessage
	loadQ := func() []watchMessage { return nil }
	saveQ := func(msgs []watchMessage) error { savedQueue = msgs; return nil }

	err := runNext(0, false, syncer, nil, func() string { return "saved" }, func(_ string) error { return nil }, loadQ, saveQ)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(savedQueue), 0)
}

func TestLoadSaveQueue(t *testing.T) {
	dir := t.TempDir()
	jackDir := filepath.Join(dir, ".jack")
	_ = os.MkdirAll(jackDir, 0o750)
	t.Chdir(dir)

	// Empty initially.
	msgs := loadQueue()
	jtesting.AssertEqual(t, len(msgs), 0)

	// Save some messages.
	queue := []watchMessage{
		{Type: "message", Sender: "@a:localhost", Body: "one"},
		{Type: "message", Sender: "@b:localhost", Body: "two"},
	}
	err := saveQueue(queue)
	jtesting.AssertNoError(t, err)

	// Load them back.
	msgs = loadQueue()
	jtesting.AssertEqual(t, len(msgs), 2)
	jtesting.AssertEqual(t, msgs[0].Body, "one")
	jtesting.AssertEqual(t, msgs[1].Body, "two")

	// Saving empty removes the file.
	err = saveQueue(nil)
	jtesting.AssertNoError(t, err)
	msgs = loadQueue()
	jtesting.AssertEqual(t, len(msgs), 0)
}
