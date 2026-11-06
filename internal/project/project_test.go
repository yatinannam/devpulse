package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yatinannam/devpulse/internal/discovery"
)

func TestDetectNextProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "next.config.mjs"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "Node / Next.js" {
		t.Fatalf("kind=%q", got.Kind)
	}
}

// tree creates the given files (and directories, for names ending in "/")
// under a fresh temporary directory and returns it.
func tree(t *testing.T, paths ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, p := range paths {
		full := filepath.Join(root, filepath.FromSlash(p))
		if strings.HasSuffix(p, "/") {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestDetectRecognisesEveryConfigExtension(t *testing.T) {
	cases := map[string]string{
		"next.config.js":  "Node / Next.js",
		"next.config.mjs": "Node / Next.js",
		"next.config.ts":  "Node / Next.js",
		"next.config.cjs": "Node / Next.js",
		"vite.config.js":  "Node / Vite",
		"vite.config.ts":  "Node / Vite",
		"vite.config.mjs": "Node / Vite",
		"vite.config.mts": "Node / Vite",
		"vite.config.cjs": "Node / Vite",
		"vite.config.cts": "Node / Vite",
	}
	for marker, want := range cases {
		t.Run(marker, func(t *testing.T) {
			// package.json is present in real projects and must not win.
			root := tree(t, ".git/", "package.json", marker)
			got, err := Detect(root)
			if err != nil {
				t.Fatal(err)
			}
			if got.Kind != want {
				t.Fatalf("kind=%q, want %q", got.Kind, want)
			}
		})
	}
}

func TestDetectSearchesUpwardForProjectRoot(t *testing.T) {
	root := tree(t, ".git/", "package.json", "vite.config.mts", "src/components/")
	got, err := Detect(filepath.Join(root, "src", "components"))
	if err != nil {
		t.Fatal(err)
	}
	want := Info{Name: filepath.Base(root), Kind: "Node / Vite", Root: root}
	if got != want {
		t.Fatalf("Detect() = %+v, want %+v", got, want)
	}
}

func TestDetectUsesNearestMarker(t *testing.T) {
	// A Go service inside a Node monorepo is a Go project.
	root := tree(t, ".git/", "package.json", "services/api/go.mod", "services/api/cmd/server/")
	got, err := Detect(filepath.Join(root, "services", "api", "cmd", "server"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "services", "api"); got.Kind != "Go" || got.Root != want || got.Name != "api" {
		t.Fatalf("Detect() = %+v, want Go project rooted at %s", got, want)
	}
}

func TestDetectStopsAtGitRoot(t *testing.T) {
	// The package.json above the repository belongs to something else.
	outer := tree(t, "package.json", "repo/.git/", "repo/docs/")
	start := filepath.Join(outer, "repo", "docs")
	got, err := Detect(start)
	if err != nil {
		t.Fatal(err)
	}
	want := Info{Name: "docs", Kind: "Unknown project", Root: start}
	if got != want {
		t.Fatalf("Detect() = %+v, want %+v", got, want)
	}
}

func TestDetectTreatsGitFileAsRepositoryRoot(t *testing.T) {
	// Worktrees and submodules have a .git file instead of a directory.
	outer := tree(t, "go.mod", "repo/.git", "repo/sub/")
	got, err := Detect(filepath.Join(outer, "repo", "sub"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "Unknown project" {
		t.Fatalf("kind=%q, want the search to stop at the .git file", got.Kind)
	}
}

func TestSelectTargetFallbackPrefersIdentifiedFrameworkThenLowestPort(t *testing.T) {
	// Raw OS order, none on a conventional port. 5000 is macOS AirTunes and
	// 2869 is Windows HTTP.sys: both answer HTTP but are not the app.
	services := []discovery.Service{
		{Port: 5000, HTTP: true, Kind: discovery.KindHTTPService, URL: "http://127.0.0.1:5000"},
		{Port: 7001, HTTP: true, Kind: "Node / Express", URL: "http://127.0.0.1:7001"},
		{Port: 2869, HTTP: true, Kind: discovery.KindHTTPService, URL: "http://127.0.0.1:2869"},
		{Port: 4321, HTTP: true, Kind: "Node / Express", URL: "http://127.0.0.1:4321"},
	}
	got, ok := SelectTarget(Info{Kind: "Unknown project"}, services)
	if !ok || got.Port != 4321 {
		t.Fatalf("got %+v, ok=%v; want the identified framework on the lowest port, 4321", got, ok)
	}

	got, ok = SelectTarget(Info{Kind: "Unknown project"}, []discovery.Service{services[0], services[2]})
	if !ok || got.Port != 2869 {
		t.Fatalf("got %+v, ok=%v; want the lowest port, 2869, when nothing is identified", got, ok)
	}
}

func TestSelectTargetPrefersExpectedPort(t *testing.T) {
	services := []discovery.Service{
		{Port: 8080, HTTP: true, URL: "http://127.0.0.1:8080"},
		{Port: 3000, HTTP: true, URL: "http://127.0.0.1:3000"},
	}
	got, ok := SelectTarget(Info{Kind: "Node / Next.js"}, services)
	if !ok || got.Port != 3000 {
		t.Fatalf("got %+v, ok=%v", got, ok)
	}
}

func TestSelectTargetNoHTTPService(t *testing.T) {
	got, ok := SelectTarget(Info{Kind: "Go"}, []discovery.Service{{Port: 8080}})
	if ok || got.Port != 0 {
		t.Fatalf("got %+v, ok=%v", got, ok)
	}
}

func TestSelectTargetNeverPicksDevPulseProxy(t *testing.T) {
	services := []discovery.Service{
		{Port: 9090, HTTP: true, URL: "http://127.0.0.1:9090", Kind: discovery.KindDevPulseProxy},
		{Port: 4321, HTTP: true, URL: "http://127.0.0.1:4321"},
	}
	got, ok := SelectTarget(Info{Kind: "Node.js"}, services)
	if !ok || got.Port != 4321 {
		t.Fatalf("got %+v, ok=%v; want the app on 4321, not the proxy", got, ok)
	}
	if _, ok := SelectTarget(Info{}, services[:1]); ok {
		t.Fatal("the DevPulse proxy alone must not be selected as a target")
	}
}
