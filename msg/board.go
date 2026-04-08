package msg

import (
	"context"
	"fmt"
	"time"
)

func boardAlias(aliasName string) string {
	return "#" + aliasName + ":" + ServerName(Homeserver)
}

// ensureBoardRoom resolves a room by alias, creating it if it doesn't exist.
func ensureBoardRoom(name, topic, aliasName string, resolve AliasResolver, create func(name, topic, aliasName string) (*Room, error)) (string, error) {
	alias := boardAlias(aliasName)
	resp, err := resolve(alias)
	if err == nil {
		return resp.RoomID, nil
	}
	room, err := create(name, topic, aliasName)
	if err != nil {
		return "", fmt.Errorf("creating room: %w", err)
	}
	return room.RoomID, nil
}

// RunBoardPost posts a message to a board room, creating it if needed.
func RunBoardPost(name, topic, aliasName, message string, resolve AliasResolver, send MessageSender, create func(string, string, string) (*Room, error)) error {
	roomID, err := ensureBoardRoom(name, topic, aliasName, resolve, create)
	if err != nil {
		return err
	}
	eventID, err := send(roomID, message)
	if err != nil {
		return err
	}
	fmt.Println(eventID)
	return nil
}

// RunBoardRead reads messages from a board room, creating it if needed.
func RunBoardRead(name, topic, aliasName string, limit int, jsonOut bool, from string, resolve AliasResolver, read MessageReader, create func(string, string, string) (*Room, error)) error {
	roomID, err := ensureBoardRoom(name, topic, aliasName, resolve, create)
	if err != nil {
		return err
	}
	if from != "" {
		return runReadFiltered(roomID, limit, jsonOut, from, read)
	}
	if jsonOut {
		return runReadJSON(roomID, limit, read)
	}
	return runRead(roomID, limit, read)
}

type syncFunc func(ctx context.Context, since string, timeout int, roomID string) (*SyncResponse, error)

func runBoardWatch(name, topic, aliasName string, timeout int, follow bool, resolve AliasResolver, sync syncFunc, create func(string, string, string) (*Room, error)) error {
	roomID, err := ensureBoardRoom(name, topic, aliasName, resolve, create)
	if err != nil {
		return err
	}

	ctx := context.Background()
	if !follow && timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second+5*time.Second)
		defer cancel()
	}

	// Initial sync to get the batch token.
	resp, err := sync(ctx, "", 0, roomID)
	if err != nil {
		return fmt.Errorf("initial sync: %w", err)
	}

	for {
		resp, err = sync(ctx, resp.NextBatch, pollInterval, roomID)
		if err != nil {
			if ctx.Err() != nil && !follow {
				return fmt.Errorf("no new messages within timeout")
			}
			return fmt.Errorf("sync: %w", err)
		}

		room, ok := resp.Rooms.Join[roomID]
		found := false
		if ok {
			for _, m := range room.Timeline.Events {
				if m.Type != msgTypeRoomMessage {
					continue
				}
				found = true
				body, _ := m.Content["body"].(string)
				fmt.Printf("%s: %s\n", m.Sender, body)
			}
		}

		if !follow {
			if found {
				return nil
			}
			if ctx.Err() != nil {
				return fmt.Errorf("no new messages within timeout")
			}
		}
	}
}
