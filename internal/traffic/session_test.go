package traffic

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	entries := []Request{{
		Time:       time.Unix(100, 0),
		Method:     "GET",
		Path:       "/api/users",
		Status:     200,
		TargetHost: "localhost",
		TargetPort: 3000,
	}}
	if err := SaveSession(path, entries); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != 1 || len(s.Requests) != 1 {
		t.Fatalf("unexpected session: %#v", s)
	}
	if s.Requests[0].Path != "/api/users" {
		t.Fatalf("unexpected path: %q", s.Requests[0].Path)
	}
}

func TestSaveSessionReplacesExistingFileAndCleansTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")
	if err := SaveSession(path, []Request{{Path: "/old"}}); err != nil {
		t.Fatal(err)
	}
	if err := SaveSession(path, []Request{{Path: "/new"}}); err != nil {
		t.Fatalf("replace existing session: %v", err)
	}

	got, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Requests) != 1 || got.Requests[0].Path != "/new" {
		t.Fatalf("got requests %#v, want only /new", got.Requests)
	}
	temps, err := filepath.Glob(filepath.Join(dir, ".session-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temps) != 0 {
		t.Fatalf("temporary session files leaked: %#v", temps)
	}
}
