package config

import (
	"os"
	"path/filepath"
	"testing"
)

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
	if err != nil { t.Fatal(err) }
	wantEval, err := filepath.EvalSymlinks(projectConfig)
	if err != nil { t.Fatal(err) }
	if gotEval != wantEval { t.Fatalf("path=%q, want %q", gotEval, wantEval) }
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
	if err != nil { t.Fatal(err) }
	wantEval, err := filepath.EvalSymlinks(ProjectPath(root))
	if err != nil { t.Fatal(err) }
	if gotEval != wantEval { t.Fatalf("path=%q, want %q", gotEval, wantEval) }
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
