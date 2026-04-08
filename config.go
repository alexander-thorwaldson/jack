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
	Profiles map[string]Profile `yaml:"profiles"`
	Matrix   MatrixConfig       `yaml:"matrix"`
}

// MatrixConfig holds Matrix homeserver connection settings.
type MatrixConfig struct {
	Homeserver        string `yaml:"homeserver"`
	RegistrationToken string `yaml:"registration_token"`
}

// Profile represents a git/GitHub/SSH identity.
type Profile struct {
	Git    GitConfig    `yaml:"git"`
	GitHub GitHubConfig `yaml:"github"`
	SSH    SSHConfig    `yaml:"ssh"`
	Repos  []string     `yaml:"repos"`
}

// GitConfig holds git identity settings.
type GitConfig struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

// GitHubConfig holds GitHub account settings.
type GitHubConfig struct {
	User string `yaml:"user"`
}

// SSHConfig holds SSH key settings.
type SSHConfig struct {
	Key string `yaml:"key"`
}

// Validate checks the Config for internal consistency.
func (c Config) Validate() error {
	if len(c.Profiles) == 0 {
		return fmt.Errorf("at least one profile must be defined")
	}
	for name := range c.Profiles {
		if strings.Contains(name, "-") {
			return fmt.Errorf("profile name %q must not contain hyphens", name)
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
