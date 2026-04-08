package msg

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Task room messaging",
	Long:  "Post to and read from task-specific rooms.",
}

var taskPostCmd = &cobra.Command{
	Use:   "post <name> <message...>",
	Short: "Post a message to a task room",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		token, err := TokenFromEnv()
		if err != nil {
			return err
		}
		client := NewClient(Homeserver, token)
		name := args[0]
		message := strings.Join(args[1:], " ")
		roomName, topic, aliasName := TaskTarget(name)
		return RunBoardPost(roomName, topic, aliasName, message, client.ResolveAlias, client.Send, client.CreateRoomWithAlias)
	},
}

var taskReadCmd = &cobra.Command{
	Use:   "read <name>",
	Short: "Read messages from a task room",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		jsonFlag, _ := cmd.Flags().GetBool("json")
		from, _ := cmd.Flags().GetString("from")
		token, err := TokenFromEnv()
		if err != nil {
			return err
		}
		client := NewClient(Homeserver, token)
		name := args[0]
		roomName, topic, aliasName := TaskTarget(name)
		return RunBoardRead(roomName, topic, aliasName, limit, jsonFlag, from, client.ResolveAlias, client.Messages, client.CreateRoomWithAlias)
	},
}

func init() {
	taskReadCmd.Flags().IntP("limit", "n", 20, "number of messages to retrieve")
	taskReadCmd.Flags().Bool("json", false, "output messages as JSON")
	taskReadCmd.Flags().String("from", "", "filter messages by sender username")
	taskCmd.AddCommand(taskPostCmd)
	taskCmd.AddCommand(taskReadCmd)
	Cmd.AddCommand(taskCmd)
}

// TaskTarget returns the room name, topic, and alias for a task room.
func TaskTarget(name string) (roomName, topic, aliasName string) {
	return "task-" + name, fmt.Sprintf("Task room: %s", name), "task-" + name
}

// TaskRoomAlias returns the full Matrix alias for a task room.
func TaskRoomAlias(name string) string {
	return boardAlias("task-" + name)
}

// ProvisionTaskRoom creates a task room for a repo and invites the given users.
func ProvisionTaskRoom(repo, name string, inviteUserIDs []string) error {
	token, err := TokenFromEnv()
	if err != nil {
		return err
	}
	client := NewClient(Homeserver, token)
	taskName := repo + "-" + name
	roomName, topic, aliasName := TaskTarget(taskName)
	roomID, err := ensureBoardRoom(roomName, topic, aliasName, client.ResolveAlias, client.CreateRoomWithAlias)
	if err != nil {
		return fmt.Errorf("provisioning task room: %w", err)
	}
	for _, userID := range inviteUserIDs {
		_ = client.Invite(roomID, userID)
	}
	return nil
}
