package main

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

func writeExportFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.json")
	entries := []traffic.Request{
		{Time: time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC), Method: "GET", Path: "/api/items", Status: 200, Latency: 1250 * time.Microsecond, Bytes: 24, TargetHost: "localhost", TargetPort: 8080},
		{Time: time.Date(2026, 10, 28, 10, 1, 0, 0, time.UTC), Method: "POST", Path: "/api/items?name=a,b", Status: 500, Latency: 1500 * time.Millisecond, Bytes: 7, TargetHost: "127.0.0.1", TargetPort: 3000},
	}
	if err := traffic.SaveSession(path, entries); err != nil {
		t.Fatal(err)
	}
	return path
}

func readExportCSV(t *testing.T, data string) [][]string {
	t.Helper()
	records, err := csv.NewReader(bytes.NewBufferString(data)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func TestRunExportWritesCSVHeaderAndRequests(t *testing.T) {
	sessionPath := writeExportFixture(t)
	var out bytes.Buffer
	if err := runExport(sessionPath, "-", exportOptions{}, &out); err != nil {
		t.Fatal(err)
	}
	records := readExportCSV(t, out.String())
	if len(records) != 3 {
		t.Fatalf("expected header and two request rows, got %d", len(records))
	}
	wantHeader := []string{"timestamp", "method", "path", "status", "latency_ms", "bytes", "target_host", "target_port"}
	if !reflect.DeepEqual(records[0], wantHeader) {
		t.Fatalf("unexpected CSV header: %#v", records[0])
	}
	if records[1][1] != "GET" || records[1][2] != "/api/items" || records[1][3] != "200" || records[1][4] != "1.250" {
		t.Fatalf("unexpected first data row: %#v", records[1])
	}
	if records[2][2] != "/api/items?name=a,b" || records[2][6] != "127.0.0.1" || records[2][7] != "3000" {
		t.Fatalf("unexpected second data row: %#v", records[2])
	}
}

func TestRunExportHandlesMissingSession(t *testing.T) {
	var out bytes.Buffer
	err := runExport(filepath.Join(t.TempDir(), "missing.json"), "-", exportOptions{}, &out)
	if err == nil {
		t.Fatal("expected missing session to fail")
	}
}

func TestRunExportWritesFile(t *testing.T) {
	sessionPath := writeExportFixture(t)
	outputPath := filepath.Join(t.TempDir(), "traffic.csv")
	if err := runExport(sessionPath, outputPath, exportOptions{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	records := readExportCSV(t, string(data))
	if len(records) != 3 {
		t.Fatalf("expected two rows in output file, got %d CSV records", len(records)-1)
	}
}


func TestRunExportFiltersHTTPMethodCaseInsensitively(t *testing.T) {
	sessionPath := writeExportFixture(t)
	var out bytes.Buffer
	if err := runExport(sessionPath, "-", exportOptions{Method: " post "}, &out); err != nil {
		t.Fatal(err)
	}
	records := readExportCSV(t, out.String())
	if len(records) != 2 || records[1][1] != "POST" {
		t.Fatalf("expected only POST request, got %#v", records)
	}
}

func TestRunExportReturnsHeaderWhenNoRequestsMatch(t *testing.T) {
	sessionPath := writeExportFixture(t)
	var out bytes.Buffer
	if err := runExport(sessionPath, "-", exportOptions{Method: "DELETE"}, &out); err != nil {
		t.Fatal(err)
	}
	records := readExportCSV(t, out.String())
	if len(records) != 1 || records[0][0] != "timestamp" {
		t.Fatalf("expected header-only CSV, got %#v", records)
	}
}


func TestRunExportFiltersByStatusAndPathPrefix(t *testing.T) {
	sessionPath := writeExportFixture(t)
	var out bytes.Buffer
	if err := runExport(sessionPath, "-", exportOptions{Status: 500, PathPrefix: "/api/items?name"}, &out); err != nil {
		t.Fatal(err)
	}
	records := readExportCSV(t, out.String())
	if len(records) != 2 || records[1][1] != "POST" || records[1][3] != "500" {
		t.Fatalf("expected only matching 500 request, got %#v", records)
	}
}

func TestRunExportRejectsInvalidStatusFilter(t *testing.T) {
	sessionPath := writeExportFixture(t)
	for _, status := range []int{-1, 99, 600} {
		err := runExport(sessionPath, "-", exportOptions{Status: status}, &bytes.Buffer{})
		if err == nil {
			t.Errorf("expected status %d to be rejected", status)
		}
	}
}


func TestRunExportLimitsMatchingRows(t *testing.T) {
	sessionPath := writeExportFixture(t)
	var out bytes.Buffer
	if err := runExport(sessionPath, "-", exportOptions{Limit: 1}, &out); err != nil {
		t.Fatal(err)
	}
	records := readExportCSV(t, out.String())
	if len(records) != 2 || records[1][1] != "GET" {
		t.Fatalf("expected only the first request after limiting, got %#v", records)
	}
}

func TestRunExportRejectsNegativeLimit(t *testing.T) {
	sessionPath := writeExportFixture(t)
	if err := runExport(sessionPath, "-", exportOptions{Limit: -1}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected negative limit to fail")
	}
}


func TestRunExportFiltersInclusiveTimeWindow(t *testing.T) {
	sessionPath := writeExportFixture(t)
	since := time.Date(2026, 10, 28, 10, 1, 0, 0, time.UTC)
	until := since
	var out bytes.Buffer
	if err := runExport(sessionPath, "-", exportOptions{Since: since, Until: until}, &out); err != nil {
		t.Fatal(err)
	}
	records := readExportCSV(t, out.String())
	if len(records) != 2 || records[1][1] != "POST" {
		t.Fatalf("expected request exactly on the time boundary, got %#v", records)
	}
}

func TestRunExportRejectsReversedTimeWindow(t *testing.T) {
	sessionPath := writeExportFixture(t)
	since := time.Date(2026, 10, 28, 10, 1, 0, 0, time.UTC)
	until := since.Add(-time.Second)
	if err := runExport(sessionPath, "-", exportOptions{Since: since, Until: until}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected reversed time range to fail")
	}
}

func TestRunExportProtectsInputSessionAndUsesPrivateFilePermissions(t *testing.T) {
	sessionPath := writeExportFixture(t)
	if err := runExport(sessionPath, sessionPath, exportOptions{}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected export to reject overwriting the input session")
	}
	if _, err := traffic.LoadSession(sessionPath); err != nil {
		t.Fatalf("input session was changed by rejected export: %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), "private", "traffic.csv")
	if err := runExport(sessionPath, outputPath, exportOptions{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		t.Fatalf("CSV permissions are too broad: %o", info.Mode().Perm())
	}
}


func TestRunExportEscapesQuotedAndMultilineCSVFields(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	pathValue := "/api/items/\"quoted\"\nnext-line"
	entry := traffic.Request{
		Time: time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC),
		Method: "GET", Path: pathValue, Status: 200, Latency: 2 * time.Millisecond,
	}
	if err := traffic.SaveSession(sessionPath, []traffic.Request{entry}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runExport(sessionPath, "-", exportOptions{}, &out); err != nil {
		t.Fatal(err)
	}
	records := readExportCSV(t, out.String())
	if len(records) != 2 || records[1][2] != pathValue {
		t.Fatalf("CSV did not round-trip quoted/newline path: %#v", records)
	}
}


func TestParseExportTime(t *testing.T) {
	t.Run("empty means unset", func(t *testing.T) {
		got, err := parseExportTime("--since", "")
		if err != nil || !got.IsZero() {
			t.Fatalf("expected zero time and no error, got %v, %v", got, err)
		}
	})
	t.Run("RFC3339 timestamp", func(t *testing.T) {
		got, err := parseExportTime("--since", "2026-10-28T12:00:00+02:00")
		if err != nil {
			t.Fatal(err)
		}
		if !got.Equal(time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)) {
			t.Fatalf("unexpected parsed instant: %v", got)
		}
	})
	t.Run("invalid timestamp", func(t *testing.T) {
		if _, err := parseExportTime("--until", "yesterday"); err == nil {
			t.Fatal("expected invalid timestamp to fail")
		}
	})
}

func TestBuildExportOptionsValidatesWithoutLoadingSession(t *testing.T) {
	got, err := buildExportOptions("post", 500, "/api/", "2026-10-28T10:00:00Z", "2026-10-28T11:00:00Z", 3)
	if err != nil {
		t.Fatal(err)
	}
	want := exportOptions{
		Method: "post", Status: 500, PathPrefix: "/api/",
		Since: time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 10, 28, 11, 0, 0, 0, time.UTC),
		Limit: 3,
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	cases := []struct {
		name         string
		status       int
		since, until string
		limit        int
		wantErr      string
	}{
		{name: "negative limit", limit: -1, wantErr: "--limit"},
		{name: "reversed window", since: "2026-10-28T11:00:00Z", until: "2026-10-28T10:00:00Z", wantErr: "--since"},
		{name: "invalid status", status: 99, wantErr: "--status"},
		{name: "invalid since", since: "yesterday", wantErr: "--since"},
		{name: "invalid until", until: "tomorrow", wantErr: "--until"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildExportOptions("", tc.status, "", tc.since, tc.until, tc.limit)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want one mentioning %s", err, tc.wantErr)
			}
		})
	}
}

func TestRunExportValidatesOptionsBeforeLoadingSession(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.json")
	since := time.Date(2026, 10, 28, 10, 1, 0, 0, time.UTC)
	cases := map[string]struct {
		opts    exportOptions
		wantErr string
	}{
		"negative limit":  {exportOptions{Limit: -1}, "--limit"},
		"reversed window": {exportOptions{Since: since, Until: since.Add(-time.Second)}, "--since"},
		"invalid status":  {exportOptions{Status: 600}, "--status"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := runExport(missing, "-", tc.opts, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want the %s usage error before the missing session", err, tc.wantErr)
			}
		})
	}
}
