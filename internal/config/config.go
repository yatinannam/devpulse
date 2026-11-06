package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	data = trimBOM(data)
	if err := json.Unmarshal(data, &c); err != nil {
		return c, nil, fmt.Errorf("decode config: %w", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return c, nil, nil
	}
	var warnings []string
	for key := range raw {
		if !isKnownKey(key) {
			warnings = append(warnings, fmt.Sprintf("unknown key %q ignored", key))
		}
	}
	sort.Strings(warnings)
	return c, warnings, nil
}

// knownKeys are the JSON names of Config's fields, in the order they are
// written.
var knownKeys = []string{"listen", "target", "watch_interval"}

// isKnownKey reports whether encoding/json decodes key into a Config field.
// It matches field names without regard to case, as strings.EqualFold does,
// so "TARGET" sets the target.
func isKnownKey(key string) bool {
	for _, k := range knownKeys {
		if strings.EqualFold(key, k) {
			return true
		}
	}
	return false
}

// trimBOM drops the UTF-8 byte order mark that Windows PowerShell 5.1 puts
// at the start of re-saved files; encoding/json rejects it.
func trimBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}

func Save(c Config) error {
	return SaveTo(c, Path())
}

func SaveProject(c Config, dir string) error {
	return SaveTo(c, ProjectPath(dir))
}

// SaveTo writes c to p. Keys in an existing file at p that Config does not
// know, for example settings written by a newer DevPulse, are kept after
// the known keys, so an older binary does not erase them.
func SaveTo(c Config, p string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if data, err = appendUnknownKeys(data, p); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

// appendUnknownKeys adds the unknown keys of the existing file at p, sorted,
// to the indented JSON object encoded. A missing or unreadable file, or one
// that is not a JSON object, has nothing to keep.
func appendUnknownKeys(encoded []byte, p string) ([]byte, error) {
	existing, err := os.ReadFile(p)
	if err != nil {
		return encoded, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimBOM(existing), &raw); err != nil {
		return encoded, nil
	}
	var unknown []string
	for key := range raw {
		if !isKnownKey(key) {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return encoded, nil
	}
	sort.Strings(unknown)
	out := bytes.TrimSuffix(bytes.TrimRight(encoded, "\n"), []byte("}"))
	out = bytes.TrimRight(out, "\n")
	for _, key := range unknown {
		name, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		var value bytes.Buffer
		if err := json.Indent(&value, raw[key], "  ", "  "); err != nil {
			return nil, err
		}
		out = append(out, ",\n  "...)
		out = append(out, name...)
		out = append(out, ": "...)
		out = append(out, value.Bytes()...)
	}
	return append(out, "\n}"...), nil
}
