package traffic

import (
	"os"
	"path/filepath"
	"strings"
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

func TestSaveSessionWaitsForAReaderToReleaseTheFile(t *testing.T) {
	// On Windows, a reader such as `devpulse watch` holds the session open
	// without FILE_SHARE_DELETE, so the rename failed with "Access is
	// denied." and the whole capture was lost. Elsewhere the rename
	// succeeds at once.
	path := filepath.Join(t.TempDir(), "session.json")
	if err := SaveSession(path, []Request{{Path: "/old"}}); err != nil {
		t.Fatal(err)
	}
	reader, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	released := make(chan struct{})
	go func() {
		time.Sleep(300 * time.Millisecond)
		reader.Close()
		close(released)
	}()
	defer func() { <-released }()

	if err := SaveSession(path, []Request{{Path: "/new"}}); err != nil {
		t.Fatalf("SaveSession while a reader briefly held the file: %v", err)
	}
	<-released
	got, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Requests) != 1 || got.Requests[0].Path != "/new" {
		t.Fatalf("got requests %#v, want only /new", got.Requests)
	}
}

func TestSaveSessionKeepsTheCaptureWhenTheRenameFails(t *testing.T) {
	// A failed rename used to delete the temporary file, the only copy of
	// the captured requests. A directory in the session's place makes the
	// rename fail on every platform.
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")
	if err := os.MkdirAll(filepath.Join(path, "occupied"), 0755); err != nil {
		t.Fatal(err)
	}
	err := SaveSession(path, []Request{{Path: "/important"}})
	if err == nil {
		t.Fatal("SaveSession over a directory succeeded")
	}
	temps, _ := filepath.Glob(filepath.Join(dir, ".session-*.tmp"))
	if len(temps) != 1 {
		t.Fatalf("temporary files after a failed rename: %v, want the one holding the capture", temps)
	}
	if !strings.Contains(err.Error(), temps[0]) {
		t.Fatalf("error %q does not say where the capture was kept (%s)", err, temps[0])
	}
	kept, err := LoadSession(temps[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(kept.Requests) != 1 || kept.Requests[0].Path != "/important" {
		t.Fatalf("kept file holds %#v, want the captured request", kept.Requests)
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
