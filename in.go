package jack

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zoobzio/jack/msg"
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
			sshAdd, ageDecrypt, readGHToken,
			msg.AnnounceOnRepoChannel,
		)
	},
}

func runIn(agent, project string, loadReg RegistryLoader, selAgent AgentSelector, selProject ProjectSelector, hasSession SessionChecker, createSession SessionCreator, attach SessionAttacher, addKey KeyAdder, decrypt TokenDecrypter, readGH GHTokenReader, announce RepoAnnouncer) error {
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

	// Create a new session.
	profile, ok := cfg.Profiles[agent]
	if !ok {
		return fmt.Errorf("unknown agent %q (no matching profile)", agent)
	}

	if profile.SSH.Key != "" {
		key := expandHome(profile.SSH.Key)
		if addErr := addKey(key); addErr != nil {
			return fmt.Errorf("ssh-add %s: %w", key, addErr)
		}
	}

	// Decrypt Matrix token using the session's age keypair.
	var token string
	agePath := tokenAgePath(dir)
	if _, statErr := os.Stat(agePath); statErr == nil {
		privKeyPath := ageKeyPath(dir)
		t, decErr := decrypt(privKeyPath, agePath)
		if decErr != nil {
			return fmt.Errorf("decrypting token: %w", decErr)
		}
		token = t
	}

	// Read plaintext GitHub token.
	ghToken, err := readGH(agent)
	if err != nil {
		return fmt.Errorf("reading github token: %w", err)
	}

	// Announce presence on the repo channel (non-fatal).
	if token != "" && announce != nil {
		if err := announce(token, project, fmt.Sprintf("%s jacked in", name)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: repo channel announcement failed: %v\n", err)
		}
	}

	shellCmd := buildShellCmd(profile, dir)

	// Ensure project and .jack directories exist.
	jackDir := filepath.Join(dir, ".jack")
	if err := os.MkdirAll(jackDir, 0o750); err != nil {
		return fmt.Errorf("creating .jack dir: %w", err)
	}

	// Write .env file at the project root with session variables.
	dotEnvContent := buildDotEnv(agent, token, ghToken)
	dotEnvPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(dotEnvPath, []byte(dotEnvContent), 0o600); err != nil {
		return fmt.Errorf("writing .env file: %w", err)
	}

	// Write to a script file so tmux doesn't have to handle long inline
	// commands. Capture stderr to a log file for diagnostics.
	scriptPath := filepath.Join(jackDir, "session.sh")
	logPath := filepath.Join(jackDir, "session.log")
	content := fmt.Sprintf("#!/bin/sh\n%s 2>%s\n", shellCmd, logPath)
	if err := os.WriteFile(scriptPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing session script: %w", err)
	}

	if err := createSession(name, dir, "sh "+scriptPath); err != nil {
		return err
	}

	return attach(name)
}
