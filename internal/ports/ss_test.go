package ports

import (
	"reflect"
	"testing"
)

func TestParseSS(t *testing.T) {
	const sample = `State   Recv-Q  Send-Q   Local Address:Port    Peer Address:Port  Process
LISTEN  0       128      127.0.0.1:6379         0.0.0.0:*          users:(("redis-server",pid=1234,fd=6))
LISTEN  0       4096     0.0.0.0:5432           0.0.0.0:*          users:(("postgres",pid=999,fd=7))
LISTEN  0       128      [::]:22                [::]:*             users:(("sshd",pid=1,fd=3))
LISTEN  0       128      127.0.0.1:3000         0.0.0.0:*
ESTAB   0       0        127.0.0.1:6379         127.0.0.1:54321
`

	entries := parseSS(sample)
	if len(entries) != 4 {
		t.Fatalf("got %d entries, want 4: %+v", len(entries), entries)
	}

	byPort := map[int]Entry{}
	for _, e := range entries {
		byPort[e.Port] = e
	}

	if e := byPort[6379]; e.Process != "redis-server" || e.PID != "1234" {
		t.Errorf("port 6379: got process=%q pid=%q, want redis-server/1234", e.Process, e.PID)
	}
	if e := byPort[5432]; e.Process != "postgres" || e.PID != "999" {
		t.Errorf("port 5432: got process=%q pid=%q, want postgres/999", e.Process, e.PID)
	}
	if e := byPort[22]; e.Process != "sshd" || e.PID != "1" {
		t.Errorf("port 22 (IPv6 addr): got process=%q pid=%q, want sshd/1", e.Process, e.PID)
	}
	// No users:(...) segment (e.g. unprivileged caller) should degrade
	// gracefully to an empty Process/PID rather than being dropped or
	// misparsed.
	if e := byPort[3000]; e.Process != "" || e.PID != "" {
		t.Errorf("port 3000: got process=%q pid=%q, want empty", e.Process, e.PID)
	}
	if _, ok := byPort[54321]; ok {
		t.Errorf("ESTAB line should not produce an entry")
	}
}

func TestParseSSCollapsesDualStackListeners(t *testing.T) {
	// One process listening on both 0.0.0.0 and [::] (or 127.0.0.1 and [::1])
	// shows up as two ss rows. Two different processes on one port stay apart.
	const sample = `State  Recv-Q Send-Q Local Address:Port Peer Address:Port Process
LISTEN 0      128          0.0.0.0:22        0.0.0.0:*    users:(("sshd",pid=812,fd=3))
LISTEN 0      128             [::]:22           [::]:*    users:(("sshd",pid=812,fd=4))
LISTEN 0      511        127.0.0.1:5173      0.0.0.0:*    users:(("node",pid=4242,fd=23))
LISTEN 0      511            [::1]:5173         [::]:*    users:(("node",pid=4242,fd=24))
LISTEN 0      4096       127.0.0.1:8080      0.0.0.0:*    users:(("app-a",pid=100,fd=3))
LISTEN 0      4096           [::1]:8080         [::]:*    users:(("app-b",pid=200,fd=3))
LISTEN 0      128          0.0.0.0:631       0.0.0.0:*
LISTEN 0      128             [::]:631          [::]:*
`
	got := parseSS(sample)
	want := []Entry{
		{Port: 22, Process: "sshd", PID: "812", State: "LISTEN"},
		{Port: 5173, Process: "node", PID: "4242", State: "LISTEN"},
		{Port: 8080, Process: "app-a", PID: "100", State: "LISTEN"},
		{Port: 8080, Process: "app-b", PID: "200", State: "LISTEN"},
		{Port: 631, State: "LISTEN"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseSS() =\n%#v\nwant\n%#v", got, want)
	}
}
