package msg

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	checkCmd.Flags().Int("timeout", 0, "seconds to watch before exiting (0 = indefinite)")
	checkCmd.Flags().Bool("json", false, "output messages as JSON")
	Cmd.AddCommand(checkCmd)
}

// addCheckFlags registers --check and --check-timeout on a post command.
func addCheckFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("check", false, "run check after posting to see new messages")
	cmd.Flags().Int("check-timeout", 30, "timeout in seconds for check (0 = indefinite)")
}

// postCheck runs check if --check was passed. Call at the end of a post RunE.
func postCheck(cmd *cobra.Command) error {
	check, _ := cmd.Flags().GetBool("check")
	if !check {
		return nil
	}
	timeout, _ := cmd.Flags().GetInt("check-timeout")
	token, err := TokenFromEnv()
	if err != nil {
		return err
	}
	client := NewClient(Homeserver, token)
	return runCheck(timeout, false, client.Sync, client.GetRoomInfo, loadSyncToken, saveSyncToken)
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for pending messages, then watch",
	Long: `Check for messages that arrived since the last check or watch session.

If pending messages exist, they are printed and the command exits.
If no messages are pending, the command enters watch mode and blocks
until a new message arrives in any joined room.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		timeout, _ := cmd.Flags().GetInt("timeout")
		jsonFlag, _ := cmd.Flags().GetBool("json")
		token, err := TokenFromEnv()
		if err != nil {
			return err
		}
		client := NewClient(Homeserver, token)
		return runCheck(timeout, jsonFlag, client.Sync, client.GetRoomInfo, loadSyncToken, saveSyncToken)
	},
}

// syncTokenFile is the filename used to persist the Matrix sync token.
const syncTokenFile = "sync_token"

// tokenLoader reads a persisted sync token.
type tokenLoader func() string

// tokenSaver persists a sync token.
type tokenSaver func(token string) error

// loadSyncToken reads the sync token from .jack/sync_token, walking up from CWD.
func loadSyncToken() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		path := filepath.Join(dir, ".jack", syncTokenFile)
		data, err := os.ReadFile(filepath.Clean(path))
		if err == nil {
			return strings.TrimSpace(string(data))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// saveSyncToken writes the sync token to the nearest .jack/ directory,
// walking up from CWD. If no .jack/ directory is found, it returns an error.
func saveSyncToken(token string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	for {
		jackDir := filepath.Join(dir, ".jack")
		if info, err := os.Stat(jackDir); err == nil && info.IsDir() {
			path := filepath.Join(jackDir, syncTokenFile)
			return os.WriteFile(filepath.Clean(path), []byte(token+"\n"), 0o600)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return fmt.Errorf("no .jack directory found")
		}
		dir = parent
	}
}

func runCheck(timeout int, jsonOut bool, sync syncFunc, getInfo RoomInfoGetter, load tokenLoader, save tokenSaver) error {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second+5*time.Second)
		defer cancel()
	}

	saved := load()

	// If no saved token, do an initial sync to get the current position
	// without dumping historical messages, then fall through to watch.
	if saved == "" {
		resp, err := sync(ctx, "", 0, "")
		if err != nil {
			return fmt.Errorf("initial sync: %w", err)
		}
		_ = save(resp.NextBatch)
		saved = resp.NextBatch
	} else {
		// Immediate sync (timeout=0) to check for pending messages.
		resp, err := sync(ctx, saved, 0, "")
		if err != nil {
			// Token may be stale — fall back to fresh sync.
			resp, err = sync(ctx, "", 0, "")
			if err != nil {
				return fmt.Errorf("sync: %w", err)
			}
			_ = save(resp.NextBatch)
			saved = resp.NextBatch
		} else {
			_ = save(resp.NextBatch)
			saved = resp.NextBatch

			if printSyncMessages(resp, jsonOut, getInfo) {
				return nil
			}
		}
	}

	// No pending messages — enter watch mode.
	return watchLoop(ctx, saved, jsonOut, sync, getInfo, save)
}

// collectSyncMessages extracts messages and invites from a sync response
// into a flat slice of watchMessage structs.
func collectSyncMessages(resp *SyncResponse, getInfo RoomInfoGetter) []watchMessage {
	roomNames := map[string]string{}
	lookupName := func(roomID string) string {
		if name, ok := roomNames[roomID]; ok {
			return name
		}
		if getInfo != nil {
			if info, err := getInfo(roomID); err == nil && info.Name != "" {
				roomNames[roomID] = info.Name
				return info.Name
			}
		}
		roomNames[roomID] = roomID
		return roomID
	}

	var msgs []watchMessage
	for roomID, room := range resp.Rooms.Join {
		for _, m := range room.Timeline.Events {
			if m.Type != msgTypeRoomMessage {
				continue
			}
			body, _ := m.Content["body"].(string)
			msgs = append(msgs, watchMessage{
				Type:     "message",
				RoomID:   roomID,
				RoomName: lookupName(roomID),
				Sender:   m.Sender,
				Body:     body,
				EventID:  m.EventID,
			})
		}
	}

	for _, inv := range parseInvites(resp.Rooms.Invite) {
		name := inv.Name
		if name == "" {
			name = inv.RoomID
		}
		sender := inv.Sender
		if sender == "" {
			sender = unknownPlaceholder
		}
		msgs = append(msgs, watchMessage{
			Type:     "invite",
			RoomID:   inv.RoomID,
			RoomName: name,
			Sender:   sender,
			Body:     fmt.Sprintf("invited you to %s", name),
		})
	}

	return msgs
}

// printSyncMessages prints messages and invites from a sync response.
// Returns true if any messages were found.
func printSyncMessages(resp *SyncResponse, jsonOut bool, getInfo RoomInfoGetter) bool {
	msgs := collectSyncMessages(resp, getInfo)
	if len(msgs) == 0 {
		return false
	}
	printMessages(msgs, jsonOut)
	return true
}

// printMessages prints a slice of watchMessages to stdout.
func printMessages(msgs []watchMessage, jsonOut bool) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	for _, m := range msgs {
		if jsonOut {
			_ = enc.Encode(m)
		} else if m.Type == "invite" {
			fmt.Printf("[invite] %s %s\n", m.Sender, m.Body)
		} else {
			fmt.Printf("[%s] %s: %s\n", m.RoomName, m.Sender, m.Body)
		}
	}
}

// watchLoop is the long-poll sync loop used when no pending messages were found.
func watchLoop(ctx context.Context, since string, jsonOut bool, sync syncFunc, getInfo RoomInfoGetter, save tokenSaver) error {
	for {
		resp, err := sync(ctx, since, pollInterval, "")
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("sync: %w", err)
		}
		_ = save(resp.NextBatch)
		since = resp.NextBatch

		if printSyncMessages(resp, jsonOut, getInfo) {
			return nil
		}

		if ctx.Err() != nil {
			return nil
		}
	}
}
