package jack

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	authCmd.Flags().StringP("team", "t", "", "team name (required)")
	_ = authCmd.MarkFlagRequired("team")
	rootCmd.AddCommand(authCmd)
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Store a GitHub token for a team",
	Long:  "Store a GitHub personal access token for a team profile.\nThe token is written to the team's config directory and used to set GH_TOKEN in sessions.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		team, _ := cmd.Flags().GetString("team")
		return runAuth(team)
	},
}

func runAuth(team string) error {
	profile, ok := cfg.Profiles[team]
	if !ok {
		return fmt.Errorf("unknown team %q (no matching profile)", team)
	}

	label := team
	if profile.GitHub.User != "" {
		label = fmt.Sprintf("%s (%s)", team, profile.GitHub.User)
	}

	fmt.Printf("Enter GitHub personal access token for %s: ", label)
	reader := bufio.NewReader(os.Stdin)
	token, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading token: %w", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token must not be empty")
	}

	outPath := ghTokenPath(team)
	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	if err := os.WriteFile(outPath, []byte(token), 0o600); err != nil {
		return fmt.Errorf("writing github token: %w", err)
	}

	fmt.Printf("GitHub token stored for team %s at %s\n", team, outPath)
	return nil
}
