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
	loadCmd.Flags().StringP("project", "p", "", "project name (required)")
	_ = loadCmd.MarkFlagRequired("project")
	rootCmd.AddCommand(loadCmd)
}

var loadCmd = &cobra.Command{
	Use:   "load",
	Short: "Store a GitHub token for a project",
	Long:  "Store a GitHub personal access token scoped to a project.\nThe token is used to set GH_TOKEN in sessions for all agents working on this project.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		project, _ := cmd.Flags().GetString("project")
		return runLoad(project)
	},
}

func runLoad(project string) error {
	fmt.Printf("Enter GitHub personal access token for %s: ", project)
	reader := bufio.NewReader(os.Stdin)
	token, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading token: %w", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token must not be empty")
	}

	outPath := ghTokenPath(project)
	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	if err := os.WriteFile(outPath, []byte(token), 0o600); err != nil {
		return fmt.Errorf("writing github token: %w", err)
	}

	fmt.Printf("GitHub token stored for project %s at %s\n", project, outPath)
	return nil
}
