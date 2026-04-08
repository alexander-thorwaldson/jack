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
	authCmd.Flags().StringP("agent", "a", "", "agent name (required)")
	_ = authCmd.MarkFlagRequired("agent")
	rootCmd.AddCommand(authCmd)
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Store a GitHub token for an agent",
	Long:  "Store a GitHub personal access token for an agent profile.\nThe token is written to the agent's config directory and used to set GH_TOKEN in sessions.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		agent, _ := cmd.Flags().GetString("agent")
		return runAuth(agent)
	},
}

func runAuth(agent string) error {
	profile, ok := cfg.Profiles[agent]
	if !ok {
		return fmt.Errorf("unknown agent %q (no matching profile)", agent)
	}

	label := agent
	if profile.GitHub.User != "" {
		label = fmt.Sprintf("%s (%s)", agent, profile.GitHub.User)
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

	outPath := ghTokenPath(agent)
	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	if err := os.WriteFile(outPath, []byte(token), 0o600); err != nil {
		return fmt.Errorf("writing github token: %w", err)
	}

	fmt.Printf("GitHub token stored for agent %s at %s\n", agent, outPath)
	return nil
}
