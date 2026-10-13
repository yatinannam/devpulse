package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/anomaly"
	"github.com/yatinannam/devpulse/internal/summary"
	"github.com/yatinannam/devpulse/internal/timeline"
	"github.com/yatinannam/devpulse/internal/traffic"
)

func TestSummaryPayloadJSONFields(t *testing.T) {
	payload := summaryPayload{
		Session: "session.json",
		Summary: summary.Report{Total: 2, Errors: 1, ErrorRate: 0.5, P50: time.Millisecond},
		Slowest: []traffic.Request{{Method: "GET", Path: "/slow"}},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal summary payload: %v", err)
	}
	assertJSONKeys(t, data, "session", "summary", "slowest")
}

func TestTimelinePayloadJSONFields(t *testing.T) {
	payload := timelinePayload{
		Session: "session.json",
		Bucket: time.Minute,
		Buckets: []timeline.Bucket{{
			Start: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC),
			Requests: 2, Errors: 1, Average: 25 * time.Millisecond,
		}},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal timeline payload: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal timeline payload: %v", err)
	}
	for _, key := range []string{"session", "bucket_width", "buckets"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("missing timeline payload field %q in %s", key, data)
		}
	}
	var buckets []map[string]json.RawMessage
	if err := json.Unmarshal(decoded["buckets"], &buckets); err != nil {
		t.Fatalf("unmarshal buckets: %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("got %d buckets, want 1", len(buckets))
	}
	for _, key := range []string{"start", "requests", "errors", "average_ns"} {
		if _, ok := buckets[0][key]; !ok {
			t.Errorf("missing bucket field %q in %s", key, data)
		}
	}
}

func TestAnomalyPayloadJSONFields(t *testing.T) {
	payload := anomalyPayload{
		Baseline: "baseline.json",
		Session: "session.json",
		Findings: []anomaly.Finding{{
			Severity: anomaly.SeverityWarning,
			Kind: "latency-regression",
			Subject: "GET /items/:id",
			Message: "latency increased",
		}},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal anomaly payload: %v", err)
	}
	assertJSONKeys(t, data, "baseline", "session", "findings")
}

func assertJSONKeys(t *testing.T, data []byte, keys ...string) {
	t.Helper()
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	for _, key := range keys {
		if _, ok := decoded[key]; !ok {
			t.Errorf("missing JSON field %q in %s", key, data)
		}
	}
}
