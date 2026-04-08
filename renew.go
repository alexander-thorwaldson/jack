package jack

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

const renewThreshold = 1 * time.Hour

func init() {
	renewCmd.Flags().StringSliceP("agent", "a", nil, "agents to renew (defaults to all)")
	rootCmd.AddCommand(renewCmd)
}

var renewCmd = &cobra.Command{
	Use:   "renew",
	Short: "Renew agent certificates",
	Long:  "Renew certificates for agents that are expiring soon.\nWith no flags, renews all agents whose certs expire within 1 hour.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		agents, _ := cmd.Flags().GetStringSlice("agent")
		return runRenew(cmd.Context(), agents, renewCert)
	},
}

func runRenew(ctx context.Context, agents []string, renew CertRenewer) error {
	if cfg.CA.URL == "" {
		return fmt.Errorf("ca.url not configured — certificate management requires a CA")
	}

	// Default to all agents.
	if len(agents) == 0 {
		for name := range cfg.Agents {
			agents = append(agents, name)
		}
		sort.Strings(agents)
	}

	var renewed, skipped int
	for _, agent := range agents {
		if _, ok := cfg.Agents[agent]; !ok {
			fmt.Printf("warning: unknown agent %q, skipping\n", agent)
			continue
		}

		if !certNeedsRenewal(agent, renewThreshold) {
			expiry, _ := certExpiry(agent)
			fmt.Printf("%s: valid until %s, skipping\n", agent, expiry.Format(time.RFC3339))
			skipped++
			continue
		}

		if err := renew(ctx, agent); err != nil {
			return fmt.Errorf("renewing cert for %s: %w", agent, err)
		}
		fmt.Printf("%s: renewed\n", agent)
		renewed++
	}

	fmt.Printf("renewed %d, skipped %d\n", renewed, skipped)
	return nil
}
