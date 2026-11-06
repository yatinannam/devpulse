package diff

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/baseline"
)

func TestCompareFindsServiceAndTrafficChanges(t *testing.T) {
	base := baseline.Snapshot{
		Services: []baseline.Service{{Port: 3000, Kind: "node"}},
		Requests: baseline.Traffic{Total: 10, Errors: 1, Average: int64(20 * time.Millisecond)},
	}
	current := baseline.Snapshot{
		Services: []baseline.Service{{Port: 4000, Kind: "go"}},
		Requests: baseline.Traffic{Total: 14, Errors: 3, Average: int64(50 * time.Millisecond)},
	}
	changes := Compare(base, current)
	if len(changes) != 3 {
		t.Fatalf("got %d changes, want 3: %#v", len(changes), changes)
	}
	kinds := map[Kind]bool{}
	for _, c := range changes {
		kinds[c.Kind] = true
	}
	if !kinds[ServiceAdded] || !kinds[ServiceRemoved] || !kinds[TrafficChanged] {
		t.Fatalf("missing expected change kinds: %#v", changes)
	}
}

func TestCompareReportsRemovedEndpoints(t *testing.T) {
	base := baseline.Snapshot{
		Requests: baseline.Traffic{
			Endpoints: []baseline.Endpoint{{Method: "GET", Path: "/old", Count: 4, Errors: 1, Average: int64(40 * time.Millisecond)}},
		},
	}
	current := baseline.Snapshot{}
	changes := Compare(base, current)
	if len(changes) != 1 {
		t.Fatalf("got %d changes, want 1: %#v", len(changes), changes)
	}
	want := Change{EndpointRemoved, "GET /old", "endpoint removed; last seen count 4, errors 1, average 40ms"}
	if changes[0] != want {
		t.Fatalf("got %#v, want %#v", changes[0], want)
	}
}

func TestCompareReportsAddedEndpoints(t *testing.T) {
	current := baseline.Snapshot{
		Requests: baseline.Traffic{
			Endpoints: []baseline.Endpoint{{Method: "POST", Path: "/new", Count: 3, Errors: 0, Average: int64(1500 * time.Millisecond)}},
		},
	}
	changes := Compare(baseline.Snapshot{}, current)
	want := []Change{{EndpointAdded, "POST /new", "new endpoint; count 3, errors 0, average 1.5s"}}
	if !reflect.DeepEqual(changes, want) {
		t.Fatalf("got %#v, want %#v", changes, want)
	}
}

func TestCompareReportsChangedEndpoints(t *testing.T) {
	endpoint := baseline.Endpoint{Method: "GET", Path: "/items/:id", Count: 4, Errors: 0, Average: int64(20 * time.Millisecond)}
	changed := endpoint
	changed.Count, changed.Errors = 6, 2
	base := baseline.Snapshot{Requests: baseline.Traffic{Endpoints: []baseline.Endpoint{endpoint}}}
	current := baseline.Snapshot{Requests: baseline.Traffic{Endpoints: []baseline.Endpoint{changed}}}
	want := []Change{{EndpointChanged, "GET /items/:id", "count 4→6, errors 0→2, average 20ms→20ms"}}
	if got := Compare(base, current); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestCompareAveragesAtMillisecondResolution(t *testing.T) {
	snapshot := func(average time.Duration) baseline.Snapshot {
		return baseline.Snapshot{Requests: baseline.Traffic{
			Total: 5, Average: int64(average),
			Endpoints: []baseline.Endpoint{{Method: "GET", Path: "/", Count: 5, Average: int64(average)}},
		}}
	}
	ms := time.Millisecond
	t.Run("sub-millisecond difference is not a change", func(t *testing.T) {
		got := Compare(snapshot(20*ms+100*time.Nanosecond), snapshot(20*ms+200*time.Nanosecond))
		if len(got) != 0 {
			t.Fatalf("got %#v, want no changes", got)
		}
	})
	t.Run("values rounding to the same millisecond are not a change", func(t *testing.T) {
		got := Compare(snapshot(19*ms+600*time.Microsecond), snapshot(20*ms+400*time.Microsecond))
		if len(got) != 0 {
			t.Fatalf("got %#v, want no changes", got)
		}
	})
	t.Run("values rounding to different milliseconds are a change", func(t *testing.T) {
		got := Compare(snapshot(20*ms+400*time.Microsecond), snapshot(20*ms+600*time.Microsecond))
		want := []Change{
			{EndpointChanged, "GET /", "count 5→5, errors 0→0, average 20ms→21ms"},
			{TrafficChanged, "traffic", "requests 5→5, errors 0→0, average 20ms→21ms"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})
	t.Run("sub-millisecond averages print as 0ms", func(t *testing.T) {
		got := Compare(snapshot(300*time.Microsecond), snapshot(2*ms))
		if len(got) != 2 || got[1].Message != "requests 5→5, errors 0→0, average 0ms→2ms" {
			t.Fatalf("got %#v", got)
		}
	})
}

func TestCompareReportsServiceChangesOnTheSamePort(t *testing.T) {
	cases := []struct {
		name          string
		base, current baseline.Service
		want          string
	}{
		{"kind and process", baseline.Service{Port: 3000, Kind: "Node / Next.js", Process: "node"}, baseline.Service{Port: 3000, Kind: "Go", Process: "api"}, "service changed from Node / Next.js (node) to Go (api)"},
		{"process only", baseline.Service{Port: 3000, Kind: "Go", Process: "api"}, baseline.Service{Port: 3000, Kind: "Go", Process: "worker"}, "service changed from Go (api) to Go (worker)"},
		{"kind only", baseline.Service{Port: 3000, Kind: "Go"}, baseline.Service{Port: 3000, Kind: "Python"}, "service changed from Go to Python"},
		{"unknown kind", baseline.Service{Port: 3000}, baseline.Service{Port: 3000, Kind: "Go"}, "service changed from TCP service to Go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Compare(baseline.Snapshot{Services: []baseline.Service{tc.base}}, baseline.Snapshot{Services: []baseline.Service{tc.current}})
			want := []Change{{ServiceChanged, ":3000", tc.want}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v, want %#v", got, want)
			}
		})
	}
}

func TestCompareIgnoresUnknownProcessNames(t *testing.T) {
	// Process lookup can fail without privileges, so a missing name on one
	// side is not evidence that a different program is listening.
	base := baseline.Snapshot{Services: []baseline.Service{{Port: 3000, Kind: "Go", Process: "api"}}}
	current := baseline.Snapshot{Services: []baseline.Service{{Port: 3000, Kind: "Go"}}}
	if got := Compare(base, current); len(got) != 0 {
		t.Fatalf("got %#v, want no changes", got)
	}
	if got := Compare(current, base); len(got) != 0 {
		t.Fatalf("got %#v, want no changes", got)
	}
}

func TestCompareSortsByKindThenNumericPortThenSubject(t *testing.T) {
	base := baseline.Snapshot{
		Services: []baseline.Service{{Port: 10000, Kind: "Go"}, {Port: 3000, Kind: "Go"}, {Port: 8080, Kind: "Go"}},
		Requests: baseline.Traffic{Endpoints: []baseline.Endpoint{{Method: "GET", Path: "/b", Count: 1}, {Method: "GET", Path: "/a", Count: 1}}},
	}
	current := baseline.Snapshot{
		Services: []baseline.Service{{Port: 9000, Kind: "Go"}, {Port: 443, Kind: "Go"}, {Port: 8080, Kind: "Node"}},
	}
	want := []string{
		"endpoint-removed GET /a", "endpoint-removed GET /b",
		"service-added :443", "service-added :9000",
		"service-changed :8080",
		"service-removed :3000", "service-removed :10000",
	}
	for i := 0; i < 20; i++ {
		var got []string
		for _, c := range Compare(base, current) {
			got = append(got, string(c.Kind)+" "+c.Subject)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: got %q, want %q", i, got, want)
		}
	}
}

func TestCompareIgnoresIdenticalSnapshots(t *testing.T) {
	snapshot := baseline.Snapshot{Requests: baseline.Traffic{Total: 2}}
	if got := Compare(snapshot, snapshot); len(got) != 0 {
		t.Fatalf("got changes for identical snapshots: %#v", got)
	}
}

func TestCompareWithoutChangesMarshalsAsEmptyArray(t *testing.T) {
	snapshot := baseline.Snapshot{Requests: baseline.Traffic{Total: 2}}
	data, err := json.Marshal(Compare(snapshot, snapshot))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "[]" {
		t.Fatalf("got %s, want []", data)
	}
}
