package baseline

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/discovery"
	"github.com/yatinannam/devpulse/internal/traffic"
)

func TestBuildNormalizesTrafficIntoBaseline(t *testing.T) {
	services := []discovery.Service{{Port: 3000, Kind: "Node / Next.js", HTTP: true, Process: "node"}}
	requests := []traffic.Request{
		{Method: "GET", Path: "/api/users?id=1", Status: 200, Latency: 100 * time.Millisecond, TargetPort: 3000},
		{Method: "GET", Path: "/api/users?id=2", Status: 500, Latency: 300 * time.Millisecond, TargetPort: 3000},
	}
	s := Build(services, requests)
	if s.Version != Version || len(s.Services) != 1 {
		t.Fatalf("unexpected snapshot metadata: %+v", s)
	}
	if s.Requests.Total != 2 || s.Requests.Errors != 1 || len(s.Requests.Endpoints) != 1 {
		t.Fatalf("unexpected traffic baseline: %+v", s.Requests)
	}
	if s.Requests.Endpoints[0].Path != "/api/users" {
		t.Fatalf("path=%q", s.Requests.Endpoints[0].Path)
	}
}

func TestSaveLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".devpulse", FileName)
	s := Build(nil, []traffic.Request{{Method: "GET", Path: "/", Status: 200}})
	if err := Save(p, s); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != Version || got.Requests.Total != 1 {
		t.Fatalf("unexpected snapshot: %+v", got)
	}
}

// Windows PowerShell 5.1 writes a UTF-8 byte order mark when it re-saves a
// file (Set-Content -Encoding UTF8); diff and anomalies must still load it.
func TestLoadIgnoresUTF8ByteOrderMark(t *testing.T) {
	p := filepath.Join(t.TempDir(), FileName)
	if err := Save(p, Build(nil, []traffic.Request{{Method: "GET", Path: "/", Status: 200}})); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append([]byte{0xEF, 0xBB, 0xBF}, data...), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatalf("Load with BOM: %v", err)
	}
	if got.Requests.Total != 1 {
		t.Fatalf("Requests.Total = %d, want 1", got.Requests.Total)
	}
}

// A failed save must not leave a temporary file next to the target. Save
// used a fixed "<path>.tmp" name and never removed it when the rename
// failed, for example when --to named a directory.
func TestSaveLeavesNoTemporaryFileWhenTheRenameFails(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "baseline.json")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := Save(target, Build(nil, nil)); err == nil {
		t.Fatal("expected saving over a directory to fail")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "baseline.json" {
			t.Errorf("leftover file after a failed save: %s", e.Name())
		}
	}
}

func TestSaveReplacesAnExistingBaseline(t *testing.T) {
	p := filepath.Join(t.TempDir(), FileName)
	ok := traffic.Request{Method: "GET", Path: "/", Status: 200}
	if err := Save(p, Build(nil, []traffic.Request{ok})); err != nil {
		t.Fatal(err)
	}
	if err := Save(p, Build(nil, []traffic.Request{ok, ok})); err != nil {
		t.Fatalf("second save: %v", err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Requests.Total != 2 {
		t.Fatalf("Requests.Total = %d, want 2", got.Requests.Total)
	}
	entries, err := os.ReadDir(filepath.Dir(p))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want only %s in the directory, got %d entries", FileName, len(entries))
	}
}
