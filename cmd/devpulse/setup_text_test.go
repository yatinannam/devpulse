package main

import (
	"strings"
	"testing"

	"github.com/yatinannam/devpulse/internal/config"
	"github.com/yatinannam/devpulse/internal/project"
)

func TestRenderHelpGroupsEveryCommand(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	renderHelp(&b, 100)
	out := b.String()

	groups := []string{"Discovery and capture", "Analysis", "Baselines and change detection", "Reporting and export", "Setup"}
	last := -1
	for _, g := range groups {
		i := strings.Index(out, "\n"+g+"\n")
		if i < 0 || i < last {
			t.Fatalf("group %q missing or out of order:\n%s", g, out)
		}
		last = i
	}
	commands := []string{"init", "baseline", "diff", "summary", "timeline", "anomalies", "validate", "report", "export", "ports", "traffic", "status", "doctor", "recent", "watch", "config", "version"}
	for _, c := range commands {
		if n := strings.Count(out, "\n  "+c+" "); n != 1 {
			t.Errorf("command %q listed %d times:\n%s", c, n, out)
		}
	}
	if !strings.HasPrefix(out, "devpulse") || !strings.Contains(out, "Usage  devpulse <command> [flags]\n") {
		t.Fatalf("unexpected header:\n%s", out)
	}
	if !strings.HasSuffix(out, "\nRun devpulse <command> --help for command-specific flags.\n") {
		t.Fatalf("unexpected footer:\n%s", out)
	}
	if strings.Contains(out, "Commands:") || strings.ToUpper(out) == out {
		t.Fatalf("help should use sentence-case groups:\n%s", out)
	}
	assertWithin(t, out, 100)
}

func TestRenderHelpAlignsDescriptions(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	renderHelp(&b, 100)
	for _, line := range strings.Split(b.String(), "\n") {
		if strings.HasPrefix(line, "  ") && len(line) > 13 && line[13] == ' ' {
			t.Errorf("description should start at column 13: %q", line)
		}
	}
	if !strings.Contains(b.String(), "\n  anomalies  Detect regressions against a saved baseline\n") {
		t.Fatalf("missing aligned anomalies row:\n%s", b.String())
	}
}

func TestRenderInitConfigured(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	info := project.Info{Name: "my-app", Kind: "Node / Next.js", Root: "/src/my-app"}
	renderInit(&b, 100, info, "http://127.0.0.1:3000", "/src/my-app/.devpulse/config.json")
	want := "devpulse init\n\n" +
		"● Configured my-app (Node / Next.js) with target http://127.0.0.1:3000\n" +
		"  Saved to /src/my-app/.devpulse/config.json\n" +
		"\n" +
		"Next: devpulse traffic\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRenderInitShortensALongSavedPath(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	saved := `C:\Users\dev\AppData\Local\Temp\a-very-long-directory-name\another-one\my-app\.devpulse\config.json`
	renderInit(&b, 60, project.Info{Name: "my-app", Kind: "Go"}, "http://127.0.0.1:8080", saved)
	assertWithin(t, b.String(), 60)
	if !strings.Contains(b.String(), "\n  Saved to …") || !strings.Contains(b.String(), `my-app\.devpulse\config.json`+"\n") {
		t.Fatalf("the path should keep its end:\n%s", b.String())
	}
}

func TestRenderInitWithoutAMatchGivesDirection(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	renderInit(&b, 100, project.Info{Name: "my-app", Kind: "Go"}, "", "")
	want := "devpulse init\n\n" +
		"▲ No local HTTP service matched my-app (Go).\n" +
		"  Start the application, then run devpulse init again.\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRenderConfig(t *testing.T) {
	plainOutput(t)
	c := config.Config{Listen: "127.0.0.1:9090", Target: "http://localhost:3000", WatchInterval: "2s"}
	var b strings.Builder
	renderConfig(&b, 80, c, "/home/dev/.devpulse/config.json", true, false)
	want := "devpulse config\n\n" +
		"Listen          127.0.0.1:9090\n" +
		"Target          http://localhost:3000\n" +
		"Watch interval  2s\n" +
		"Source          /home/dev/.devpulse/config.json\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRenderConfigNotesSavesAndMissingFiles(t *testing.T) {
	plainOutput(t)
	c := config.Default()
	var b strings.Builder
	renderConfig(&b, 80, c, "/home/dev/.devpulse/config.json", false, false)
	if !strings.Contains(b.String(), "Source          /home/dev/.devpulse/config.json (not created yet)\n") {
		t.Fatalf("missing-file note expected:\n%s", b.String())
	}
	b.Reset()
	long := "/home/dev/" + strings.Repeat("nested/", 12) + ".devpulse/config.json"
	renderConfig(&b, 60, c, long, false, false)
	assertWithin(t, b.String(), 60)
	if !strings.Contains(b.String(), "…") || !strings.Contains(b.String(), "/.devpulse/config.json (not created yet)\n") {
		t.Fatalf("a long source path should keep its end and the note:\n%s", b.String())
	}
	b.Reset()
	renderConfig(&b, 80, c, "/home/dev/.devpulse/config.json", true, true)
	if !strings.HasPrefix(b.String(), titleLine(80, "config", "saved")) {
		t.Fatalf("a saved change should say so in the title:\n%s", b.String())
	}
}
