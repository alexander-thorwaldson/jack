package jack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"jack.dev/jack/msg"
)

func init() {
	inCmd.Flags().StringP("agent", "a", "", "agent name")
	inCmd.Flags().StringP("project", "p", "", "project name")
	rootCmd.AddCommand(inCmd)
}

// RepoAnnouncer posts a presence message to the repo channel.
type RepoAnnouncer func(token, repo, message string) error

var inCmd = &cobra.Command{
	Use:   "in",
	Short: "Enter a session",
	Long:  "Attach to an existing session or create one.\nWith no arguments, interactively select an agent and project.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		agent, _ := cmd.Flags().GetString("agent")
		project, _ := cmd.Flags().GetString("project")
		return runIn(agent, project,
			loadRegistry,
			selectAgent, selectProject,
			HasSession, CreateSession, AttachSession,
			readGHToken,
			msg.AnnounceOnRepoChannel,
			DockerRun, DockerExec, DockerStop,
		)
	},
}

func runIn(agent, project string, loadReg RegistryLoader, selAgent AgentSelector, selProject ProjectSelector, hasSession SessionChecker, createSession SessionCreator, attach SessionAttacher, readGH GHTokenReader, announce RepoAnnouncer, runContainer ContainerRunner, execContainer ContainerExecer, stopContainer ContainerStopper) error {
	reg, err := loadReg()
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	// Resolve agent.
	if agent == "" {
		agents := reg.Agents()
		switch len(agents) {
		case 0:
			return fmt.Errorf("no projects cloned — run jack clone first")
		case 1:
			agent = agents[0]
		default:
			a, selErr := selAgent(agents)
			if selErr != nil {
				return selErr
			}
			agent = a
		}
	}

	// Resolve project.
	if project == "" {
		repos := reg.ReposForAgent(agent)
		switch len(repos) {
		case 0:
			return fmt.Errorf("no projects cloned for agent %q", agent)
		case 1:
			project = repos[0]
		default:
			p, selErr := selProject(agent, repos)
			if selErr != nil {
				return selErr
			}
			project = p
		}
	}

	name := SessionName(agent, project)
	dir := filepath.Join(env.dataDir(), agent, project)

	// If session exists, attach to it.
	if hasSession(name) {
		return attach(name)
	}

	// Verify agent exists in config.
	if _, ok := cfg.Agents[agent]; !ok {
		return fmt.Errorf("unknown agent %q", agent)
	}

	// Renew agent certificate if expiring soon.
	if cfg.CA.URL != "" && certNeedsRenewal(agent, renewThreshold) {
		if renewErr := renewCert(context.Background(), agent); renewErr != nil {
			fmt.Fprintf(os.Stderr, "warning: cert renewal failed for %s: %v\n", agent, renewErr)
		}
	}

	// Read Matrix token.
	token, _ := readToken(dir)

	// Read GitHub token (per-repo).
	ghToken, err := readGH(project)
	if err != nil {
		return fmt.Errorf("reading github token: %w", err)
	}

	// Announce presence on the repo channel (non-fatal).
	if token != "" && announce != nil {
		if err := announce(token, project, fmt.Sprintf("%s jacked in", name)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: repo channel announcement failed: %v\n", err)
		}
	}

	// Sync Claude OAuth credentials from keychain to disk so the
	// container (which cannot access the macOS keychain) can authenticate.
	if err := syncClaudeCredentials(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not sync claude credentials: %v\n", err)
	}

	// Start the container.
	containerName := ContainerName(agent, project)
	mounts := SessionMounts(cfg, agent, dir)
	envVars := SessionEnv(cfg, agent, token, ghToken)

	if err := runContainer(containerName, mounts, envVars); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	// Run dev.sh inside the container if present.
	if hasDevSh(dir) {
		fmt.Printf("running dev.sh for %s...\n", project)
		if err := execContainer(containerName, []string{"sh", "/workspace/.jack/dev.sh"}); err != nil {
			_ = stopContainer(containerName)
			return fmt.Errorf("running dev.sh: %w", err)
		}
	}

	// Build the tmux command as docker exec into the container.
	shellCmd := buildContainerShellCmd()
	tmuxCmd := DockerExecCmd(containerName, shellCmd)

	if err := createSession(name, dir, tmuxCmd); err != nil {
		_ = stopContainer(containerName)
		return err
	}

	return attach(name)
}
