package jack

import (
	"fmt"
	"os"
	"path/filepath"
)

// Env holds path overrides loaded from environment variables.
type Env struct {
	ConfigDir string
	DataDir   string
}

// loadEnv reads environment variables with defaults.
func loadEnv() Env {
	return Env{
		ConfigDir: envOrDefault("JACK_CONFIG_DIR", "~/.config/jack"),
		DataDir:   envOrDefault("JACK_DATA_DIR", "~/.jack"),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
