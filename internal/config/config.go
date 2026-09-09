package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const projectConfigDir = ".devpulse"
const projectConfigFile = "config.json"

type Config struct {
	Listen        string `json:"listen"`
	Target        string `json:"target"`
	WatchInterval string `json:"watch_interval"`
}

func Default() Config {
	return Config{Listen: ":9090", Target: "http://localhost:3000", WatchInterval: "2s"}
}

// Path returns the highest-priority configuration path: explicit override,
// project-local .devpulse/config.json, then the user-global configuration.
func Path() string {
	if p := os.Getenv("DEVPULSE_CONFIG"); p != "" {
		return p
	}
	if p, ok := findProjectConfig("."); ok {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(projectConfigDir, projectConfigFile)
	}
	return filepath.Join(home, projectConfigDir, projectConfigFile)
}

func ProjectPath(dir string) string {
	return filepath.Join(dir, projectConfigDir, projectConfigFile)
}

func findProjectConfig(dir string) (string, bool) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		p := filepath.Join(abs, projectConfigDir, projectConfigFile)
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", false
		}
		abs = parent
	}
}

func Load() (Config, error) {
	c := Default()
	data, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("decode config: %w", err)
	}
	return c, nil
}

func Save(c Config) error {
	return SaveTo(c, Path())
}

func SaveProject(c Config, dir string) error {
	return SaveTo(c, ProjectPath(dir))
}

func SaveTo(c Config, p string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}
