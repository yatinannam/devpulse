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
