package project

import (
	"os"
	"path/filepath"
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
