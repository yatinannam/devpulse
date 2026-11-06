package main

import (
	"strings"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
	"github.com/yatinannam/devpulse/internal/ui"
)

func TestRenderTrafficStart(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	renderTrafficStart(&b, 80, "127.0.0.1:9090", "http://localhost:3000", "/home/dev/.devpulse/session.json")
	want := "devpulse traffic\n\n" +
		"Proxy    http://127.0.0.1:9090 → http://localhost:3000\n" +
		"Session  /home/dev/.devpulse/session.json\n" +
		"\n" +
		"Waiting for HTTP traffic. Press Ctrl+C to save and stop.\n" +
		"\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestTrafficLine(t *testing.T) {
	plainOutput(t)
	r := traffic.Request{Time: at(10, 0, 1), Method: "GET", Path: "/api/users", Status: 200, Latency: 52 * time.Millisecond, TargetHost: "localhost", TargetPort: 3000}
	got := trafficLine(80, r).Plain()
	want := "10:00:01  GET      /api/users" + strings.Repeat(" ", 20) + "  200    52ms  → localhost:3000"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	if ui.Width(got) != 80 {
		t.Fatalf("line is %d wide, want 80", ui.Width(got))
	}
}

func TestTrafficLineFitsNarrowTerminals(t *testing.T) {
	plainOutput(t)
	r := traffic.Request{Time: at(10, 0, 1), Method: "OPTIONS", Path: "/api/" + strings.Repeat("segment/", 20), Status: 503, Latency: 1300 * time.Millisecond, TargetHost: "localhost", TargetPort: 3000}
	got := trafficLine(60, r).Plain()
	if ui.Width(got) > 60 || !strings.Contains(got, "…  503    1.3s") {
		t.Fatalf("line should truncate the path and keep the numbers: %q", got)
	}
}

func TestTrafficLineTruncatesALongMethod(t *testing.T) {
	// Go accepts any token as a method. A 40-character one left no room for
	// the path, and strings.Repeat panicked on the negative padding.
	plainOutput(t)
	r := traffic.Request{Time: at(10, 0, 1), Method: strings.Repeat("X", 40), Path: "/api/users", Status: 500, Latency: 1300 * time.Millisecond, TargetHost: "localhost", TargetPort: 3000}
	var got string
	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("trafficLine panicked: %v", p)
			}
		}()
		got = trafficLine(60, r).Plain()
	}()
	if ui.Width(got) > 60 {
		t.Fatalf("line is %d wide, limit 60: %q", ui.Width(got), got)
	}
	if !strings.Contains(got, "  "+strings.Repeat("X", 11)+"…  ") || !strings.Contains(got, "500    1.3s") {
		t.Fatalf("method should be cut to 12 columns and the numbers kept: %q", got)
	}
	// A method a little longer than the 7-column field is still shown whole.
	r.Method = "PROPFIND"
	if got := trafficLine(80, r).Plain(); !strings.Contains(got, "  PROPFIND  /api/users") {
		t.Fatalf("an 8-letter method should not be cut: %q", got)
	}
}

func TestTrafficLineColorsStatusAndLatency(t *testing.T) {
	plainOutput(t)
	ui.SetColor(true)
	defer ui.SetColor(false)
	r := traffic.Request{Time: at(10, 0, 1), Method: "GET", Path: "/", Status: 404, Latency: 1300 * time.Millisecond, TargetHost: "localhost", TargetPort: 3000}
	got := trafficLine(80, r).String()
	if !strings.Contains(got, ui.Warn("404")) || !strings.Contains(got, ui.Error("1.3s")) {
		t.Fatalf("expected a yellow 404 and a red latency: %q", got)
	}
}

func TestRenderTrafficSaved(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	renderTrafficSaved(&b, 80, 12, "/home/dev/.devpulse/session.json", 0)
	if want := "● Saved 12 requests to /home/dev/.devpulse/session.json\n"; b.String() != want {
		t.Fatalf("got %q, want %q", b.String(), want)
	}
	b.Reset()
	renderTrafficSaved(&b, 80, 10000, "/home/dev/.devpulse/session.json", 3)
	want := "● Saved 10000 requests to /home/dev/.devpulse/session.json\n" +
		"▲ 3 older requests were dropped; raise --max-requests to keep more.\n"
	if b.String() != want {
		t.Fatalf("got\n%s\nwant\n%s", b.String(), want)
	}
}
