package jack

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zoobzio/jack/msg"
)

// Cloner clones a git repository into a directory.
type Cloner func(url, dir string) error

func init() {
	cloneCmd.Flags().StringSliceP("team", "t", nil, "teams to clone for (required, repeatable)")
	_ = cloneCmd.MarkFlagRequired("team")
	cloneCmd.Flags().BoolP("force", "f", false, "remove existing repo and session before cloning")
	rootCmd.AddCommand(cloneCmd)
}

var cloneCmd = &cobra.Command{
	Use:   "clone <url>",
	Short: "Clone a repo for a team",
	Long:  "Clone a git repo into each team's isolated workspace and apply team skills.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		teams, _ := cmd.Flags().GetStringSlice("team")
		force, _ := cmd.Flags().GetBool("force")
		client := msg.NewClient(msg.Homeserver, "")
		return runClone(args[0], teams, force, gitClone, HasSession, KillSession, client.Register, client.Login, ageKeygen, ageEncryptToRecipient, loadRegistry, saveRegistry, msg.ProvisionRepoChannel)
	},
}

// RepoProvisioner creates a per-repo Matrix channel and invites other teams.
type RepoProvisioner func(token, repo string, inviteUserIDs []string) error

func runClone(url string, teams []string, force bool, clone Cloner, hasSession SessionChecker, kill SessionKiller, register msg.Registerer, login msg.Authenticator, keygen KeypairGenerator, encrypt TokenEncrypterByRecipient, loadReg RegistryLoader, saveReg RegistrySaver, provisionRepo RepoProvisioner) error {
	repo := repoName(url)
	if repo == "" {
		return fmt.Errorf("cannot extract repo name from %q", url)
	}

	configDir := env.configDir()

	reg, err := loadReg()
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	for _, teamName := range teams {
		// Validate team prerequisites.
		if err := validateTeam(configDir, teamName); err != nil {
			return err
		}

		if _, ok := cfg.Profiles[teamName]; !ok {
			return fmt.Errorf("unknown team %q (no matching profile)", teamName)
		}

		dir := filepath.Join(env.dataDir(), teamName, repo)

		// Check for existing clone.
		if _, err := os.Stat(dir); err == nil {
			if !force {
				fmt.Printf("warning: %s already exists for team %s, skipping (use --force to replace)\n", repo, teamName)
				continue
			}
			// Kill the session if it's running.
			name := SessionName(teamName, repo)
			if hasSession(name) {
				if err := kill(name); err != nil {
					return fmt.Errorf("killing session %s: %w", name, err)
				}
			}
			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("removing %s: %w", dir, err)
			}
		}

		parent := filepath.Dir(dir)
		if err := os.MkdirAll(parent, 0o750); err != nil {
			return fmt.Errorf("creating directory %s: %w", parent, err)
		}

		if err := clone(url, dir); err != nil {
			return fmt.Errorf("cloning %s for team %s: %w", repo, teamName, err)
		}

		if err := applyAgent(teamName, dir); err != nil {
			return fmt.Errorf("applying team %s: %w", teamName, err)
		}

		// Register Matrix user for this session, falling back to login if
		// the user already exists (e.g. re-clone after a failed attempt).
		username := teamName + "-" + repo
		mReg, err := register(username, username, cfg.Matrix.RegistrationToken)
		if err != nil {
			if !strings.Contains(err.Error(), "M_USER_IN_USE") {
				return fmt.Errorf("registering Matrix user %s: %w", username, err)
			}
			mReg, err = login(username, username)
			if err != nil {
				return fmt.Errorf("logging in Matrix user %s: %w", username, err)
			}
		}

		// Generate age keypair for token encryption.
		keypair, err := keygen(ageKeyPath(dir))
		if err != nil {
			return fmt.Errorf("generating age keypair for %s: %w", username, err)
		}

		// Encrypt and store the Matrix token.
		if err := encrypt(mReg.AccessToken, keypair.PublicKey, tokenAgePath(dir)); err != nil {
			return fmt.Errorf("encrypting token for %s: %w", username, err)
		}

		// Record in registry.
		reg.Add(teamName, repo, url)
		if err := saveReg(reg); err != nil {
			return fmt.Errorf("saving registry: %w", err)
		}

		// Provision per-repo channel and invite other teams (non-fatal).
		if provisionRepo != nil {
			server := msg.ServerName(msg.Homeserver)
			var inviteUserIDs []string
			for _, other := range reg.TeamsForRepo(repo) {
				if other != teamName {
					inviteUserIDs = append(inviteUserIDs, fmt.Sprintf("@%s-%s:%s", other, repo, server))
				}
			}
			if err := provisionRepo(mReg.AccessToken, repo, inviteUserIDs); err != nil {
				fmt.Fprintf(os.Stderr, "warning: repo channel provisioning failed: %v\n", err)
			}
		}

		fmt.Printf("cloned %s for team %s\n", repo, teamName)
	}

	return nil
}

func gitClone(url, dir string) error {
	cmd := exec.CommandContext(context.Background(), "git", "clone", url, dir) // #nosec G204 -- args from CLI input
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// repoName extracts the repository name from a git URL.
// Handles both SCP-style (git@host:user/repo.git) and standard URLs.
func repoName(url string) string {
	// Strip trailing .git
	name := strings.TrimSuffix(url, ".git")

	// Handle SCP-style URLs (git@github.com:user/repo)
	if i := strings.LastIndex(name, ":"); i != -1 && !strings.Contains(name, "://") {
		name = name[i+1:]
	}

	// Take last path segment
	if i := strings.LastIndex(name, "/"); i != -1 {
		name = name[i+1:]
	}

	return name
}
