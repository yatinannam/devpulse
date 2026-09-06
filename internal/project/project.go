package project

import (
	"os"
	"path/filepath"

	"github.com/yatinannam/devpulse/internal/discovery"
)

type Info struct {
	Name string
	Kind string
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

	result := Info{Name: info.Name(), Kind: "Unknown project"}
	has := func(name string) bool {
		_, err := os.Stat(filepath.Join(abs, name))
		return err == nil
	}

	switch {
	case has("next.config.js") || has("next.config.mjs") || has("next.config.ts"):
		result.Kind = "Node / Next.js"
	case has("vite.config.js") || has("vite.config.ts"):
		result.Kind = "Node / Vite"
	case has("go.mod"):
		result.Kind = "Go"
	case has("pyproject.toml") || has("requirements.txt") || has("requirements-dev.txt"):
		result.Kind = "Python"
	case has("pom.xml") || has("build.gradle") || has("build.gradle.kts"):
		result.Kind = "Java"
	case has("package.json"):
		result.Kind = "Node.js"
	}
	return result, nil
}

func SelectTarget(info Info, services []discovery.Service) (discovery.Service, bool) {
	candidates := make([]discovery.Service, 0, len(services))
	for _, s := range services {
		if s.HTTP {
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
	return candidates[0], true
}
