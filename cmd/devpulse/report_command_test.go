package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

func writeReportFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.json")
	err := traffic.SaveSession(path, []traffic.Request{
		{Time: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC), Method: "GET", Path: "/ok", Status: 200, Latency: 12 * time.Millisecond, TargetHost: "localhost", TargetPort: 8080},
		{Time: time.Date(2026, 10, 1, 10, 0, 1, 0, time.UTC), Method: "GET", Path: "/broken", Status: 500, Latency: 900 * time.Millisecond, TargetHost: "localhost", TargetPort: 8080},
	})
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunReportWritesMarkdownToStdout(t *testing.T) {
	sessionPath := writeReportFixture(t)
	var out bytes.Buffer
	if err := runReport(sessionPath, "-", "Smoke Report", 1, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# Smoke Report", "## Summary", "## Top Endpoints", "/broken"} {
		if !strings.Contains(out.String(), expected) {
			t.Errorf("report missing %q: %s", expected, out.String())
		}
	}
}

func TestRunReportWritesFileWithPrivatePermissions(t *testing.T) {
	sessionPath := writeReportFixture(t)
	outputPath := filepath.Join(t.TempDir(), "nested", "report.md")
	if err := runReport(sessionPath, outputPath, "File Report", 1, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# File Report") {
		t.Fatalf("unexpected report: %s", data)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		t.Fatalf("report permissions are too broad: %o", info.Mode().Perm())
	}
}

// replaceSyncFile swaps syncFile for the test and restores it afterwards.
func replaceSyncFile(t *testing.T, sync func(*os.File) error) {
	t.Helper()
	original := syncFile
	syncFile = sync
	t.Cleanup(func() { syncFile = original })
}

func TestRunReportFlushesBeforeRename(t *testing.T) {
	// report renamed its temporary file without fsync, so a crash could
	// leave an empty report, although README promises it never does.
	sessionPath := writeReportFixture(t)
	dir := t.TempDir()
	var flushed []string
	replaceSyncFile(t, func(f *os.File) error {
		flushed = append(flushed, filepath.Base(f.Name()))
		return f.Sync()
	})
	for _, format := range []string{"markdown", "json"} {
		flushed = nil
		if err := runReportWithFormat(sessionPath, filepath.Join(dir, "report."+format), "Report", 1, format, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		if len(flushed) != 1 || !strings.HasPrefix(flushed[0], ".devpulse-report-") {
			t.Fatalf("%s report: flushed %q, want the temporary report once", format, flushed)
		}
	}
}

func TestRunReportFailsAndLeavesNothingWhenTheFlushFails(t *testing.T) {
	sessionPath := writeReportFixture(t)
	dir := t.TempDir()
	replaceSyncFile(t, func(*os.File) error { return io.ErrShortWrite })
	if err := runReport(sessionPath, filepath.Join(dir, "report.md"), "Report", 1, &bytes.Buffer{}); err == nil {
		t.Fatal("expected a failed flush to fail the report")
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("want an empty directory after a failed flush, got %v (%v)", entries, err)
	}
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

func TestRunReportJSONDoesNotEscapeHTML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	err := traffic.SaveSession(path, []traffic.Request{
		{Time: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC), Method: "GET", Path: "/search?a=1&b=<x>", Status: 200, Latency: time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runReportWithFormat(path, "-", "A & B <report>", 1, "json", &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"title": "A & B <report>"`, `"path": "/search?a=1&b=<x>"`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("JSON report missing %s: %s", want, out.String())
		}
	}
}

func TestRunReportRejectsUnsupportedFormat(t *testing.T) {
	if err := runReportWithFormat("ignored", "-", "Report", 1, "yaml", &bytes.Buffer{}); err == nil {
		t.Fatal("expected unsupported format to fail")
	}
}

func TestParseReportArgsResolvesOutputPath(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"markdown default", nil, "devpulse-report.md"},
		{"json default", []string{"--format", "json"}, "devpulse-report.json"},
		{"explicit markdown name with json wins", []string{"--format", "json", "--out", "devpulse-report.md"}, "devpulse-report.md"},
		{"explicit markdown name with equals wins", []string{"--out=devpulse-report.md", "--format=json"}, "devpulse-report.md"},
		{"explicit path", []string{"--out", "artifacts/r.md"}, "artifacts/r.md"},
		{"dash is stdout", []string{"--format", "json", "--out", "-"}, "-"},
		{"empty is stdout", []string{"--out", ""}, "-"},
		{"empty with json is stdout", []string{"--format", "json", "--out="}, "-"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseReportArgs(flag.NewFlagSet("report", flag.ContinueOnError), tc.args)
			if err != nil {
				t.Fatal(err)
			}
			if got.Out != tc.want {
				t.Fatalf("Out = %q, want %q", got.Out, tc.want)
			}
		})
	}
}

func TestParseReportArgsReadsFlagsAndRejectsPositionalArguments(t *testing.T) {
	got, err := parseReportArgs(flag.NewFlagSet("report", flag.ContinueOnError),
		[]string{"--from", "s.json", "--title", "T", "--top", "7", "--format", "json"})
	if err != nil {
		t.Fatal(err)
	}
	if want := (reportArgs{From: "s.json", Out: "devpulse-report.json", Format: "json", Title: "T", Top: 7}); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if _, err := parseReportArgs(fs, []string{"session.json"}); err == nil {
		t.Fatal("expected a positional argument to be rejected")
	}
}

func TestRunReportEmptyOutputWritesToStdout(t *testing.T) {
	sessionPath := writeReportFixture(t)
	var out bytes.Buffer
	if err := runReportWithFormat(sessionPath, "", "Empty Out", 1, "markdown", &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "# Empty Out") {
		t.Fatalf("report was not written to stdout: %q", out.String())
	}
}

func TestParseReportArgsRejectsInvalidTopAndFormatAsUsageErrors(t *testing.T) {
	// Usage errors are reported by parseReportArgs so reportCommand exits 2,
	// like every other command, instead of 1 after trying to load a session.
	for _, args := range [][]string{{"--top", "-1"}, {"--format", "yaml"}, {"--format", "JSON "}} {
		fs := flag.NewFlagSet("report", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		if _, err := parseReportArgs(fs, args); err == nil {
			t.Errorf("parseReportArgs(%q) succeeded, want a usage error", args)
		}
	}
}
