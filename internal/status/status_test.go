package status

import (
	"github.com/yatinannam/devpulse/internal/discovery"
	"github.com/yatinannam/devpulse/internal/traffic"
	"testing"
	"time"
)

func TestBuild(t *testing.T) {
	e := []traffic.Request{{Status: 200, Latency: 100 * time.Millisecond}, {Status: 500, Latency: 600 * time.Millisecond}}
	s := Build(e)
	if s.Total != 2 || s.Errors != 1 || s.Slow != 1 || s.Average != 350*time.Millisecond {
		t.Fatalf("unexpected %+v", s)
	}
}
func TestGroupByService(t *testing.T) {
	services := []discovery.Service{{Port: 3000, HTTP: true, Kind: "Node / Vite"}, {Port: 5432, HTTP: false}}
	e := []traffic.Request{{TargetPort: 3000, Status: 200, Latency: 100 * time.Millisecond, Method: "GET", Path: "/x"}, {TargetPort: 3000, Status: 500, Latency: 600 * time.Millisecond, Method: "GET", Path: "/x"}}
	g := GroupByService(services, e)
	if len(g) != 2 || g[0].Total != 2 || g[0].Errors != 1 || Health(g[0]) != "error" || g[1].Total != 0 {
		t.Fatalf("unexpected %+v", g)
	}
}

func TestNormalizeEndpointPath(t *testing.T) {
	cases := map[string]string{
		"/api/users?id=1": "/api/users",
		"/api/users?id=2": "/api/users",
		"/api/users/123": "/api/users/:id",
		"/api/users/456": "/api/users/:id",
		"/api/items/550e8400-e29b-41d4-a716-446655440000": "/api/items/:id",
		"/health": "/health",
	}
	for input, want := range cases {
		if got := normalizeEndpointPath(input); got != want {
			t.Errorf("normalizeEndpointPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestEndpointsAggregatesNormalizedPaths(t *testing.T) {
	entries := []traffic.Request{
		{Method: "GET", Path: "/api/users?id=1", Status: 200, Latency: 100 * time.Millisecond},
		{Method: "GET", Path: "/api/users?id=2", Status: 500, Latency: 300 * time.Millisecond},
		{Method: "GET", Path: "/api/users/123", Status: 200, Latency: 200 * time.Millisecond},
	}
	got := Endpoints(entries)
	if len(got) != 2 {
		t.Fatalf("got %d endpoints, want 2: %+v", len(got), got)
	}
	var users Endpoint
	for _, e := range got {
		if e.Path == "/api/users" {
			users = e
		}
	}
	if users.Count != 2 || users.Errors != 1 || users.Average != 200*time.Millisecond {
		t.Fatalf("normalized query endpoint = %+v", users)
	}
}

func TestGroupByServiceDedupesServicesByPort(t *testing.T) {
	// The same port can be listed more than once, for example by two PIDs
	// sharing a socket. Each request must be counted for one service only.
	services := []discovery.Service{
		{Port: 3000, PID: "1", HTTP: true, Kind: "Node / Express"},
		{Port: 3000, PID: "2", HTTP: true, Kind: "Node / Express"},
		{Port: 5432, PID: "3"},
	}
	e := []traffic.Request{
		{TargetPort: 3000, Status: 200, Latency: 10 * time.Millisecond, Method: "GET", Path: "/a"},
		{TargetPort: 3000, Status: 200, Latency: 10 * time.Millisecond, Method: "GET", Path: "/b"},
	}
	g := GroupByService(services, e)
	if len(g) != 2 {
		t.Fatalf("got %d groups, want one per port: %+v", len(g), g)
	}
	if g[0].Service.Port != 3000 || g[0].Service.PID != "1" || g[0].Total != 2 {
		t.Fatalf("first group = %+v, want port 3000 (PID 1) with 2 requests", g[0])
	}
	if g[1].Service.Port != 5432 || g[1].Total != 0 {
		t.Fatalf("second group = %+v, want port 5432 with no requests", g[1])
	}
}

func TestEndpointsBreaksTiesByPathThenMethod(t *testing.T) {
	var entries []traffic.Request
	for _, r := range []struct{ method, path string }{
		{"POST", "/a"}, {"GET", "/c"}, {"PUT", "/b"}, {"GET", "/a"}, {"DELETE", "/a"}, {"GET", "/b"},
	} {
		entries = append(entries, traffic.Request{Method: r.method, Path: r.path, Status: 200, Latency: 100 * time.Millisecond})
	}
	want := []string{"DELETE /a", "GET /a", "POST /a", "GET /b", "PUT /b", "GET /c"}
	// Endpoints aggregates through a map, so an order without a full
	// tie-break changes from call to call. Repeat to catch that.
	for run := 0; run < 50; run++ {
		got := Endpoints(entries)
		if len(got) != len(want) {
			t.Fatalf("got %d endpoints, want %d: %+v", len(got), len(want), got)
		}
		for i, e := range got {
			if key := e.Method + " " + e.Path; key != want[i] {
				t.Fatalf("run %d: endpoint %d = %q, want order %v (got %+v)", run, i, key, want, got)
			}
		}
	}
}
