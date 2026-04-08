package jack

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"jack.dev/jack/msg"
)

// Cloner clones a git repository into a directory.
type Cloner func(url, dir string) error

// TokenPrompter prompts for a GitHub PAT and returns it.
type TokenPrompter func(repo string) (string, error)

func init() {
	cloneCmd.Flags().StringSliceP("agent", "a", nil, "agents to clone for (required, repeatable)")
	_ = cloneCmd.MarkFlagRequired("agent")
	cloneCmd.Flags().BoolP("force", "f", false, "remove existing repo and session before cloning")
	rootCmd.AddCommand(cloneCmd)
}

var cloneCmd = &cobra.Command{
	Use:   "clone <url>",
	Short: "Clone a repo for an agent",
	Long:  "Clone a git repo into each agent's isolated workspace and apply agent skills.\nPrompts for a GitHub PAT if one is not already stored for this repo.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agents, _ := cmd.Flags().GetStringSlice("agent")
		force, _ := cmd.Flags().GetBool("force")
		client := msg.NewClient(msg.Homeserver, "")
		return runClone(cmd.Context(), args[0], agents, force, gitCloneWithPAT, promptForPAT, readGHToken, HasSession, KillSession, client.Register, client.Login, writeToken, loadRegistry, saveRegistry, msg.ProvisionRepoChannel, DockerBuild)
	},
}

// RepoProvisioner creates a per-repo Matrix channel and invites other agents.
type RepoProvisioner func(token, repo string, inviteUserIDs []string) error

func runClone(ctx context.Context, url string, agents []string, force bool, clone Cloner, promptPAT TokenPrompter, readGH GHTokenReader, hasSession SessionChecker, kill SessionKiller, register msg.Registerer, login msg.Authenticator, storeToken TokenWriter, loadReg RegistryLoader, saveReg RegistrySaver, provisionRepo RepoProvisioner, buildImage ImageBuilder) error {
	repo := repoName(url)
	if repo == "" {
		return fmt.Errorf("cannot extract repo name from %q", url)
	}

	// Build the jack base image.
	if err := buildImage(ctx); err != nil {
		return fmt.Errorf("building jack image: %w", err)
	}

	// Resolve GitHub PAT: read stored token or prompt for one.
	ghToken, _ := readGH(repo)
	if ghToken == "" {
		t, err := promptPAT(repo)
		if err != nil {
			return err
		}
		ghToken = t

		// Store the PAT for future use.
		outPath := ghTokenPath(repo)
		if err := storeToken(ghToken, outPath); err != nil {
			return fmt.Errorf("storing github token: %w", err)
		}
	}

	configDir := env.configDir()

	reg, err := loadReg()
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	for _, agentName := range agents {
		// Validate agent prerequisites.
		if err := validateAgent(configDir, agentName); err != nil {
			return err
		}

		if _, ok := cfg.Agents[agentName]; !ok {
			return fmt.Errorf("unknown agent %q", agentName)
		}

		// Issue a certificate for this agent if CA is configured and no cert exists.
		if cfg.CA.URL != "" && !hasCert(agentName) {
			if err := issueCert(ctx, agentName); err != nil {
				return fmt.Errorf("issuing cert for agent %s: %w", agentName, err)
			}
			fmt.Printf("issued certificate for agent %s\n", agentName)
		}

		dir := filepath.Join(env.dataDir(), agentName, repo)

		// Check for existing clone.
		if _, err := os.Stat(dir); err == nil {
			if !force {
				fmt.Printf("warning: %s already exists for agent %s, skipping (use --force to replace)\n", repo, agentName)
				continue
			}
			// Kill the session if it's running.
			name := SessionName(agentName, repo)
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

		// Clone using HTTPS with PAT authentication.
		cloneURL := httpsCloneURL(url, ghToken)
		if err := clone(cloneURL, dir); err != nil {
			return fmt.Errorf("cloning %s for agent %s: %w", repo, agentName, err)
		}

		if err := applyAgent(agentName, dir); err != nil {
			return fmt.Errorf("applying agent %s: %w", agentName, err)
		}

		if err := applyRepo(repo, dir); err != nil {
			return fmt.Errorf("applying repo config for %s: %w", repo, err)
		}

		// Clone supporting repos specified in the agent config.
		agentCfg := cfg.Agents[agentName]
		for _, supportURL := range agentCfg.Repos {
			supportRepo := repoName(supportURL)
			if supportRepo == "" {
				fmt.Fprintf(os.Stderr, "warning: cannot extract repo name from %q, skipping\n", supportURL)
				continue
			}
			supportDir := filepath.Join(env.dataDir(), agentName, supportRepo)
			if _, err := os.Stat(supportDir); err == nil {
				if !force {
					continue
				}
				if err := os.RemoveAll(supportDir); err != nil {
					return fmt.Errorf("removing %s: %w", supportDir, err)
				}
			}
			supportParent := filepath.Dir(supportDir)
			if err := os.MkdirAll(supportParent, 0o750); err != nil {
				return fmt.Errorf("creating directory %s: %w", supportParent, err)
			}
			supportCloneURL := httpsCloneURL(supportURL, ghToken)
			if err := clone(supportCloneURL, supportDir); err != nil {
				return fmt.Errorf("cloning supporting repo %s for agent %s: %w", supportRepo, agentName, err)
			}
			fmt.Printf("cloned supporting repo %s for agent %s\n", supportRepo, agentName)
		}

		// Register Matrix user for this session, falling back to login if
		// the user already exists (e.g. re-clone after a failed attempt).
		username := agentName + "-" + repo
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

		// Store the Matrix token.
		if err := storeToken(mReg.AccessToken, tokenPath(dir)); err != nil {
			return fmt.Errorf("storing token for %s: %w", username, err)
		}

		// Record in registry.
		reg.Add(agentName, repo, url)
		if err := saveReg(reg); err != nil {
			return fmt.Errorf("saving registry: %w", err)
		}

		// Provision per-repo channel and invite other agents (non-fatal).
		if provisionRepo != nil {
			server := msg.ServerName(msg.Homeserver)
			var inviteUserIDs []string
			for _, other := range reg.AgentsForRepo(repo) {
				if other != agentName {
					inviteUserIDs = append(inviteUserIDs, fmt.Sprintf("@%s-%s:%s", other, repo, server))
				}
			}
			if err := provisionRepo(mReg.AccessToken, repo, inviteUserIDs); err != nil {
				fmt.Fprintf(os.Stderr, "warning: repo channel provisioning failed: %v\n", err)
			}
		}

		fmt.Printf("cloned %s for agent %s\n", repo, agentName)
	}

	return nil
}

// promptForPAT interactively prompts the user for a GitHub PAT.
func promptForPAT(repo string) (string, error) {
	fmt.Printf("Enter GitHub personal access token for %s: ", repo)
	reader := bufio.NewReader(os.Stdin)
	token, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("reading token: %w", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("token must not be empty")
	}
	return token, nil
}

// httpsCloneURL converts any git URL to an HTTPS URL with PAT authentication.
func httpsCloneURL(url, pat string) string {
	// Already HTTPS — inject PAT.
	if strings.HasPrefix(url, "https://") {
		// https://github.com/org/repo.git → https://x-access-token:PAT@github.com/org/repo.git
		return strings.Replace(url, "https://", "https://x-access-token:"+pat+"@", 1)
	}

	// SCP-style: git@github.com:org/repo.git → https://x-access-token:PAT@github.com/org/repo.git
	if strings.HasPrefix(url, "git@") {
		trimmed := strings.TrimPrefix(url, "git@")
		trimmed = strings.Replace(trimmed, ":", "/", 1)
		return "https://x-access-token:" + pat + "@" + trimmed
	}

	// ssh://git@github.com/org/repo.git → https://x-access-token:PAT@github.com/org/repo.git
	if strings.HasPrefix(url, "ssh://") {
		trimmed := strings.TrimPrefix(url, "ssh://git@")
		return "https://x-access-token:" + pat + "@" + trimmed
	}

	return url
}

func gitCloneWithPAT(url, dir string) error {
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
