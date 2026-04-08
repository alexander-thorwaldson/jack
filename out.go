package jack

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"jack.dev/jack/msg"
)

func init() {
	outCmd.Flags().StringP("agent", "a", "", "agent name")
	outCmd.Flags().StringP("project", "p", "", "project name")
	rootCmd.AddCommand(outCmd)
}

// DepartureAnnouncer posts a departure message to the repo channel.
type DepartureAnnouncer func(token, repo, message string) error

// TokenReader reads the Matrix token for a session from the env file.
type TokenReader func(agent, project string) string

func defaultDepartureAnnouncer(token, repo, message string) error {
	return msg.AnnounceOnRepoChannel(token, repo, message)
}

func defaultTokenReader(agent, project string) string {
	dir := fmt.Sprintf("%s/%s/%s", env.dataDir(), agent, project)
	token, _ := readToken(dir)
	return token
}

// parseSessionName splits a session name into agent and project.
// Agent names cannot contain hyphens, so the first hyphen is the delimiter.
func parseSessionName(name string) (agent, project string) {
	parts := strings.SplitN(name, "-", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return name, ""
}

var outCmd = &cobra.Command{
	Use:   "out [name]",
	Short: "Terminate a session",
	Long:  "Terminate a session by name or by --agent and --project flags.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string
		if len(args) > 0 {
			name = args[0]
		}
		agent, _ := cmd.Flags().GetString("agent")
		project, _ := cmd.Flags().GetString("project")
		return runOut(name, agent, project, HasSession, KillSession, defaultTokenReader, defaultDepartureAnnouncer, DockerStop)
	},
}

func runOut(name, agent, project string, hasSession SessionChecker, kill SessionKiller, readToken TokenReader, announce DepartureAnnouncer, stopContainer ContainerStopper) error {
	if name == "" && agent != "" && project != "" {
		name = SessionName(agent, project)
	}
	if name == "" {
		return fmt.Errorf("specify a session name or both --agent and --project")
	}
	if !hasSession(name) {
		return fmt.Errorf("session %q not found", name)
	}

	// Post departure announcement if token is available (non-fatal).
	if agent == "" || project == "" {
		agent, project = parseSessionName(name)
	}
	if agent != "" && project != "" && readToken != nil && announce != nil {
		if token := readToken(agent, project); token != "" {
			_ = announce(token, project, fmt.Sprintf("%s jacked out", name))
		}
	}

	if err := kill(name); err != nil {
		return err
	}

	// Stop and remove the container.
	if agent != "" && project != "" {
		containerName := ContainerName(agent, project)
		_ = stopContainer(containerName)
	}

	fmt.Printf("killed session %s\n", name)
	return nil
}
