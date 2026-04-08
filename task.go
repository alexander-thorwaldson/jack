package jack

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"jack.dev/jack/msg"
)

var jackTaskCmd = &cobra.Command{
	Use:   "task",
	Short: "Task room messaging",
	Long:  "Post to and read from task-specific rooms as the operator.",
}

var jackTaskPostCmd = &cobra.Command{
	Use:   "post <name> <message...>",
	Short: "Post a message to a task room",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		token := operatorToken(project)
		if token == "" {
			return fmt.Errorf("no token available for project %q", project)
		}
		client := msg.NewClient(msg.Homeserver, token)
		name := args[0]
		message := strings.Join(args[1:], " ")
		taskName := project + "-" + name
		roomName, topic, aliasName := msg.TaskTarget(taskName)
		return msg.RunBoardPost(roomName, topic, aliasName, message, client.ResolveAlias, client.Send, client.CreateRoomWithAlias)
	},
}

var jackTaskReadCmd = &cobra.Command{
	Use:   "read <name>",
	Short: "Read messages from a task room",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		limit, _ := cmd.Flags().GetInt("limit")
		jsonFlag, _ := cmd.Flags().GetBool("json")
		from, _ := cmd.Flags().GetString("from")
		token := operatorToken(project)
		if token == "" {
			return fmt.Errorf("no token available for project %q", project)
		}
		client := msg.NewClient(msg.Homeserver, token)
		name := args[0]
		taskName := project + "-" + name
		roomName, topic, aliasName := msg.TaskTarget(taskName)
		return msg.RunBoardRead(roomName, topic, aliasName, limit, jsonFlag, from, client.ResolveAlias, client.Messages, client.CreateRoomWithAlias)
	},
}

// operatorToken resolves a Matrix token for the operator by reading from the
// first agent's clone for the given project.
func operatorToken(project string) string {
	if t, err := msg.TokenFromEnv(); err == nil && t != "" {
		return t
	}
	reg, err := loadRegistry()
	if err != nil {
		return ""
	}
	for _, agent := range reg.AgentsForRepo(project) {
		dir := fmt.Sprintf("%s/%s/%s", env.dataDir(), agent, project)
		if t, readErr := readToken(dir); readErr == nil && t != "" {
			return t
		}
	}
	return ""
}

func init() {
	jackTaskPostCmd.Flags().StringP("project", "p", "", "project name (required)")
	_ = jackTaskPostCmd.MarkFlagRequired("project")
	jackTaskReadCmd.Flags().StringP("project", "p", "", "project name (required)")
	_ = jackTaskReadCmd.MarkFlagRequired("project")
	jackTaskReadCmd.Flags().IntP("limit", "n", 20, "number of messages to retrieve")
	jackTaskReadCmd.Flags().Bool("json", false, "output messages as JSON")
	jackTaskReadCmd.Flags().String("from", "", "filter messages by sender username")
	jackTaskCmd.AddCommand(jackTaskPostCmd)
	jackTaskCmd.AddCommand(jackTaskReadCmd)
	rootCmd.AddCommand(jackTaskCmd)

	openCmd.Flags().StringSliceP("agent", "a", nil, "agents to invite (defaults to all agents on the repo)")
	closeCmd.Flags().StringSliceP("agent", "a", nil, "agents to remove (defaults to all members)")
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(closeCmd)
}

var openCmd = &cobra.Command{
	Use:   "open <repo> <name>",
	Short: "Open a task room",
	Long:  "Create a task room and invite agents to collaborate on a focused task.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		agents, _ := cmd.Flags().GetStringSlice("agent")
		repo := args[0]
		name := args[1]
		return runOpen(repo, name, agents, loadRegistry, msg.ProvisionTaskRoom)
	},
}

// TaskProvisioner creates a task room and invites agents.
type TaskProvisioner func(repo, name string, inviteUserIDs []string) error

func runOpen(repo, name string, agents []string, loadReg RegistryLoader, provision TaskProvisioner) error {
	reg, err := loadReg()
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	// Resolve which agents to invite.
	if len(agents) == 0 {
		agents = reg.AgentsForRepo(repo)
	}
	if len(agents) == 0 {
		return fmt.Errorf("no agents found for repo %q", repo)
	}

	// Build Matrix user IDs for each agent.
	server := msg.ServerName(msg.Homeserver)
	inviteUserIDs := make([]string, 0, len(agents))
	for _, agent := range agents {
		inviteUserIDs = append(inviteUserIDs, fmt.Sprintf("@%s-%s:%s", agent, repo, server))
	}

	if err := provision(repo, name, inviteUserIDs); err != nil {
		return err
	}

	fmt.Printf("opened task room %s for %s (invited: %s)\n", name, repo, strings.Join(agents, ", "))
	return nil
}

var closeCmd = &cobra.Command{
	Use:   "close <repo> <name>",
	Short: "Close a task room",
	Long:  "Archive a task room by removing all agents and setting it to read-only.\nHistory is preserved on the homeserver.",
	Args:  cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		repo := args[0]
		name := args[1]
		token := operatorToken(repo)
		if token == "" {
			return fmt.Errorf("no token available for repo %q", repo)
		}
		client := msg.NewClient(msg.Homeserver, token)
		return runClose(repo, name, client.ResolveAlias, client.GetMembers, client.WhoAmI, client.Kick, client.SetRoomReadOnly)
	},
}

// WhoAmIFunc returns the current user's identity.
type WhoAmIFunc func() (*msg.WhoAmIResponse, error)

func runClose(repo, name string, resolve msg.AliasResolver, getMembers msg.MemberLister, whoami WhoAmIFunc, kick msg.MemberKicker, setReadOnly msg.RoomReadOnlySetter) error {
	alias := msg.TaskRoomAlias(repo + "-" + name)
	resp, err := resolve(alias)
	if err != nil {
		return fmt.Errorf("task room %q not found: %w", name, err)
	}
	roomID := resp.RoomID

	// Get current user to avoid kicking ourselves.
	me, err := whoami()
	if err != nil {
		return fmt.Errorf("identifying current user: %w", err)
	}

	// Kick all members except the current user.
	members, err := getMembers(roomID)
	if err != nil {
		return fmt.Errorf("listing members: %w", err)
	}
	for _, member := range members {
		if member == me.UserID {
			continue
		}
		if err := kick(roomID, member, "task room closed"); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove %s: %v\n", member, err)
		}
	}

	// Set room to read-only.
	if err := setReadOnly(roomID); err != nil {
		return fmt.Errorf("setting room read-only: %w", err)
	}

	fmt.Printf("closed task room %s for %s\n", name, repo)
	return nil
}
