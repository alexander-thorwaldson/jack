package jack

import (
	"fmt"
	"os"
	"path/filepath"
)

// Env holds path overrides loaded from environment variables via fig.
type Env struct {
	ConfigDir string `env:"JACK_CONFIG_DIR" default:"~/.config/jack"`
	DataDir   string `env:"JACK_DATA_DIR"   default:"~/.jack"`
}

// Validate ensures the configured directories are valid paths.
func (e *Env) Validate() error {
	if e.ConfigDir == "" {
		return fmt.Errorf("config dir must not be empty")
	}
	if e.DataDir == "" {
		return fmt.Errorf("data dir must not be empty")
	}
	return nil
}

// configDir returns the expanded config directory path.
func (e *Env) configDir() string {
	return expandHome(e.ConfigDir)
}

// configPath returns the full path to the config file.
func (e *Env) configPath() string {
	return filepath.Join(e.configDir(), "config.yaml")
}

// dataDir returns the expanded data directory path.
func (e *Env) dataDir() string {
	return expandHome(e.DataDir)
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
