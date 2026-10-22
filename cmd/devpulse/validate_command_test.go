package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

func TestRunValidateJSONReportsValidSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := traffic.SaveSession(path, []traffic.Request{{
		Time: time.Now(), Method: "GET", Path: "/health", Status: 200,
		Latency: time.Millisecond, TargetHost: "localhost", TargetPort: 8080,
	}}); err != nil { t.Fatal(err) }
	var out bytes.Buffer
	code, err := runValidate(path, true, &out)
	if err != nil || code != 0 { t.Fatalf("runValidate = (%d, %v)", code, err) }
	var payload struct {
		Session string `json:"session"`
		Report struct {
			Valid bool `json:"valid"`
			RequestCount int `json:"request_count"`
		} `json:"report"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil { t.Fatal(err) }
	if payload.Session != path || !payload.Report.Valid || payload.Report.RequestCount != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestRunValidateReturnsFailureForInvalidSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.json")
	data := []byte(`{"version":9,"createdAt":"2026-01-02T03:04:05Z","requests":[]}`)
	if err := os.WriteFile(path, data, 0600); err != nil { t.Fatal(err) }
	var out bytes.Buffer
	code, err := runValidate(path, true, &out)
	if err != nil || code != 1 { t.Fatalf("runValidate = (%d, %v), want (1, nil)", code, err) }
	var payload struct { Report struct { Valid bool `json:"valid"` } `json:"report"` }
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil { t.Fatal(err) }
	if payload.Report.Valid { t.Fatalf("expected invalid report: %s", out.String()) }
}

func TestRunValidateTextIncludesIssueDetails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(path, []byte(`{"version":9,"createdAt":"2026-01-02T03:04:05Z","requests":[]}`), 0600); err != nil { t.Fatal(err) }
	var out bytes.Buffer
	code, err := runValidate(path, false, &out)
	if err != nil || code != 1 { t.Fatalf("runValidate = (%d, %v)", code, err) }
	if !bytes.Contains(out.Bytes(), []byte("unsupported_version")) { t.Fatalf("expected issue code in output: %s", out.String()) }
}
