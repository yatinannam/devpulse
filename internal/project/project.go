package project

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/yatinannam/devpulse/internal/discovery"
)

type Info struct {
	Name string
	Kind string
	// Root is the nearest directory, at or above the one passed to Detect,
	// that holds a project marker. Without a marker it is that directory.
	Root string
}

func Detect(dir string) (Info, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Info{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Info{}, err
	}

	// Run from a subdirectory, the project is the nearest directory above
	// with a marker. The search stops at the repository root (.git is a
	// directory, or a file in worktrees and submodules) or the filesystem root.
	for d := abs; ; {
		if kind, ok := markerKind(d); ok {
			return Info{Name: filepath.Base(d), Kind: kind, Root: d}, nil
		}
		parent := filepath.Dir(d)
		if exists(d, ".git") || parent == d {
			break
		}
		d = parent
	}
	return Info{Name: info.Name(), Kind: "Unknown project", Root: abs}, nil
}

func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

// markerKind reports the project type whose marker file is in dir.
func markerKind(dir string) (string, bool) {
	has := func(name string) bool { return exists(dir, name) }

	switch {
	case has("next.config.js") || has("next.config.mjs") || has("next.config.ts") || has("next.config.cjs"):
		return "Node / Next.js", true
	case has("vite.config.js") || has("vite.config.ts") || has("vite.config.mjs") || has("vite.config.mts") || has("vite.config.cjs") || has("vite.config.cts"):
		return "Node / Vite", true
	case has("go.mod"):
		return "Go", true
	case has("pyproject.toml") || has("requirements.txt") || has("requirements-dev.txt"):
		return "Python", true
	case has("pom.xml") || has("build.gradle") || has("build.gradle.kts"):
		return "Java", true
	case has("package.json"):
		return "Node.js", true
	}
	return "", false
}

func SelectTarget(info Info, services []discovery.Service) (discovery.Service, bool) {
	candidates := make([]discovery.Service, 0, len(services))
	for _, s := range services {
		// Never target DevPulse's own proxy: traffic would loop back into itself.
		if s.HTTP && s.Kind != discovery.KindDevPulseProxy {
			candidates = append(candidates, s)
		}
	}
	if len(candidates) == 0 {
		return discovery.Service{}, false
	}

	expected := map[string]int{
		"Node / Next.js": 3000,
		"Node / Vite":   5173,
		"Python":        8000,
		"Go":            8080,
		"Java":          8080,
	}
	if port, ok := expected[info.Kind]; ok {
		for _, s := range candidates {
			if s.Port == port {
				return s, true
			}
		}
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	for _, port := range []int{3000, 5173, 8000, 8080, 8081, 9000} {
		for _, s := range candidates {
			if s.Port == port {
				return s, true
			}
		}
	}
	// Last resort. Raw OS order often puts an OS service first, such as
	// HTTP.sys on Windows or AirTunes on macOS, which answers HTTP but is
	// never fingerprinted. Prefer an identified framework, then the lowest
	// port, so the choice does not depend on socket order.
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := identified(candidates[i]), identified(candidates[j])
		if a != b {
			return a
		}
		return candidates[i].Port < candidates[j].Port
	})
	return candidates[0], true
}

func identified(s discovery.Service) bool {
	return s.Kind != "" && s.Kind != discovery.KindHTTPService
}
