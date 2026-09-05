//go:build darwin

package ports

import "testing"

func TestParseLsof(t *testing.T) {
	const sample = `p1234
cnode
n*:3000
p5678
cpostgres
n127.0.0.1:5432
p9999
cpython
n[::1]:8000
`

	entries := parseLsof(sample)
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(entries), entries)
	}

	byPort := map[int]Entry{}
	for _, e := range entries {
		byPort[e.Port] = e
	}

	if e := byPort[3000]; e.Process != "node" || e.PID != "1234" || e.State != "LISTEN" {
		t.Errorf("port 3000: got process=%q pid=%q state=%q, want node/1234/LISTEN", e.Process, e.PID, e.State)
	}
	if e := byPort[5432]; e.Process != "postgres" || e.PID != "5678" || e.State != "LISTEN" {
		t.Errorf("port 5432: got process=%q pid=%q state=%q, want postgres/5678/LISTEN", e.Process, e.PID, e.State)
	}
	if e := byPort[8000]; e.Process != "python" || e.PID != "9999" || e.State != "LISTEN" {
		t.Errorf("port 8000: got process=%q pid=%q state=%q, want python/9999/LISTEN", e.Process, e.PID, e.State)
	}
}