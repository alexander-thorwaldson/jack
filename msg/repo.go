package msg

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Repo channel messaging",
	Long:  "Post to, read from, and watch a per-repo channel.",
}

var repoPostCmd = &cobra.Command{
	Use:   "post <repo> <message...>",
	Short: "Post a message to a repo channel",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := TokenFromEnv()
		if err != nil {
			return err
		}
		client := NewClient(Homeserver, token)
		repo := args[0]
		message := strings.Join(args[1:], " ")
		name, topic, aliasName := repoTarget(repo)
		if err := runBoardPost(name, topic, aliasName, message, client.ResolveAlias, client.Send, client.CreateRoomWithAlias); err != nil {
			return err
		}
		return postCheck(cmd)
	},
}

var repoReadCmd = &cobra.Command{
	Use:   "read <repo>",
	Short: "Read messages from a repo channel",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		jsonFlag, _ := cmd.Flags().GetBool("json")
		since, _ := cmd.Flags().GetString("since")
		from, _ := cmd.Flags().GetString("from")
		token, err := TokenFromEnv()
		if err != nil {
			return err
		}
		client := NewClient(Homeserver, token)
		repo := args[0]
		name, topic, aliasName := repoTarget(repo)
		if since != "" {
			roomID, err := ensureBoardRoom(name, topic, aliasName, client.ResolveAlias, client.CreateRoomWithAlias)
			if err != nil {
				return err
			}
			return runReadSince(roomID, since, limit, jsonFlag, client.EventContext, client.MessagesFrom)
		}
		return runBoardRead(name, topic, aliasName, limit, jsonFlag, from, client.ResolveAlias, client.Messages, client.CreateRoomWithAlias)
	},
}

var repoWatchCmd = &cobra.Command{
	Use:   "watch <repo>",
	Short: "Watch a repo channel for new messages",
	Long:  "Block until a new message arrives on the repo channel, print it, and exit. Use --follow to stream continuously.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		timeout, _ := cmd.Flags().GetInt("timeout")
		follow, _ := cmd.Flags().GetBool("follow")
		token, err := TokenFromEnv()
		if err != nil {
			return err
		}
		client := NewClient(Homeserver, token)
		repo := args[0]
		name, topic, aliasName := repoTarget(repo)
		return runBoardWatch(name, topic, aliasName, timeout, follow, client.ResolveAlias, client.Sync, client.CreateRoomWithAlias)
	},
}

func init() {
	repoReadCmd.Flags().IntP("limit", "n", 20, "number of messages to retrieve")
	repoReadCmd.Flags().Bool("json", false, "output messages as JSON")
	repoReadCmd.Flags().String("since", "", "show messages after this event ID")
	repoReadCmd.Flags().String("from", "", "filter messages by sender username")
	repoWatchCmd.Flags().Int("timeout", 30, "seconds to wait before giving up")
	repoWatchCmd.Flags().BoolP("follow", "f", false, "stream messages continuously")
	addCheckFlags(repoPostCmd)
	repoCmd.AddCommand(repoPostCmd)
	repoCmd.AddCommand(repoReadCmd)
	repoCmd.AddCommand(repoWatchCmd)
	Cmd.AddCommand(repoCmd)
}

func repoTarget(repo string) (name, topic, aliasName string) {
	return "repo-" + repo, fmt.Sprintf("Repo channel for %s", repo), "repo-" + repo
}

// ProvisionRepoChannel ensures the per-repo channel exists, joins the current
// user, and invites the given user IDs. Invite errors are non-fatal since users
// may already be members.
func ProvisionRepoChannel(token, repo string, inviteUserIDs []string) error {
	client := NewClient(Homeserver, token)
	name, topic, aliasName := repoTarget(repo)
	roomID, err := ensureBoardRoom(name, topic, aliasName, client.ResolveAlias, client.CreateRoomWithAlias)
	if err != nil {
		return fmt.Errorf("provisioning repo channel: %w", err)
	}
	if _, err := client.Join(roomID); err != nil {
		return fmt.Errorf("joining repo channel: %w", err)
	}
	for _, userID := range inviteUserIDs {
		_ = client.Invite(roomID, userID)
	}
	return nil
}

// AnnounceOnRepoChannel posts a message to the per-repo channel.
func AnnounceOnRepoChannel(token, repo, message string) error {
	client := NewClient(Homeserver, token)
	name, topic, aliasName := repoTarget(repo)
	roomID, err := ensureBoardRoom(name, topic, aliasName, client.ResolveAlias, client.CreateRoomWithAlias)
	if err != nil {
		return fmt.Errorf("resolving repo channel: %w", err)
	}
	if _, err := client.Send(roomID, message); err != nil {
		return fmt.Errorf("posting to repo channel: %w", err)
	}
	return nil
}
