package status

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/discovery"
	"github.com/yatinannam/devpulse/internal/traffic"
	"github.com/yatinannam/devpulse/internal/ui"
)

func TestPrintServicesGroupsHTTPServicesAndOtherListeners(t *testing.T) {
	ui.SetColor(false)
	services := []discovery.Service{
		{Port: 8790, HTTP: true, Kind: "HTTP service", Process: "python", PID: "1234"},
		{Port: 445, Process: "System", PID: "4"},
		{Port: 3000, HTTP: true, Kind: "Node / Next.js", Process: "node", PID: "46552"},
		{Port: 22, Process: "sshd", PID: "5884"},
	}
	ms := time.Millisecond
	requests := []traffic.Request{
		{TargetPort: 3000, Method: "GET", Path: "/api/users", Status: 200, Latency: 52 * ms},
		{TargetPort: 3000, Method: "GET", Path: "/api/users", Status: 200, Latency: 52 * ms},
		{TargetPort: 3000, Method: "GET", Path: "/api/orders/1", Status: 500, Latency: 1300 * ms},
		{TargetPort: 3000, Method: "GET", Path: "/api/orders/2", Status: 200, Latency: 820 * ms},
	}
	var b strings.Builder
	PrintServices(&b, GroupByService(services, requests), 80)

	want := "" +
		"HTTP services\n" +
		"  ✖ :3000  Node / Next.js  node 46552   4 req  1 err  556ms avg  2 slow\n" +
		"      GET /api/orders/:id               2      1      1.06s      slow\n" +
		"      GET /api/users                    2      0       52ms\n" +
		"  ● :8790  HTTP service    python 1234  0 req\n" +
		"\n" +
		"Other listeners (2)\n" +
		"  :22 sshd  :445 System\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestPrintServicesMarksSlowServicesWithAWord(t *testing.T) {
	ui.SetColor(false)
	services := []discovery.Service{{Port: 5173, HTTP: true, Kind: "Node / Vite", Process: "node", PID: "9"}}
	requests := []traffic.Request{
		{TargetPort: 5173, Method: "GET", Path: "/", Status: 200, Latency: 700 * time.Millisecond},
		{TargetPort: 5173, Method: "GET", Path: "/", Status: 200, Latency: 100 * time.Millisecond},
	}
	var b strings.Builder
	PrintServices(&b, GroupByService(services, requests), 80)
	if !strings.Contains(b.String(), "  ▲ :5173  Node / Vite  node 9  2 req  0 err  400ms avg  1 slow\n") {
		t.Fatalf("expected a warning row naming the slow request:\n%s", b.String())
	}
}

func TestPrintServicesCapsEndpointsPerService(t *testing.T) {
	ui.SetColor(false)
	services := []discovery.Service{{Port: 3000, HTTP: true, Kind: "Node / Express", Process: "node", PID: "1"}}
	var requests []traffic.Request
	for i := 0; i < 8; i++ {
		requests = append(requests, traffic.Request{TargetPort: 3000, Method: "GET", Path: fmt.Sprintf("/route%d", i), Status: 200, Latency: time.Millisecond})
	}
	var b strings.Builder
	PrintServices(&b, GroupByService(services, requests), 80)
	out := b.String()
	if n := strings.Count(out, "      GET /route"); n != 5 {
		t.Fatalf("got %d endpoint rows, want 5:\n%s", n, out)
	}
	if !strings.Contains(out, "      + 3 more endpoints\n") {
		t.Fatalf("expected a count of the hidden endpoints:\n%s", out)
	}
}

func TestPrintServicesKeepsLongPathsFromPushingNumbersAway(t *testing.T) {
	ui.SetColor(false)
	services := []discovery.Service{{Port: 3000, HTTP: true, Kind: "Node / Next.js", Process: "node", PID: "46552"}}
	requests := []traffic.Request{
		{TargetPort: 3000, Method: "GET", Path: "/api/reports/annual/2026/departments/engineering/summary", Status: 200, Latency: 95 * time.Millisecond},
	}
	var b strings.Builder
	PrintServices(&b, GroupByService(services, requests), 160)
	want := "" +
		"HTTP services\n" +
		"  ● :3000  Node / Next.js  node 46552       1 req  0 err  95ms avg\n" +
		"      GET /api/reports/annual/:id/departm…  1      0      95ms\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestPrintServicesWithoutHTTPServicesGivesDirection(t *testing.T) {
	ui.SetColor(false)
	var b strings.Builder
	PrintServices(&b, GroupByService([]discovery.Service{{Port: 22, Process: "sshd"}}, nil), 80)
	want := "" +
		"HTTP services\n" +
		"  No HTTP services found. Start your app, then run devpulse status again.\n" +
		"\n" +
		"Other listeners (1)\n" +
		"  :22 sshd\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestPrintServicesFitsNarrowTerminals(t *testing.T) {
	ui.SetColor(false)
	services := []discovery.Service{
		{Port: 3000, HTTP: true, Kind: "Node / Next.js", Process: "SamsungFindWindowsService", PID: "46552"},
		{Port: 49664, Process: "OneDrive.Sync.Service.With.A.Long.Name", PID: "1"},
	}
	requests := []traffic.Request{
		{TargetPort: 3000, Method: "DELETE", Path: "/api/reports/annual/2026/departments/engineering/summary", Status: 503, Latency: 2 * time.Second},
	}
	var b strings.Builder
	PrintServices(&b, GroupByService(services, requests), 60)
	for _, line := range strings.Split(b.String(), "\n") {
		if ui.Width(line) > 60 {
			t.Errorf("line exceeds 60 columns: %q", line)
		}
	}
	if !strings.Contains(b.String(), "1 req  1 err  2s avg") {
		t.Fatalf("numbers must survive truncation:\n%s", b.String())
	}
}

func TestHealthMarkerGlyphs(t *testing.T) {
	ui.SetColor(false)
	want := map[string]string{"healthy": "●", "warning": "▲", "error": "✖", "running": "○"}
	for health, glyph := range want {
		if got := healthMarker(health).Plain(); got != glyph {
			t.Errorf("healthMarker(%q) = %q, want %q", health, got, glyph)
		}
	}
}
