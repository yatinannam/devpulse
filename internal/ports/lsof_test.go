package ports

import (
	"reflect"
	"testing"
)

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

func TestParseLsofCollapsesDualStackListeners(t *testing.T) {
	// lsof lists the TCP4 and TCP6 descriptors of one process separately.
	// lsof always adds the f (file descriptor) field, which is ignored.
	const sample = `p812
csshd
f3
n*:22
f4
n*:22
p4242
cnode
f23
n127.0.0.1:5173
f24
n[::1]:5173
p100
capp-a
f3
n127.0.0.1:8080
p200
capp-b
f3
n[::1]:8080
`
	got := parseLsof(sample)
	want := []Entry{
		{Port: 22, Process: "sshd", PID: "812", State: "LISTEN"},
		{Port: 5173, Process: "node", PID: "4242", State: "LISTEN"},
		{Port: 8080, Process: "app-a", PID: "100", State: "LISTEN"},
		{Port: 8080, Process: "app-b", PID: "200", State: "LISTEN"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLsof() =\n%#v\nwant\n%#v", got, want)
	}
}
