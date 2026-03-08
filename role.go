package jack

import (
	"fmt"
	"os"
	"path/filepath"
)

// applyAgent provisions CLAUDE.md and skills into the repo's .claude directory.
// Files are symlinked so the jack config directory remains the single source of truth.
func applyAgent(teamName, dir string) error {
	configDir := env.configDir()
	teamDir := filepath.Join(configDir, "teams", teamName)
	claudeDir := filepath.Join(dir, ".claude")

	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		return fmt.Errorf("creating .claude dir: %w", err)
	}

	// Symlink CLAUDE.md into .claude/.
	src := filepath.Join(teamDir, "CLAUDE.md")
	resolved, err := filepath.EvalSymlinks(src)
	if err != nil {
		return fmt.Errorf("resolving CLAUDE.md for team %q: %w", teamName, err)
	}
	if linkErr := os.Symlink(resolved, filepath.Join(claudeDir, "CLAUDE.md")); linkErr != nil {
		return fmt.Errorf("linking CLAUDE.md for team %q: %w", teamName, linkErr)
	}

	// Skills — symlink into .claude/commands/.
	skills, err := discoverTeamSkills(teamName)
	if err != nil {
		return err
	}
	if len(skills) > 0 {
		commandsDir := filepath.Join(claudeDir, "commands")
		if err := os.MkdirAll(commandsDir, 0o750); err != nil {
			return fmt.Errorf("creating commands dir: %w", err)
		}
		for _, skill := range skills {
			src := filepath.Join(teamDir, "skills", skill)
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

// validateTeam checks that the team directory exists with a CLAUDE.md and
// skills directory before cloning begins.
func validateTeam(configDir, teamName string) error {
	teamDir := filepath.Join(configDir, "teams", teamName)
	if _, err := os.Stat(teamDir); err != nil {
		return fmt.Errorf("team directory not found: teams/%s/", teamName)
	}

	claudeMD := filepath.Join(teamDir, "CLAUDE.md")
	if _, err := os.Stat(claudeMD); err != nil {
		return fmt.Errorf("CLAUDE.md not found in teams/%s/", teamName)
	}

	skillsDir := filepath.Join(teamDir, "skills")
	if info, err := os.Stat(skillsDir); err != nil || !info.IsDir() {
		return fmt.Errorf("skills directory not found: teams/%s/skills/", teamName)
	}

	return nil
}
