package main

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the entire user configuration: where the hub lives. Everything
// else is hardcoded by design (ADR-0001) so the tool stays zero-maintenance.
type Config struct {
	Hub string `toml:"hub"`
}

// DefaultTargets are the two hardcoded link destinations (ADR-0001).
// ZCode reads ~/.agents/skills, Claude Code reads ~/.claude/skills.
func DefaultTargets() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".agents", "skills"),
		filepath.Join(home, ".claude", "skills"),
	}
}

// configPath is ~/.config/skillsend/config.toml — the Unix convention the
// spec pins, regardless of the platform's UserConfigDir.
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "skillsend", "config.toml"), nil
}

// LoadConfig reads the config; found=false means first run.
func LoadConfig() (Config, bool, error) {
	var c Config
	path, err := configPath()
	if err != nil {
		return c, false, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, false, nil
	}
	if err != nil {
		return c, false, err
	}
	if err := toml.Unmarshal(data, &c); err != nil {
		return c, true, err
	}
	return c, true, nil
}

// SaveConfig persists the config.
func SaveConfig(c Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
