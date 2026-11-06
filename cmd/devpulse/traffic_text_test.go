package main

import (
	"flag"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/config"
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

func TestParseTrafficArgsRejectsBadListenAddressesAsUsageErrors(t *testing.T) {
	// An empty --listen made net.Listen bind every interface on a random
	// port, and the banner printed "http://" without saying which port.
	for _, args := range [][]string{
		{"--listen", ""},
		{"--listen", "9090"},
		{"--listen", "abc"},
		{"--listen", "127.0.0.1:"},
		{"--max-requests", "0"},
		// A target that is the proxy itself would loop.
		{"--listen", "127.0.0.1:9090", "--target", "http://127.0.0.1:9090"},
		{"--listen", "127.0.0.1:9090", "--target", "http://localhost:9090"},
	} {
		fs := flag.NewFlagSet("traffic", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		if _, err := parseTrafficArgs(fs, args, config.Default()); err == nil {
			t.Errorf("parseTrafficArgs(%q) succeeded, want a usage error", args)
		}
	}
	fs := flag.NewFlagSet("traffic", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	a, err := parseTrafficArgs(fs, []string{"--listen", "127.0.0.1:0"}, config.Default())
	if err != nil || a.Listen != "127.0.0.1:0" || a.Target != "http://localhost:3000" || a.MaxRequests != traffic.DefaultMaxRequests {
		t.Fatalf("parseTrafficArgs = %+v, %v", a, err)
	}
}

func TestStartTrafficProxyShowsTheBoundAddress(t *testing.T) {
	// The banner used to echo --listen, so --listen 127.0.0.1:0 printed
	// http://127.0.0.1:0 instead of the port the proxy actually got.
	plainOutput(t)
	var b strings.Builder
	server, _, err := startTrafficProxy(&b, 80, http.NotFoundHandler(), "127.0.0.1:0", "http://localhost:3000", "/tmp/session.json")
	if err != nil {
		t.Fatal(err)
	}
	defer traffic.Shutdown(server, 0)
	_, port, _ := net.SplitHostPort(server.Addr)
	if port == "0" || port == "" {
		t.Fatalf("server.Addr = %q, want a real port", server.Addr)
	}
	if want := "Proxy    http://127.0.0.1:" + port + " → http://localhost:3000\n"; !strings.Contains(b.String(), want) {
		t.Fatalf("banner\n%s\nwant it to contain %q", b.String(), want)
	}
}

func TestStartTrafficProxyPrintsNoBannerWhenTheBindFails(t *testing.T) {
	first, _, err := traffic.Start(http.NotFoundHandler(), "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer traffic.Shutdown(first, 0)
	var b strings.Builder
	if _, _, err := startTrafficProxy(&b, 80, http.NotFoundHandler(), first.Addr, "http://localhost:3000", "/tmp/session.json"); err == nil {
		t.Fatal("startTrafficProxy succeeded on a port that is in use")
	}
	if b.Len() != 0 {
		t.Fatalf("printed a banner for a proxy that never started: %q", b.String())
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
