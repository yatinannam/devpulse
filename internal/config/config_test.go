package config

import (
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
