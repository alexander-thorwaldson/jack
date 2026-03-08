package jack

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zoobzio/fig"
	"github.com/zoobzio/jack/msg"
)

var env Env

var rootCmd = &cobra.Command{
	Use:   "jack",
	Short: "Operator console for multi-agent development",
	Long:  "Jack manages teams, sandboxes, sessions, and profiles for multi-agent Claude Code development.",
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		if err := fig.Load(&env); err != nil {
			return fmt.Errorf("failed to load environment config: %w", err)
		}
		if err := initConfig(cmd.Context(), env.configPath()); err != nil {
			return err
		}
		msg.Homeserver = cfg.Matrix.Homeserver
		msg.RegistrationToken = cfg.Matrix.RegistrationToken
		msg.DataDir = env.dataDir()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(msg.Cmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
