package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yatinannam/devpulse/internal/anomaly"
	"github.com/yatinannam/devpulse/internal/diff"
	"github.com/yatinannam/devpulse/internal/sessioncheck"
	"github.com/yatinannam/devpulse/internal/ui"
)

func TestHumanizeKind(t *testing.T) {
	cases := map[string]string{
		"service-added":      "Service added",
		"latency-regression": "Latency regression",
		"error-rate":         "Error rate",
		"":                   "",
	}
	for in, want := range cases {
		if got := humanizeKind(in); got != want {
			t.Errorf("humanizeKind(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDiffLevels(t *testing.T) {
	want := map[diff.Kind]ui.Level{
		diff.ServiceAdded:    ui.LevelOK,
		diff.EndpointAdded:   ui.LevelOK,
		diff.ServiceRemoved:  ui.LevelWarn,
		diff.ServiceChanged:  ui.LevelWarn,
		diff.EndpointRemoved: ui.LevelWarn,
		diff.TrafficChanged:  ui.LevelIdle,
		diff.EndpointChanged: ui.LevelIdle,
		diff.Kind("future"):  ui.LevelIdle,
	}
	for kind, level := range want {
		if got := diffLevel(kind); got != level {
			t.Errorf("diffLevel(%q) = %v, want %v", kind, got, level)
		}
	}
}

func TestRenderDiff(t *testing.T) {
	plainOutput(t)
	changes := []diff.Change{
		{Kind: diff.EndpointAdded, Subject: "GET /api/new", Message: "new endpoint; count 3, errors 0, average 52ms"},
		{Kind: diff.ServiceRemoved, Subject: ":8080", Message: "service HTTP service removed"},
		{Kind: diff.TrafficChanged, Subject: "traffic", Message: "requests 10→12, errors 1→3, average 200ms→226ms"},
	}
	var b strings.Builder
	renderDiff(&b, 100, "/tmp/session.json", ".devpulse/baseline.json", changes)
	row := func(label, subject, message string) string {
		return fmt.Sprintf("%-17s  %-12s  %s\n", label, subject, message)
	}
	want := titleLine(100, "diff", "session.json vs baseline.json") +
		row("● Endpoint added", "GET /api/new", "new endpoint; count 3, errors 0, average 52ms") +
		row("▲ Service removed", ":8080", "service HTTP service removed") +
		row("○ Traffic changed", "traffic", "requests 10→12, errors 1→3, average 200ms→226ms") +
		"\n3 changes\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRenderDiffWrapsLongMessages(t *testing.T) {
	plainOutput(t)
	changes := []diff.Change{{Kind: diff.EndpointChanged, Subject: "GET /api/reports/annual/:id/departments/engineering", Message: "count 4→6, errors 0→1, average 52.4ms→60.1ms, and a long tail to force a wrap"}}
	var b strings.Builder
	renderDiff(&b, 60, "session.json", "baseline.json", changes)
	assertWithin(t, b.String(), 60)
	if words := strings.Join(strings.Fields(b.String()), " "); !strings.Contains(words, "52.4ms→60.1ms, and a long tail to force a wrap") {
		t.Fatalf("a long message should wrap, not lose words:\n%s", b.String())
	}
}

func TestRenderDiffWithoutChanges(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	renderDiff(&b, 80, "session.json", "baseline.json", []diff.Change{})
	if !strings.HasSuffix(b.String(), "\n\n● No changes since the baseline.\n") {
		t.Fatalf("got\n%s", b.String())
	}
}

func TestRenderAnomalies(t *testing.T) {
	plainOutput(t)
	findings := []anomaly.Finding{
		{Severity: anomaly.SeverityError, Kind: "error-rate", Subject: "traffic", Message: "error rate increased from 5.0% to 25.0%"},
		{Severity: anomaly.SeverityWarning, Kind: "latency-regression", Subject: "GET /api/users", Message: "average latency increased from 50ms to 120ms"},
		{Severity: anomaly.SeverityInfo, Kind: "service-added", Subject: ":5173", Message: "service Node / Vite started"},
	}
	var b strings.Builder
	renderAnomalies(&b, 100, "session.json", "baseline.json", findings)
	row := func(label, subject, message string) string {
		return fmt.Sprintf("%-20s  %-14s  %s\n", label, subject, message)
	}
	want := titleLine(100, "anomalies", "session.json vs baseline.json") +
		row("✖ Error rate", "traffic", "error rate increased from 5.0% to 25.0%") +
		row("▲ Latency regression", "GET /api/users", "average latency increased from 50ms to 120ms") +
		row("○ Service added", ":5173", "service Node / Vite started") +
		"\n1 error, 1 warning, 1 info\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRenderAnomaliesWithoutFindings(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	renderAnomalies(&b, 80, "session.json", "baseline.json", nil)
	if !strings.HasSuffix(b.String(), "\n\n● No regressions against the baseline.\n") {
		t.Fatalf("got\n%s", b.String())
	}
}

func TestRenderValidationValid(t *testing.T) {
	plainOutput(t)
	var b strings.Builder
	renderValidation(&b, 80, "ci/demo.json", sessioncheck.Report{Valid: true, RequestCount: 12, Issues: []sessioncheck.Issue{}})
	want := titleLine(80, "validate", "demo.json, 12 requests") + "● Session is valid (12 requests).\n"
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRenderValidationInvalid(t *testing.T) {
	plainOutput(t)
	report := sessioncheck.Report{RequestCount: 3, Issues: []sessioncheck.Issue{
		{Field: "version", Code: "unsupported_version", Message: "session version must be 1"},
		{Request: 3, Field: "status", Code: "invalid_status", Message: "HTTP status must be between 100 and 599"},
	}}
	var b strings.Builder
	renderValidation(&b, 100, "demo.json", report)
	row := func(request, field, code, message string) string {
		return fmt.Sprintf("%7s  %-7s  %-19s  %s\n", request, field, code, message)
	}
	want := titleLine(100, "validate", "demo.json, 3 requests") +
		"✖ Session is invalid: 2 issues\n" +
		"\n" +
		row("Request", "Field", "Code", "Message") +
		row("-", "version", "unsupported_version", "session version must be 1") +
		row("3", "status", "invalid_status", "HTTP status must be between 100 and 599")
	if got := b.String(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}
