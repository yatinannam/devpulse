package traffic

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	entries := []Request{{
		Time: time.Unix(100, 0),
		Method: "GET",
		Path: "/api/users",
		Status: 200,
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
