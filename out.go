package jack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zoobzio/jack/msg"
)

func init() {
	outCmd.Flags().StringP("team", "t", "", "team name")
	outCmd.Flags().StringP("project", "p", "", "project name")
	rootCmd.AddCommand(outCmd)
}

// DepartureAnnouncer posts a departure message to the repo channel.
type DepartureAnnouncer func(token, repo, message string) error

// TokenReader reads the Matrix token for a session from the env file.
type TokenReader func(team, project string) string

func defaultDepartureAnnouncer(token, repo, message string) error {
	return msg.AnnounceOnRepoChannel(token, repo, message)
}

func defaultTokenReader(team, project string) string {
	envPath := filepath.Clean(filepath.Join(env.dataDir(), team, project, ".env"))
	data, err := os.ReadFile(envPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimPrefix(line, "export ")
		if k, v, ok := strings.Cut(line, "="); ok && k == "JACK_MSG_TOKEN" {
			return v
		}
	}
	return ""
}

// parseSessionName splits a session name into team and project.
// Team names cannot contain hyphens, so the first hyphen is the delimiter.
func parseSessionName(name string) (team, project string) {
	parts := strings.SplitN(name, "-", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return name, ""
}

var outCmd = &cobra.Command{
	Use:   "out [name]",
	Short: "Terminate a session",
	Long:  "Terminate a session by name or by --team and --project flags.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var name string
		if len(args) > 0 {
			name = args[0]
		}
		team, _ := cmd.Flags().GetString("team")
		project, _ := cmd.Flags().GetString("project")
		return runOut(name, team, project, HasSession, KillSession, defaultTokenReader, defaultDepartureAnnouncer)
	},
}

func runOut(name, team, project string, hasSession SessionChecker, kill SessionKiller, readToken TokenReader, announce DepartureAnnouncer) error {
	if name == "" && team != "" && project != "" {
		name = SessionName(team, project)
	}
	if name == "" {
		return fmt.Errorf("specify a session name or both --team and --project")
	}
	if !hasSession(name) {
		return fmt.Errorf("session %q not found", name)
	}

	// Post departure announcement if token is available (non-fatal).
	if team == "" || project == "" {
		team, project = parseSessionName(name)
	}
	if team != "" && project != "" && readToken != nil && announce != nil {
		if token := readToken(team, project); token != "" {
			_ = announce(token, project, fmt.Sprintf("%s jacked out", name))
		}
	}

	if err := kill(name); err != nil {
		return err
	}
	fmt.Printf("killed session %s\n", name)
	return nil
}
