package jack

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zoobzio/jack/msg"
)

func init() {
	inCmd.Flags().StringP("team", "t", "", "team name")
	inCmd.Flags().StringP("project", "p", "", "project name")
	rootCmd.AddCommand(inCmd)
}

// RepoAnnouncer posts a presence message to the repo channel.
type RepoAnnouncer func(token, repo, message string) error

var inCmd = &cobra.Command{
	Use:   "in",
	Short: "Enter a session",
	Long:  "Attach to an existing session or create one.\nWith no arguments, interactively select a team and project.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		team, _ := cmd.Flags().GetString("team")
		project, _ := cmd.Flags().GetString("project")
		return runIn(team, project,
			loadRegistry,
			selectTeam, selectProject,
			HasSession, CreateSession, AttachSession,
			sshAdd, ageDecrypt, readGHToken,
			msg.AnnounceOnRepoChannel,
		)
	},
}

func runIn(team, project string, loadReg RegistryLoader, selTeam TeamSelector, selProject ProjectSelector, hasSession SessionChecker, createSession SessionCreator, attach SessionAttacher, addKey KeyAdder, decrypt TokenDecrypter, readGH GHTokenReader, announce RepoAnnouncer) error {
	reg, err := loadReg()
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	// Resolve team.
	if team == "" {
		teams := reg.Teams()
		switch len(teams) {
		case 0:
			return fmt.Errorf("no projects cloned — run jack clone first")
		case 1:
			team = teams[0]
		default:
			t, selErr := selTeam(teams)
			if selErr != nil {
				return selErr
			}
			team = t
		}
	}

	// Resolve project.
	if project == "" {
		repos := reg.ReposForTeam(team)
		switch len(repos) {
		case 0:
			return fmt.Errorf("no projects cloned for team %q", team)
		case 1:
			project = repos[0]
		default:
			p, selErr := selProject(team, repos)
			if selErr != nil {
				return selErr
			}
			project = p
		}
	}

	name := SessionName(team, project)
	dir := filepath.Join(env.dataDir(), team, project)

	// If session exists, attach to it.
	if hasSession(name) {
		return attach(name)
	}

	// Create a new session.
	profile, ok := cfg.Profiles[team]
	if !ok {
		return fmt.Errorf("unknown team %q (no matching profile)", team)
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
	ghToken, err := readGH(team)
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
	dotEnvContent := buildDotEnv(team, token, ghToken)
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
