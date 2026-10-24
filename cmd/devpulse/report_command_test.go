package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

func writeReportFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.json")
	err := traffic.SaveSession(path, []traffic.Request{
		{Time: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC), Method: "GET", Path: "/ok", Status: 200, Latency: 12*time.Millisecond, TargetHost: "localhost", TargetPort: 8080},
		{Time: time.Date(2026, 10, 1, 10, 0, 1, 0, time.UTC), Method: "GET", Path: "/broken", Status: 500, Latency: 900*time.Millisecond, TargetHost: "localhost", TargetPort: 8080},
	})
	if err != nil { t.Fatal(err) }
	return path
}

func TestRunReportWritesMarkdownToStdout(t *testing.T) {
	sessionPath := writeReportFixture(t)
	var out bytes.Buffer
	if err := runReport(sessionPath, "-", "Smoke Report", 1, &out); err != nil { t.Fatal(err) }
	for _, expected := range []string{"# Smoke Report", "## Summary", "## Top Endpoints", "/broken"} {
		if !strings.Contains(out.String(), expected) { t.Errorf("report missing %q: %s", expected, out.String()) }
	}
}

func TestRunReportWritesFileWithPrivatePermissions(t *testing.T) {
	sessionPath := writeReportFixture(t)
	outputPath := filepath.Join(t.TempDir(), "nested", "report.md")
	if err := runReport(sessionPath, outputPath, "File Report", 1, &bytes.Buffer{}); err != nil { t.Fatal(err) }
	data, err := os.ReadFile(outputPath)
	if err != nil { t.Fatal(err) }
	if !strings.Contains(string(data), "# File Report") { t.Fatalf("unexpected report: %s", data) }
	info, err := os.Stat(outputPath)
	if err != nil { t.Fatal(err) }
	if info.Mode().Perm()&0077 != 0 { t.Fatalf("report permissions are too broad: %o", info.Mode().Perm()) }
}

func TestRunReportRejectsNegativeTopAndMissingSession(t *testing.T) {
	if err := runReport("ignored", "-", "Report", -1, &bytes.Buffer{}); err == nil {
		t.Fatal("expected negative top to fail")
	}
	if err := runReport(filepath.Join(t.TempDir(), "missing.json"), "-", "Report", 1, &bytes.Buffer{}); err == nil {
		t.Fatal("expected missing session to fail")
	}
}
