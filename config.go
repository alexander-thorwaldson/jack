package jack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zoobzio/flux"
	"github.com/zoobzio/flux/file"
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

// discoverTeamSkills returns skill names for a team by reading entries from
// the teams/{name}/skills/ directory. Entries may be directories or symlinks.
func discoverTeamSkills(teamName string) ([]string, error) {
	skillsDir := filepath.Join(env.configDir(), "teams", teamName, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("team skills directory for %q: %w", teamName, err)
	}
	var skills []string
	for _, e := range entries {
		if e.IsDir() || e.Type()&os.ModeSymlink != 0 {
			skills = append(skills, e.Name())
		}
	}
	return skills, nil
}

var capacitor *flux.Capacitor[Config]

var cfg Config

// initConfig creates and starts the flux capacitor for the config file.
func initConfig(ctx context.Context, configPath string) error {
	capacitor = flux.New[Config](
		file.New(configPath),
		func(_ context.Context, _, curr Config) error {
			cfg = curr
			return nil
		},
	).Codec(flux.YAMLCodec{})

	if err := capacitor.Start(ctx); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	current, ok := capacitor.Current()
	if !ok {
		return fmt.Errorf("config loaded but no current value available")
	}
	cfg = current

	return nil
}
