package msg

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	nextCmd.Flags().Int("timeout", 0, "seconds to wait before exiting (0 = indefinite)")
	nextCmd.Flags().Bool("json", false, "output message as JSON")
	Cmd.AddCommand(nextCmd)
}

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Get the next pending message",
	Long: `Return a single message from the queue.

If messages are buffered locally, the next one is returned immediately.
Otherwise, a Matrix sync is performed. If new messages arrive, the first
is returned and the rest are queued. If no messages are pending, the
command blocks until one arrives.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		timeout, _ := cmd.Flags().GetInt("timeout")
		jsonFlag, _ := cmd.Flags().GetBool("json")
		token, err := TokenFromEnv()
		if err != nil {
			return err
		}
		client := NewClient(Homeserver, token)
		return runNext(timeout, jsonFlag, client.Sync, client.GetRoomInfo, loadSyncToken, saveSyncToken, loadQueue, saveQueue)
	},
}

// queueFile is the filename used to persist the local message queue.
const queueFile = "msg_queue"

// queueLoader reads the local message queue.
type queueLoader func() []watchMessage

// queueSaver persists the local message queue.
type queueSaver func(msgs []watchMessage) error

// loadQueue reads the message queue from .jack/msg_queue, walking up from CWD.
func loadQueue() []watchMessage {
	dir, err := os.Getwd()
	if err != nil {
		return nil
	}
	for {
		path := filepath.Join(dir, ".jack", queueFile)
		data, err := os.ReadFile(filepath.Clean(path))
		if err == nil {
			var msgs []watchMessage
			if json.Unmarshal(data, &msgs) == nil && len(msgs) > 0 {
				return msgs
			}
			return nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}

// saveQueue writes the message queue to the nearest .jack/ directory.
func saveQueue(msgs []watchMessage) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	for {
		jackDir := filepath.Join(dir, ".jack")
		if info, err := os.Stat(jackDir); err == nil && info.IsDir() {
			path := filepath.Join(jackDir, queueFile)
			if len(msgs) == 0 {
				_ = os.Remove(filepath.Clean(path))
				return nil
			}
			data, err := json.Marshal(msgs)
			if err != nil {
				return fmt.Errorf("marshalling queue: %w", err)
			}
			return os.WriteFile(filepath.Clean(path), data, 0o600)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return fmt.Errorf("no .jack directory found")
		}
		dir = parent
	}
}

func runNext(timeout int, jsonOut bool, sync syncFunc, getInfo RoomInfoGetter, loadToken tokenLoader, saveToken tokenSaver, loadQ queueLoader, saveQ queueSaver) error {
	// 1. Check local queue first.
	if queue := loadQ(); len(queue) > 0 {
		printMessages(queue[:1], jsonOut)
		return saveQ(queue[1:])
	}

	// 2. Queue empty — sync from Matrix.
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second+5*time.Second)
		defer cancel()
	}

	saved := loadToken()

	if saved == "" {
		// No saved token — initial sync to get position.
		resp, err := sync(ctx, "", 0, "")
		if err != nil {
			return fmt.Errorf("initial sync: %w", err)
		}
		_ = saveToken(resp.NextBatch)
		saved = resp.NextBatch
	} else {
		// Immediate sync to check for pending messages.
		resp, err := sync(ctx, saved, 0, "")
		if err != nil {
			// Stale token — fall back to fresh sync.
			resp, err = sync(ctx, "", 0, "")
			if err != nil {
				return fmt.Errorf("sync: %w", err)
			}
			_ = saveToken(resp.NextBatch)
			saved = resp.NextBatch
		} else {
			_ = saveToken(resp.NextBatch)
			saved = resp.NextBatch

			msgs := collectSyncMessages(resp, getInfo)
			if len(msgs) > 0 {
				printMessages(msgs[:1], jsonOut)
				return saveQ(msgs[1:])
			}
		}
	}

	// 3. No pending messages — watch until one arrives.
	return nextWatchLoop(ctx, saved, jsonOut, sync, getInfo, saveToken)
}

// nextWatchLoop polls until a single message arrives, prints it, and returns.
func nextWatchLoop(ctx context.Context, since string, jsonOut bool, sync syncFunc, getInfo RoomInfoGetter, save tokenSaver) error {
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

		msgs := collectSyncMessages(resp, getInfo)
		if len(msgs) > 0 {
			printMessages(msgs[:1], jsonOut)
			// Watch mode receives one batch at a time, no need to queue
			// the rest — they'll be in the next sync.
			return nil
		}

		if ctx.Err() != nil {
			return nil
		}
	}
}
