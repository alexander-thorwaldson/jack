package jack

import (
	"fmt"
	"os"
	"path/filepath"
)

// applyAgent provisions CLAUDE.md and skills into the repo's .claude directory.
// Files are symlinked so the jack config directory remains the single source of truth.
func applyAgent(agentName, dir string) error {
	configDir := env.configDir()
	agentDir := filepath.Join(configDir, "agents", agentName)
	claudeDir := filepath.Join(dir, ".claude")

	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		return fmt.Errorf("creating .claude dir: %w", err)
	}

	// Symlink CLAUDE.md into .claude/.
	src := filepath.Join(agentDir, "CLAUDE.md")
	resolved, err := filepath.EvalSymlinks(src)
	if err != nil {
		return fmt.Errorf("resolving CLAUDE.md for agent %q: %w", agentName, err)
	}
	if linkErr := os.Symlink(resolved, filepath.Join(claudeDir, "CLAUDE.md")); linkErr != nil {
		return fmt.Errorf("linking CLAUDE.md for agent %q: %w", agentName, linkErr)
	}

	// Skills — symlink into .claude/commands/.
	skills, err := discoverAgentSkills(agentName)
	if err != nil {
		return err
	}
	if len(skills) > 0 {
		commandsDir := filepath.Join(claudeDir, "commands")
		if err := os.MkdirAll(commandsDir, 0o750); err != nil {
			return fmt.Errorf("creating commands dir: %w", err)
		}
		for _, skill := range skills {
			src := filepath.Join(agentDir, "skills", skill)
			resolved, err := filepath.EvalSymlinks(src)
			if err != nil {
				return fmt.Errorf("resolving skill %q: %w", skill, err)
			}
			dst := filepath.Join(commandsDir, skill)
			if err := os.Symlink(resolved, dst); err != nil {
				return fmt.Errorf("linking skill %q: %w", skill, err)
			}
		}
	}

	return nil
}

// applyRepo provisions repo-level config from ~/.config/jack/repos/<repo>/ into
// the cloned repo. Symlinks dev.sh into .jack/, repo CLAUDE.md into .claude/,
// and repo skills into .claude/commands/. All files are optional.
func applyRepo(repo, dir string) error {
	configDir := env.configDir()
	repoDir := filepath.Join(configDir, "repos", repo)

	// Repo config is optional — skip silently if not present.
	if _, err := os.Stat(repoDir); err != nil {
		return nil
	}

	// Symlink dev.sh into .jack/ if it exists.
	devSh := filepath.Join(repoDir, "dev.sh")
	if _, err := os.Stat(devSh); err == nil {
		jackDir := filepath.Join(dir, ".jack")
		if err := os.MkdirAll(jackDir, 0o750); err != nil {
			return fmt.Errorf("creating .jack dir: %w", err)
		}
		resolved, err := filepath.EvalSymlinks(devSh)
		if err != nil {
			return fmt.Errorf("resolving dev.sh for repo %q: %w", repo, err)
		}
		if err := os.Symlink(resolved, filepath.Join(jackDir, "dev.sh")); err != nil {
			return fmt.Errorf("linking dev.sh for repo %q: %w", repo, err)
		}
	}

	// Symlink repo CLAUDE.md into .claude/ if it exists.
	// This is placed alongside the agent CLAUDE.md — Claude Code merges them.
	repoClaudeMD := filepath.Join(repoDir, "CLAUDE.md")
	if _, err := os.Stat(repoClaudeMD); err == nil {
		claudeDir := filepath.Join(dir, ".claude")
		if err := os.MkdirAll(claudeDir, 0o750); err != nil {
			return fmt.Errorf("creating .claude dir: %w", err)
		}
		resolved, err := filepath.EvalSymlinks(repoClaudeMD)
		if err != nil {
			return fmt.Errorf("resolving CLAUDE.md for repo %q: %w", repo, err)
		}
		dst := filepath.Join(claudeDir, "CLAUDE.local.md")
		if err := os.Symlink(resolved, dst); err != nil {
			return fmt.Errorf("linking CLAUDE.md for repo %q: %w", repo, err)
		}
	}

	// Symlink repo skills into .claude/commands/.
	skills, err := discoverRepoSkills(repo)
	if err != nil {
		return nil // non-fatal if skills dir is missing
	}
	if len(skills) > 0 {
		commandsDir := filepath.Join(dir, ".claude", "commands")
		if err := os.MkdirAll(commandsDir, 0o750); err != nil {
			return fmt.Errorf("creating commands dir: %w", err)
		}
		for _, skill := range skills {
			src := filepath.Join(repoDir, "skills", skill)
			resolved, err := filepath.EvalSymlinks(src)
			if err != nil {
				return fmt.Errorf("resolving repo skill %q: %w", skill, err)
			}
			dst := filepath.Join(commandsDir, skill)
			// Skip if already exists (agent skill takes precedence).
			if _, err := os.Lstat(dst); err == nil {
				continue
			}
			if err := os.Symlink(resolved, dst); err != nil {
				return fmt.Errorf("linking repo skill %q: %w", skill, err)
			}
		}
	}

	return nil
}

// hasDevSh reports whether a repo has a dev.sh script in its .jack directory.
func hasDevSh(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".jack", "dev.sh"))
	return err == nil
}

// validateAgent checks that the agent directory exists with a CLAUDE.md and
// skills directory before cloning begins.
func validateAgent(configDir, agentName string) error {
	agentDir := filepath.Join(configDir, "agents", agentName)
	if _, err := os.Stat(agentDir); err != nil {
		return fmt.Errorf("agent directory not found: agents/%s/", agentName)
	}

	claudeMD := filepath.Join(agentDir, "CLAUDE.md")
	if _, err := os.Stat(claudeMD); err != nil {
		return fmt.Errorf("CLAUDE.md not found in agents/%s/", agentName)
	}

	skillsDir := filepath.Join(agentDir, "skills")
	if info, err := os.Stat(skillsDir); err != nil || !info.IsDir() {
		return fmt.Errorf("skills directory not found: agents/%s/skills/", agentName)
	}

	return nil
}
