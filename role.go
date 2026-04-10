package jack

import (
	"fmt"
	"os"
	"path/filepath"
)

// applyAgent provisions CLAUDE.md and skills into the repo's .claude directory.
// Files are copied so they are available inside Docker containers where host
// symlink targets are not reachable.
func applyAgent(agentName, dir string) error {
	configDir := env.configDir()
	agentDir := filepath.Join(configDir, "agents", agentName)
	claudeDir := filepath.Join(dir, ".claude")

	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		return fmt.Errorf("creating .claude dir: %w", err)
	}

	// Copy CLAUDE.md into .claude/.
	src := filepath.Join(agentDir, "CLAUDE.md")
	if err := copyFile(src, filepath.Join(claudeDir, "CLAUDE.md")); err != nil {
		return fmt.Errorf("copying CLAUDE.md for agent %q: %w", agentName, err)
	}

	// Skills — copy into .claude/commands/.
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
			dst := filepath.Join(commandsDir, skill)
			if err := copyPath(src, dst); err != nil {
				return fmt.Errorf("copying skill %q: %w", skill, err)
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

	// Copy dev.sh into .jack/ if it exists.
	devSh := filepath.Join(repoDir, "dev.sh")
	if _, err := os.Stat(devSh); err == nil {
		jackDir := filepath.Join(dir, ".jack")
		if err := os.MkdirAll(jackDir, 0o750); err != nil {
			return fmt.Errorf("creating .jack dir: %w", err)
		}
		if err := copyFile(devSh, filepath.Join(jackDir, "dev.sh")); err != nil {
			return fmt.Errorf("copying dev.sh for repo %q: %w", repo, err)
		}
	}

	// Copy repo CLAUDE.md into .claude/ if it exists.
	// This is placed alongside the agent CLAUDE.md — Claude Code merges them.
	repoClaudeMD := filepath.Join(repoDir, "CLAUDE.md")
	if _, err := os.Stat(repoClaudeMD); err == nil {
		claudeDir := filepath.Join(dir, ".claude")
		if err := os.MkdirAll(claudeDir, 0o750); err != nil {
			return fmt.Errorf("creating .claude dir: %w", err)
		}
		dst := filepath.Join(claudeDir, "CLAUDE.local.md")
		if err := copyFile(repoClaudeMD, dst); err != nil {
			return fmt.Errorf("copying CLAUDE.md for repo %q: %w", repo, err)
		}
	}

	// Copy repo skills into .claude/commands/.
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
			dst := filepath.Join(commandsDir, skill)
			// Skip if already exists (agent skill takes precedence).
			if _, err := os.Lstat(dst); err == nil {
				continue
			}
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("copying repo skill %q: %w", skill, err)
			}
		}
	}

	return nil
}

// copyPath copies src to dst. If src is a file, it is copied directly.
// If src is a directory, it is copied recursively. Symlinks are followed.
func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

// copyFile reads src (following symlinks) and writes a regular file at dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src) // #nosec G304 -- paths from trusted agent config
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600) // #nosec G703 -- paths from trusted agent/repo config
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o750); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if err := copyPath(s, d); err != nil {
			return err
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
