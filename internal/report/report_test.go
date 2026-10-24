package report

import (
	"bytes"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

func sampleSession() traffic.Session {
	created := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	return traffic.Session{Version: 1, CreatedAt: created, Requests: []traffic.Request{
		{Time: created, Method: "GET", Path: "/health", Status: 200, Latency: 10*time.Millisecond, TargetHost: "localhost", TargetPort: 8080},
		{Time: created.Add(time.Second), Method: "GET", Path: "/api/items", Status: 500, Latency: 800*time.Millisecond, TargetHost: "localhost", TargetPort: 8080},
		{Time: created.Add(2*time.Second), Method: "GET", Path: "/api/items", Status: 200, Latency: 100*time.Millisecond, TargetHost: "localhost", TargetPort: 8080},
	}}
}

func TestBuildSummaryAndMetadata(t *testing.T) {
	generated := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	got := Build(sampleSession(), Options{Title: "CI Report", SessionPath: "session.json", Top: 2, GeneratedAt: generated})
	if got.Title != "CI Report" || got.SessionPath != "session.json" || !got.GeneratedAt.Equal(generated) {
		t.Fatalf("metadata mismatch: %+v", got)
	}
	if got.Summary.Total != 3 || got.Summary.Errors != 1 || got.Summary.ErrorRate < 0.333 || got.Summary.ErrorRate > 0.334 {
		t.Fatalf("summary mismatch: %+v", got.Summary)
	}
	if len(got.Slowest) != 2 || got.Slowest[0].Path != "/api/items" || got.Slowest[0].Latency != 800*time.Millisecond {
		t.Fatalf("slowest mismatch: %+v", got.Slowest)
	}
}

func TestRenderIncludesSections(t *testing.T) {
	got := Build(sampleSession(), Options{SessionPath: "session.json", Top: 3, GeneratedAt: time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)})
	var out bytes.Buffer
	if err := Render(&out, got); err != nil { t.Fatal(err) }
	for _, want := range []string{"# DevPulse Traffic Report", "## Summary", "## HTTP Status Codes", "## Top Endpoints", "## Slowest Requests", "## Diagnostic Findings"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) { t.Errorf("report missing %q", want) }
	}
}

func TestBuildEndpointsSortedByVolumeThenErrors(t *testing.T) {
	got := Build(sampleSession(), Options{Top: 5})
	if len(got.Endpoints) != 2 { t.Fatalf("got %d endpoints, want 2", len(got.Endpoints)) }
	if got.Endpoints[0].Path != "/api/items" || got.Endpoints[0].Requests != 2 || got.Endpoints[0].Errors != 1 {
		t.Fatalf("unexpected top endpoint: %+v", got.Endpoints[0])
	}
	if got.Endpoints[0].Average != 450*time.Millisecond || got.Endpoints[0].ErrorRate != 0.5 {
		t.Fatalf("unexpected endpoint metrics: %+v", got.Endpoints[0])
	}
}

func TestBuildEndpointOrderIsDeterministicForTies(t *testing.T) {
	session := sampleSession()
	session.Requests = append(session.Requests,
		traffic.Request{Time: session.CreatedAt, Method: "POST", Path: "/api/a", Status: 200, Latency: time.Millisecond},
		traffic.Request{Time: session.CreatedAt, Method: "GET", Path: "/api/a", Status: 200, Latency: time.Millisecond})
	got := Build(session, Options{Top: 5})
	for i := 1; i < len(got.Endpoints); i++ {
		prev, curr := got.Endpoints[i-1], got.Endpoints[i]
		if prev.Requests == curr.Requests && prev.Errors == curr.Errors && prev.Path > curr.Path {
			t.Fatalf("path order is not stable: %+v", got.Endpoints)
		}
	}
}
