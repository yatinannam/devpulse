package main

import (
	"bytes"
	"os"
	"runtime"
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
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 { t.Fatalf("report permissions are too broad: %o", info.Mode().Perm()) }
}

func TestRunReportRejectsNegativeTopAndMissingSession(t *testing.T) {
	if err := runReport("ignored", "-", "Report", -1, &bytes.Buffer{}); err == nil {
		t.Fatal("expected negative top to fail")
	}
	if err := runReport(filepath.Join(t.TempDir(), "missing.json"), "-", "Report", 1, &bytes.Buffer{}); err == nil {
		t.Fatal("expected missing session to fail")
	}
}


func TestRunReportWritesJSONToStdout(t *testing.T) {
	sessionPath := writeReportFixture(t)
	var out bytes.Buffer
	if err := runReportWithFormat(sessionPath, "-", "JSON Report", 1, "json", &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"title": "JSON Report"`, `"summary"`, `"status_codes"`, `"slowest_requests"`, `"path": "/broken"`} {
		if !strings.Contains(out.String(), expected) {
			t.Errorf("JSON report missing %q: %s", expected, out.String())
		}
	}
}

func TestRunReportWritesJSONFile(t *testing.T) {
	sessionPath := writeReportFixture(t)
	outputPath := filepath.Join(t.TempDir(), "report.json")
	if err := runReportWithFormat(sessionPath, outputPath, "File JSON", 0, "json", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"title": "File JSON"`) || strings.Contains(string(data), "# File JSON") {
		t.Fatalf("unexpected JSON report: %s", data)
	}
}

func TestRunReportRejectsUnsupportedFormat(t *testing.T) {
	if err := runReportWithFormat("ignored", "-", "Report", 1, "yaml", &bytes.Buffer{}); err == nil {
		t.Fatal("expected unsupported format to fail")
	}
}
