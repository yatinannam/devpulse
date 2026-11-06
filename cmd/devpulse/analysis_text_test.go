package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/summary"
	"github.com/yatinannam/devpulse/internal/timeline"
	"github.com/yatinannam/devpulse/internal/traffic"
	"github.com/yatinannam/devpulse/internal/ui"
)

func at(hour, minute, second int) time.Time {
	return time.Date(2026, 11, 5, hour, minute, second, 0, time.Local)
}

func titleLine(width int, command, context string) string {
	left := "devpulse " + command
	return left + strings.Repeat(" ", width-len(left)-len(context)) + context + "\n\n"
}

func TestRenderRecentListsNewestFirst(t *testing.T) {
	plainOutput(t)
	ms := time.Millisecond
	recent := []traffic.Request{
		{Time: at(10, 0, 1), Method: "GET", Path: "/api/users", Status: 200, Latency: 52 * ms},
		{Time: at(10, 0, 2), Method: "POST", Path: "/api/login", Status: 401, Latency: 34 * ms},
		{Time: at(10, 0, 3), Method: "GET", Path: "/api/orders/43", Status: 500, Latency: 1300 * ms},
	}
	var b strings.Builder
	renderRecent(&b, 80, "/tmp/demo.json", 12, recent)

	row := func(cells ...any) string {
		return strings.TrimRight(fmt.Sprintf("%-8s  %-6s  %-14s  %6s  %7s", cells...), " ") + "\n"
	}
	want := titleLine(80, "recent", "demo.json, 12 requests") +
		row("Time", "Method", "Path", "Status", "Latency") +
		row("10:00:03", "GET", "/api/orders/43", "500", "1.3s") +
		row("10:00:02", "POST", "/api/login", "401", "34ms") +
		row("10:00:01", "GET", "/api/users", "200", "52ms") +
		"\nShowing 3 of 12 requests\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRenderRecentTruncatesThePathToTheWidth(t *testing.T) {
	plainOutput(t)
	recent := []traffic.Request{{Time: at(10, 0, 1), Method: "GET", Path: "/api/" + strings.Repeat("segment/", 20), Status: 503, Latency: 2 * time.Second}}
	var b strings.Builder
	renderRecent(&b, 60, "demo.json", 1, recent)
	assertWithin(t, b.String(), 60)
	if !strings.Contains(b.String(), "…     503       2s\n") {
		t.Fatalf("path should truncate and the numbers survive:\n%s", b.String())
	}
}

func TestRenderRecentColorsStatusAndLatency(t *testing.T) {
	plainOutput(t)
	ui.SetColor(true)
	defer ui.SetColor(false)
	recent := []traffic.Request{{Time: at(10, 0, 1), Method: "GET", Path: "/", Status: 503, Latency: 700 * time.Millisecond}}
	var b strings.Builder
	renderRecent(&b, 80, "demo.json", 1, recent)
	if !strings.Contains(b.String(), ui.Error("503")) || !strings.Contains(b.String(), ui.Warn("700ms")) {
		t.Fatalf("expected a red status and a yellow latency: %q", b.String())
	}
}

func TestRenderRecentWithoutTrafficGivesDirection(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	renderRecent(&b, 100, "demo.json", 0, nil)
	want := titleLine(100, "recent", "demo.json, 0 requests") +
		"No captured traffic yet. Run devpulse traffic, then send requests to http://127.0.0.1:9090.\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func summaryFixture() []traffic.Request {
	ms := time.Millisecond
	return []traffic.Request{
		{Time: at(10, 0, 1), Method: "GET", Path: "/api/users", Status: 200, Latency: 52 * ms},
		{Time: at(10, 0, 2), Method: "GET", Path: "/api/orders/43", Status: 500, Latency: 1300 * ms},
		{Time: at(10, 0, 3), Method: "GET", Path: "/api/orders/42", Status: 200, Latency: 820 * ms},
		{Time: at(10, 0, 4), Method: "POST", Path: "/api/login", Status: 401, Latency: 34 * ms},
	}
}

func TestRenderSummary(t *testing.T) {
	plainOutput(t)
	requests := summaryFixture()
	var b strings.Builder
	renderSummary(&b, 80, "demo.json", requests, summary.Build(requests), summary.Slowest(requests, 2))

	row := func(cells ...any) string {
		return strings.TrimRight(fmt.Sprintf("  %-8s  %-6s  %-14s  %6s  %7s", cells...), " ") + "\n"
	}
	want := titleLine(80, "summary", "demo.json, 4 requests") +
		"Requests  4\n" +
		"Errors    2 (50.0%)\n" +
		"Average   552ms\n" +
		"p50       52ms\n" +
		"p95       1.3s\n" +
		"p99       1.3s\n" +
		"Pulse     ▁█▆▁  latency in request order\n" +
		"\n" +
		"Slowest requests\n" +
		row("Time", "Method", "Path", "Status", "Latency") +
		row("10:00:02", "GET", "/api/orders/43", "500", "1.3s") +
		row("10:00:03", "GET", "/api/orders/42", "200", "820ms")
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRenderSummaryDrawsThePulseInTheAccentColor(t *testing.T) {
	plainOutput(t)
	ui.SetColor(true)
	defer ui.SetColor(false)
	requests := summaryFixture()
	var b strings.Builder
	renderSummary(&b, 80, "demo.json", requests, summary.Build(requests), nil)
	if !strings.Contains(b.String(), ui.Accent("▁█▆▁")) {
		t.Fatalf("expected an accent sparkline: %q", b.String())
	}
	if strings.Contains(b.String(), "Slowest requests") {
		t.Fatalf("--slowest 0 should omit the table: %q", b.String())
	}
}

func TestRenderSummaryResamplesALongPulseToFit(t *testing.T) {
	plainOutput(t)
	var requests []traffic.Request
	for i := 0; i < 500; i++ {
		requests = append(requests, traffic.Request{Time: at(10, 0, 0).Add(time.Duration(i) * time.Second), Method: "GET", Path: "/", Status: 200, Latency: time.Duration(i) * time.Millisecond})
	}
	var b strings.Builder
	renderSummary(&b, 60, "demo.json", requests, summary.Build(requests), summary.Slowest(requests, 3))
	assertWithin(t, b.String(), 60)
	if !strings.Contains(b.String(), "█  latency in request order\n") {
		t.Fatalf("the caption should survive resampling:\n%s", b.String())
	}
}

func TestRenderSummaryWithoutTrafficGivesDirection(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	renderSummary(&b, 100, "demo.json", nil, summary.Build(nil), summary.Slowest(nil, 5))
	if !strings.HasSuffix(b.String(), "\n\nNo captured traffic yet. Run devpulse traffic, then send requests to http://127.0.0.1:9090.\n") {
		t.Fatalf("got\n%s", b.String())
	}
}

func TestRenderTimelineDrawsABarPerBucket(t *testing.T) {
	plainOutput(t)
	buckets := []timeline.Bucket{
		{Start: at(10, 0, 0), Requests: 5, Errors: 1, Average: 50 * time.Millisecond},
		{Start: at(10, 1, 0), Requests: 3, Errors: 1, Average: 708 * time.Millisecond},
		{Start: at(10, 2, 0), Requests: 1, Average: 1200 * time.Millisecond},
	}
	var b strings.Builder
	renderTimeline(&b, 80, "demo.json", 9, time.Minute, buckets)

	row := func(start, requests, errors, avg string, bar int) string {
		return strings.TrimRight(fmt.Sprintf("%-8s  %8s  %6s  %5s  %s", start, requests, errors, avg, strings.Repeat("█", bar)), " ") + "\n"
	}
	want := titleLine(80, "timeline", "demo.json, 9 requests, 1m buckets") +
		strings.TrimRight(fmt.Sprintf("%-8s  %8s  %6s  %5s  %s", "Start", "Requests", "Errors", "Avg", "Pulse"), " ") + "\n" +
		row("10:00:00", "5", "1", "50ms", 40) +
		row("10:01:00", "3", "1", "708ms", 24) +
		row("10:02:00", "1", "0", "1.2s", 8)
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRenderTimelineShowsTheErrorShareInRed(t *testing.T) {
	plainOutput(t)
	ui.SetColor(true)
	defer ui.SetColor(false)
	buckets := []timeline.Bucket{{Start: at(10, 0, 0), Requests: 5, Errors: 1, Average: 50 * time.Millisecond}}
	var b strings.Builder
	renderTimeline(&b, 80, "demo.json", 5, time.Minute, buckets)
	if want := ui.Accent(strings.Repeat("█", 32)) + ui.Error(strings.Repeat("█", 8)); !strings.Contains(b.String(), want) {
		t.Fatalf("expected 32 accent cells then 8 red ones: %q", b.String())
	}
}

func TestRenderTimelineAddsTheDateWhenBucketsSpanDays(t *testing.T) {
	plainOutput(t)
	buckets := []timeline.Bucket{
		{Start: time.Date(2026, 11, 5, 23, 59, 0, 0, time.Local), Requests: 1},
		{Start: time.Date(2026, 11, 6, 0, 0, 0, 0, time.Local), Requests: 1},
	}
	var b strings.Builder
	renderTimeline(&b, 80, "demo.json", 2, time.Minute, buckets)
	if !strings.Contains(b.String(), "\n2026-11-05 23:59:00  ") || !strings.Contains(b.String(), "\n2026-11-06 00:00:00  ") {
		t.Fatalf("expected dated starts:\n%s", b.String())
	}
	assertWithin(t, b.String(), 80)
}

func TestBucketLabel(t *testing.T) {
	cases := map[time.Duration]string{
		time.Minute:             "1m",
		30 * time.Second:        "30s",
		90 * time.Second:        "1m30s",
		time.Hour:               "1h",
		90 * time.Minute:        "1h30m",
		1500 * time.Millisecond: "1.5s",
	}
	for in, want := range cases {
		if got := bucketLabel(in); got != want {
			t.Errorf("bucketLabel(%s) = %q, want %q", in, got, want)
		}
	}
}
