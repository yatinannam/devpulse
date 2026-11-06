package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const projectConfigDir = ".devpulse"
const projectConfigFile = "config.json"

type Config struct {
	Listen        string `json:"listen"`
	Target        string `json:"target"`
	WatchInterval string `json:"watch_interval"`
}

func Default() Config {
	return Config{Listen: "127.0.0.1:9090", Target: "http://localhost:3000", WatchInterval: "2s"}
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
	c, _, err := LoadWithWarnings()
	return c, err
}

// LoadWithWarnings is Load plus a sorted list of non-fatal problems, such as
// unknown keys, that the caller should show the user. Unknown keys are
// ignored rather than rejected so older binaries can read newer files.
func LoadWithWarnings() (Config, []string, error) {
	c := Default()
	data, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		return c, nil, nil
	}
	if err != nil {
		return c, nil, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, nil, fmt.Errorf("decode config: %w", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return c, nil, nil
	}
	known := map[string]bool{"listen": true, "target": true, "watch_interval": true}
	var warnings []string
	for key := range raw {
		if !known[key] {
			warnings = append(warnings, fmt.Sprintf("unknown key %q ignored", key))
		}
	}
	sort.Strings(warnings)
	return c, warnings, nil
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
