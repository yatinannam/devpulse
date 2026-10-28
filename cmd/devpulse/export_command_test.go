package main

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
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
