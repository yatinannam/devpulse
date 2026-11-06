package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultListenIsLoopbackOnly(t *testing.T) {
	// An all-interfaces default such as ":9090" exposes the proxied dev app
	// and its recorder to the whole local network.
	if got := Default().Listen; got != "127.0.0.1:9090" {
		t.Fatalf("default listen = %q, want 127.0.0.1:9090", got)
	}
}

func TestSaveLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("DEVPULSE_CONFIG", p)
	c := Default()
	c.Target = "http://localhost:8080"
	if err := Save(c); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Target != c.Target {
		t.Fatalf("target=%q", got.Target)
	}
}

func TestPathPrefersProjectConfig(t *testing.T) {
	root := t.TempDir()
	projectConfig := ProjectPath(root)
	if err := SaveProject(Default(), root); err != nil {
		t.Fatal(err)
	}

	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	got := Path()
	gotEval, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatal(err)
	}
	wantEval, err := filepath.EvalSymlinks(projectConfig)
	if err != nil {
		t.Fatal(err)
	}
	if gotEval != wantEval {
		t.Fatalf("path=%q, want %q", gotEval, wantEval)
	}
}

func TestPathFindsProjectConfigFromNestedDirectory(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "src", "api")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := SaveProject(Default(), root); err != nil {
		t.Fatal(err)
	}

	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	got := Path()
	gotEval, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatal(err)
	}
	wantEval, err := filepath.EvalSymlinks(ProjectPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if gotEval != wantEval {
		t.Fatalf("path=%q, want %q", gotEval, wantEval)
	}
}

func TestExplicitConfigOverridesProjectConfig(t *testing.T) {
	root := t.TempDir()
	projectConfig := ProjectPath(root)
	if err := SaveProject(Default(), root); err != nil {
		t.Fatal(err)
	}
	override := filepath.Join(t.TempDir(), "override.json")
	if err := SaveTo(Config{Target: "http://override"}, override); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEVPULSE_CONFIG", override)

	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	if got := Path(); got != override {
		t.Fatalf("path=%q, want %q (project=%q)", got, override, projectConfig)
	}
}

func TestLoadWithWarningsReportsUnknownKeys(t *testing.T) {
	// A typo such as "watchInterval" (instead of "watch_interval") used to be
	// ignored silently, leaving the user wondering why the setting had no effect.
	p := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("DEVPULSE_CONFIG", p)
	if err := os.WriteFile(p, []byte(`{"listen":"127.0.0.1:9191","watchInterval":"5s","targte":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c, warnings, err := LoadWithWarnings()
	if err != nil {
		t.Fatal(err)
	}
	if c.Listen != "127.0.0.1:9191" {
		t.Fatalf("known keys must still load, listen = %q", c.Listen)
	}
	want := []string{`unknown key "targte" ignored`, `unknown key "watchInterval" ignored`}
	if len(warnings) != len(want) {
		t.Fatalf("warnings = %q, want %q", warnings, want)
	}
	for i := range want {
		if warnings[i] != want[i] {
			t.Fatalf("warnings = %q, want %q", warnings, want)
		}
	}
}

func TestLoadWithWarningsIsQuietForAValidFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("DEVPULSE_CONFIG", p)
	if err := SaveTo(Default(), p); err != nil {
		t.Fatal(err)
	}
	if _, warnings, err := LoadWithWarnings(); err != nil || len(warnings) != 0 {
		t.Fatalf("LoadWithWarnings() = %q, %v; want no warnings", warnings, err)
	}
}

func TestLoadWithWarningsAcceptsCaseVariantKeys(t *testing.T) {
	// encoding/json matches keys without regard to case, so "TARGET" sets
	// the target. It used to be applied and also reported as ignored.
	p := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("DEVPULSE_CONFIG", p)
	if err := os.WriteFile(p, []byte(`{"TARGET":"http://localhost:4000","Watch_Interval":"5s","watchInterval":"1s"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c, warnings, err := LoadWithWarnings()
	if err != nil {
		t.Fatal(err)
	}
	if c.Target != "http://localhost:4000" || c.WatchInterval != "5s" {
		t.Fatalf("case-variant keys should load: target %q, watch_interval %q", c.Target, c.WatchInterval)
	}
	want := []string{`unknown key "watchInterval" ignored`}
	if len(warnings) != len(want) || warnings[0] != want[0] {
		t.Fatalf("warnings = %q, want %q", warnings, want)
	}
}

func TestSaveKeepsUnknownKeysAlreadyInTheFile(t *testing.T) {
	// An older binary must not erase settings that a newer one wrote, so
	// keys it does not know survive `devpulse config --listen ...`.
	p := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("DEVPULSE_CONFIG", p)
	original := "\xEF\xBB\xBF" + `{"TARGET":"http://localhost:4000","future_key":{"a":[1,2]},"zeta":true}`
	if err := os.WriteFile(p, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	c.Listen = "127.0.0.1:9191"
	if err := Save(c); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("saved file is not JSON: %v\n%s", err, data)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw["future_key"]); err != nil || compact.String() != `{"a":[1,2]}` {
		t.Fatalf("future_key = %s, want {\"a\":[1,2]}\n%s", raw["future_key"], data)
	}
	if string(raw["zeta"]) != "true" {
		t.Fatalf("zeta = %s, want true\n%s", raw["zeta"], data)
	}
	if _, ok := raw["TARGET"]; ok {
		t.Fatalf("a case variant of a known key should be rewritten as the known key:\n%s", data)
	}
	got, warnings, err := LoadWithWarnings()
	if err != nil {
		t.Fatal(err)
	}
	if got.Listen != "127.0.0.1:9191" || got.Target != "http://localhost:4000" || got.WatchInterval != "2s" {
		t.Fatalf("reloaded config = %+v", got)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %q, want the two unknown keys", warnings)
	}
	if !bytes.HasPrefix(data, []byte("{\n  \"listen\": ")) {
		t.Fatalf("known keys should come first:\n%s", data)
	}
}

func TestLoadIgnoresUTF8ByteOrderMark(t *testing.T) {
	// Windows PowerShell 5.1 (Set-Content -Encoding UTF8) writes a BOM; every
	// command loads config, so rejecting it broke the whole CLI.
	p := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("DEVPULSE_CONFIG", p)
	if err := os.WriteFile(p, []byte("\xEF\xBB\xBF"+`{"target":"http://localhost:5000"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c, warnings, err := LoadWithWarnings()
	if err != nil {
		t.Fatalf("LoadWithWarnings with BOM: %v", err)
	}
	if c.Target != "http://localhost:5000" || len(warnings) != 0 {
		t.Fatalf("got target %q, warnings %q; want http://localhost:5000 and none", c.Target, warnings)
	}
}
