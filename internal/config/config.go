package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Listen        string `json:"listen"`
	Target        string `json:"target"`
	WatchInterval string `json:"watch_interval"`
}

func Default() Config {
	return Config{Listen: ":9090", Target: "http://localhost:3000", WatchInterval: "2s"}
}

func Path() string {
	if p := os.Getenv("DEVPULSE_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".devpulse/config.json"
	}
	return filepath.Join(home, ".devpulse", "config.json")
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
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}
