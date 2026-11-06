package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/discovery"
	"github.com/yatinannam/devpulse/internal/traffic"
	"github.com/yatinannam/devpulse/internal/ui"
)

// plainOutput turns color off for the test and points config lookups at a
// file that does not exist, so empty-state hints use the default listen
// address.
func plainOutput(t *testing.T) {
	t.Helper()
	ui.SetColor(false)
	t.Setenv("DEVPULSE_CONFIG", filepath.Join(t.TempDir(), "missing.json"))
}

func assertWithin(t *testing.T, out string, width int) {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if ui.Width(line) > width {
			t.Errorf("line is %d wide, limit %d: %q", ui.Width(line), width, line)
		}
	}
}

var statusServices = []discovery.Service{
	{Port: 3000, HTTP: true, Kind: "Node / Next.js", Process: "node", PID: "46552"},
	{Port: 22, Process: "sshd", PID: "5884"},
}

func statusRequests() []traffic.Request {
	ms := time.Millisecond
	return []traffic.Request{
		{TargetPort: 3000, Method: "GET", Path: "/api/users", Status: 200, Latency: 52 * ms},
		{TargetPort: 3000, Method: "GET", Path: "/api/users", Status: 200, Latency: 52 * ms},
		{TargetPort: 3000, Method: "GET", Path: "/api/orders/1", Status: 500, Latency: 1300 * ms},
		{TargetPort: 3000, Method: "GET", Path: "/api/orders/2", Status: 200, Latency: 820 * ms},
	}
}

func TestRenderStatusBodyEndsWithTrafficFacts(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	renderStatusBody(&b, 120, statusServices, traffic.Session{Version: 1, Requests: statusRequests()}, nil, "demo.json")
	out := b.String()
	want := "\nTraffic   4 requests   1 error (25.0%)   avg 556ms   p50 52ms   p95 1.3s   p99 1.3s   2 slow\n"
	if !strings.HasSuffix(out, want) {
		t.Fatalf("got\n%s\nwant suffix\n%s", out, want)
	}
	if !strings.HasPrefix(out, "HTTP services\n  ✖ :3000") {
		t.Fatalf("services should come first:\n%s", out)
	}
}

func TestRenderStatusBodyWrapsTrafficFactsToTheWidth(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	renderStatusBody(&b, 60, statusServices, traffic.Session{Version: 1, Requests: statusRequests()}, nil, "demo.json")
	assertWithin(t, b.String(), 60)
	if !strings.Contains(b.String(), "\n          p50 52ms   p95 1.3s   p99 1.3s   2 slow\n") {
		t.Fatalf("facts should wrap under the first fact:\n%s", b.String())
	}
}

func TestRenderStatusBodyWithoutSessionGivesDirection(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	loadErr := fmt.Errorf("read session: %w", fs.ErrNotExist)
	renderStatusBody(&b, 120, statusServices, traffic.Session{}, loadErr, "demo.json")
	want := "\nNo captured traffic yet. Run devpulse traffic, then send requests to http://127.0.0.1:9090.\n"
	if !strings.HasSuffix(b.String(), want) {
		t.Fatalf("got\n%s\nwant suffix\n%s", b.String(), want)
	}
}

func TestRenderStatusBodyReportsAnUnreadableSession(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	renderStatusBody(&b, 120, statusServices, traffic.Session{}, fmt.Errorf("decode session: unexpected EOF"), "demo.json")
	if !strings.HasSuffix(b.String(), "\n▲ Could not read demo.json: decode session: unexpected EOF\n") {
		t.Fatalf("got\n%s", b.String())
	}
}

func TestProxyURLFor(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1:9090": "http://127.0.0.1:9090",
		":9090":          "http://127.0.0.1:9090",
		"0.0.0.0:8000":   "http://127.0.0.1:8000",
		"[::]:8000":      "http://127.0.0.1:8000",
		"localhost:9091": "http://localhost:9091",
		"not-an-address": "http://not-an-address",
	}
	for listen, want := range cases {
		if got := proxyURLFor(listen); got != want {
			t.Errorf("proxyURLFor(%q) = %q, want %q", listen, got, want)
		}
	}
}

func TestRatePulseTracksRequestsAddedPerRefresh(t *testing.T) {
	plainOutput(t)
	pulse := ratePulse{limit: 4}
	now := time.Date(2026, 11, 5, 10, 1, 2, 0, time.Local)
	pulse.observe(5)
	if got := pulse.context(now).Plain(); got != "updated 10:01:02" {
		t.Fatalf("first refresh has no rate yet, got %q", got)
	}
	for _, total := range []int{8, 8, 2, 4, 10} {
		pulse.observe(total)
	}
	// Added per refresh: 3, 0, 2 (a new, smaller session), 2, 6; the limit
	// keeps the last four.
	if got, want := pulse.context(now).Plain(), "▁▃▃█  6 new, updated 10:01:02"; got != want {
		t.Fatalf("context = %q, want %q", got, want)
	}
}

func TestRenderWatchFrame(t *testing.T) {
	plainOutput(t)
	pulse := ratePulse{limit: 30}
	pulse.observe(0)
	pulse.observe(4)
	now := time.Date(2026, 11, 5, 10, 1, 2, 0, time.Local)
	var b strings.Builder
	renderWatchFrame(&b, 80, now, 2*time.Second, &pulse, statusServices, traffic.Session{Version: 1, Requests: statusRequests()}, nil, "demo.json")
	out := b.String()
	if !strings.HasPrefix(out, "devpulse watch") || !strings.Contains(out, "█  4 new, updated 10:01:02\n\nHTTP services\n") {
		t.Fatalf("unexpected title area:\n%s", out)
	}
	if !strings.HasSuffix(out, "\n\nRefreshing every 2s. Press Ctrl+C to stop.\n") {
		t.Fatalf("unexpected footer:\n%s", out)
	}
	assertWithin(t, out, 80)
}
