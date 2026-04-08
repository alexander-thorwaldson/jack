package jack

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// Lister retrieves tmux sessions.
type Lister func() ([]TmuxSession, error)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show agent and session status",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return runStatus(os.Stdout, loadRegistry, ListSessions)
	},
}

func runStatus(w io.Writer, loadReg RegistryLoader, list Lister) error {
	reg, err := loadReg()
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	sessions, err := list()
	if err != nil {
		return fmt.Errorf("listing tmux sessions: %w", err)
	}

	// Build session lookup by name.
	sessionMap := make(map[string]TmuxSession)
	for _, s := range sessions {
		sessionMap[s.Name] = s
	}

	agents := reg.Agents()
	if len(agents) == 0 {
		_, _ = fmt.Fprintln(w, "no projects cloned")
		return nil
	}

	for i, agent := range agents {
		if i > 0 {
			_, _ = fmt.Fprintln(w)
		}
		_, _ = fmt.Fprintln(w, agent)

		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "PROJECT\tSESSION\tSTATUS")

		for _, entry := range reg.ForAgent(agent) {
			name := SessionName(agent, entry.Repo)
			if s, ok := sessionMap[name]; ok {
				info := SessionInfo{TmuxSession: s, Agent: agent, Repo: entry.Repo}
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", entry.Repo, name, sessionStatus(info))
			} else {
				_, _ = fmt.Fprintf(tw, "%s\t-\tnot running\n", entry.Repo)
			}
		}
		_ = tw.Flush()
	}

	return nil
}

func sessionStatus(info SessionInfo) string {
	if info.Attached {
		return "attached"
	}
	idle := time.Since(info.Activity)
	if idle < time.Minute {
		return "active"
	}
	return fmt.Sprintf("idle %s", formatDuration(idle))
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
