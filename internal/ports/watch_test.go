package ports

import "testing"

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
