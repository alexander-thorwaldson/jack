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
