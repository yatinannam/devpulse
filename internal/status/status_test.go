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
