package ports

import (
	"errors"
	"testing"
	"time"
)

func TestSnapshotOf(t *testing.T) {
	entries := []Entry{{Port: 3000, Process: "node", PID: "1"}, {Port: 5432, Process: "postgres", PID: "2"}}
	s := SnapshotOf(entries)
	if len(s) != 2 {
		t.Fatalf("len=%d want 2", len(s))
	}
	if s[3000].Process != "node" || s[5432].PID != "2" {
		t.Fatalf("unexpected snapshot: %+v", s)
	}
}

type fakeExitError struct{ code int }

func (e fakeExitError) Error() string { return "exit status" }
func (e fakeExitError) ExitCode() int  { return e.code }

func TestTolerateNoMatchesTreatsExitOneAsEmptyResult(t *testing.T) {
	// lsof exits 1 when no socket matches the filter.
	out, err := tolerateNoMatches([]byte(""), fakeExitError{1})
	if err != nil || len(out) != 0 {
		t.Fatalf("got (%q, %v), want empty output and no error", out, err)
	}
	if _, err := tolerateNoMatches(nil, fakeExitError{2}); err == nil {
		t.Fatal("exit status 2 must still be an error")
	}
	if _, err := tolerateNoMatches(nil, errors.New("exec: not found")); err == nil {
		t.Fatal("a missing binary must still be an error")
	}
}

func TestWatchSurvivesTransientListFailures(t *testing.T) {
	results := []struct {
		entries []Entry
		err     error
	}{
		{[]Entry{{Port: 3000, PID: "1"}}, nil},
		{nil, errors.New("transient")},
		{nil, nil},
	}
	call := 0
	list := func() ([]Entry, error) {
		r := results[call]
		call++
		return r.entries, r.err
	}
	ticks := make(chan time.Time, 2)
	ticks <- time.Time{}
	ticks <- time.Time{}
	close(ticks)

	var removed []Entry
	var errs []error
	err := watchWith(ticks, list, func(_, r []Entry) { removed = append(removed, r...) }, func(e error) { errs = append(errs, e) })
	if err != nil {
		t.Fatalf("watchWith returned %v", err)
	}
	if len(errs) != 1 {
		t.Fatalf("expected the transient error to be reported once, got %v", errs)
	}
	if len(removed) != 1 || removed[0].Port != 3000 {
		t.Fatalf("expected port 3000 to be reported closed after the failure, got %#v", removed)
	}
}

func TestWatchReportsRestartWithNewPIDOnSamePort(t *testing.T) {
	// nodemon or air can restart a server between two polls: the port stays
	// open but the process behind it is new.
	polls := [][]Entry{
		{{Port: 3000, Process: "node", PID: "100"}, {Port: 5432, Process: "postgres", PID: "7"}},
		{{Port: 3000, Process: "node", PID: "200"}, {Port: 5432, Process: "postgres", PID: "7"}},
		{{Port: 3000, Process: "node", PID: "200"}, {Port: 5432, Process: "postgres", PID: "7"}},
	}
	call := 0
	list := func() ([]Entry, error) {
		p := polls[call]
		call++
		return p, nil
	}
	ticks := make(chan time.Time, 2)
	ticks <- time.Time{}
	ticks <- time.Time{}
	close(ticks)

	type change struct{ added, removed []Entry }
	var changes []change
	err := watchWith(ticks, list, func(a, r []Entry) { changes = append(changes, change{a, r}) }, nil)
	if err != nil {
		t.Fatalf("watchWith returned %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("got %d change events, want 1 for the restart and none for the unchanged poll: %#v", len(changes), changes)
	}
	got := changes[0]
	if len(got.removed) != 1 || got.removed[0] != (Entry{Port: 3000, Process: "node", PID: "100"}) {
		t.Fatalf("removed = %#v, want the old node process on 3000", got.removed)
	}
	if len(got.added) != 1 || got.added[0] != (Entry{Port: 3000, Process: "node", PID: "200"}) {
		t.Fatalf("added = %#v, want the new node process on 3000", got.added)
	}
}
