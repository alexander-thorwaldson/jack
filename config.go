package jack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level YAML configuration.
type Config struct {
	Agents map[string]AgentConfig `yaml:"agents"`
	Matrix MatrixConfig           `yaml:"matrix"`
	Git    GitConfig              `yaml:"git"`
}

// AgentConfig holds per-agent settings.
type AgentConfig struct {
	Repos []string `yaml:"repos"`
}

// MatrixConfig holds Matrix homeserver connection settings.
type MatrixConfig struct {
	Homeserver        string `yaml:"homeserver"`
	RegistrationToken string `yaml:"registration_token"`
}

// GitConfig holds the operator's git identity.
type GitConfig struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

// Validate checks the Config for internal consistency.
func (c Config) Validate() error {
	if len(c.Agents) == 0 {
		return fmt.Errorf("at least one agent must be defined")
	}
	for name := range c.Agents {
		if strings.Contains(name, "-") {
			return fmt.Errorf("agent name %q must not contain hyphens", name)
		}
	}
	return nil
}

// discoverAgentSkills returns skill names for an agent by reading entries from
// the agents/{name}/skills/ directory. Entries may be directories or symlinks.
func discoverAgentSkills(agentName string) ([]string, error) {
	skillsDir := filepath.Join(env.configDir(), "agents", agentName, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("agent skills directory for %q: %w", agentName, err)
	}
	var skills []string
	for _, e := range entries {
		if e.IsDir() || e.Type()&os.ModeSymlink != 0 {
			skills = append(skills, e.Name())
		}
	}
	return skills, nil
}

// discoverRepoSkills returns skill names for a repo by reading entries from
// the repos/{name}/skills/ directory. Returns nil if the directory doesn't exist.
func discoverRepoSkills(repo string) ([]string, error) {
	skillsDir := filepath.Join(env.configDir(), "repos", repo, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, err
	}
	var skills []string
	for _, e := range entries {
		if e.IsDir() || e.Type()&os.ModeSymlink != 0 {
			skills = append(skills, e.Name())
		}
	}
	return skills, nil
}

var cfg Config

// initConfig loads the configuration from the given YAML file.
func initConfig(configPath string) error {
	data, err := os.ReadFile(filepath.Clean(configPath)) // #nosec G304 -- path from internal config
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}
	return nil
}
