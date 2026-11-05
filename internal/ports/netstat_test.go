package ports

import (
	"reflect"
	"testing"
)

func TestParseNetstatFindsIPv4AndIPv6Listeners(t *testing.T) {
	const sample = `
Active Connections

  Proto  Local Address          Foreign Address        State           PID
  TCP    0.0.0.0:135            0.0.0.0:0              LISTENING       1840
  TCP    127.0.0.1:5432         0.0.0.0:0              LISTENING       6472
  TCP    127.0.0.1:5432         127.0.0.1:61000        ESTABLISHED     6472
  TCP    [::]:3000              [::]:0                 LISTENING       4242
  TCP    [::1]:5173             [::]:0                 LISTENING       5151
  TCP    [::1]:5173             [::1]:62000            ESTABLISHED     5151
  UDP    0.0.0.0:5353           *:*                                    2020
`
	got := parseNetstat(sample)
	want := []Entry{
		{Port: 135, PID: "1840", State: "LISTENING"},
		{Port: 5432, PID: "6472", State: "LISTENING"},
		{Port: 3000, PID: "4242", State: "LISTENING"},
		{Port: 5173, PID: "5151", State: "LISTENING"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseNetstat() =\n%#v\nwant\n%#v", got, want)
	}
}

func TestParseNetstatIgnoresLocalizedStateNames(t *testing.T) {
	// German Windows prints ABHÖREN/HERGESTELLT instead of LISTENING/ESTABLISHED.
	const sample = `
Aktive Verbindungen

  Proto  Lokale Adresse         Remoteadresse          Status           PID
  TCP    0.0.0.0:8080           0.0.0.0:0              ABHÖREN          900
  TCP    127.0.0.1:8080         127.0.0.1:50000        HERGESTELLT      900
`
	got := parseNetstat(sample)
	want := []Entry{{Port: 8080, PID: "900", State: "LISTENING"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseNetstat() = %#v, want %#v", got, want)
	}
}

func TestParseNetstatCollapsesDualStackListeners(t *testing.T) {
	const sample = `
  TCP    0.0.0.0:22             0.0.0.0:0              LISTENING       5884
  TCP    [::]:22                [::]:0                 LISTENING       5884
`
	got := parseNetstat(sample)
	want := []Entry{{Port: 22, PID: "5884", State: "LISTENING"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseNetstat() = %#v, want %#v", got, want)
	}
}

func TestParseTasklistCSV(t *testing.T) {
	const sample = `"node.exe","1234","Console","1","50,000 K"
"postgres.exe","5678","Services","0","120,000 K"
"Code.exe","9999","Console","1","200,000 K"
`
	got := parseTasklist(sample)
	want := map[string]string{"1234": "node", "5678": "postgres", "9999": "Code"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseTasklist() = %#v, want %#v", got, want)
	}
}
