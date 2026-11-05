package doctor

import (
	"github.com/yatinannam/devpulse/internal/traffic"
	"strings"
	"testing"
	"time"
)

func TestAnalyzeDuplicateRequests(t *testing.T) {
	now := time.Now()
	entries := []traffic.Request{
		{Time: now, Method: "GET", Path: "/api/users", Status: 200},
		{Time: now.Add(time.Second), Method: "GET", Path: "/api/users", Status: 200},
		{Time: now.Add(2 * time.Second), Method: "GET", Path: "/api/users", Status: 200},
	}
	findings := Analyze(entries)
	if len(findings) != 1 || findings[0].Title != "Duplicate requests" {
		t.Fatalf("unexpected findings: %#v", findings)
	}
}

func TestAnalyzeDetectsSustainedRequestLoop(t *testing.T) {
	// A runaway effect hitting the same endpoint every 100ms for 10s spans
	// more than the 3s window end to end, but every 3s slice holds ~30 calls.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var entries []traffic.Request
	for i := 0; i < 100; i++ {
		entries = append(entries, traffic.Request{Time: base.Add(time.Duration(i) * 100 * time.Millisecond), Method: "GET", Path: "/api/users", Status: 200})
	}
	findings := Analyze(entries)
	if len(findings) != 1 || findings[0].Title != "Duplicate requests" {
		t.Fatalf("expected a duplicate-request finding, got %#v", findings)
	}
}

func TestAnalyzeDetectsBurstFollowedByLateRequest(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var entries []traffic.Request
	for i := 0; i < 5; i++ {
		entries = append(entries, traffic.Request{Time: base.Add(time.Duration(i) * 200 * time.Millisecond), Method: "GET", Path: "/api/users", Status: 200})
	}
	entries = append(entries, traffic.Request{Time: base.Add(5 * time.Minute), Method: "GET", Path: "/api/users", Status: 200})
	findings := Analyze(entries)
	if len(findings) != 1 || findings[0].Title != "Duplicate requests" {
		t.Fatalf("expected a duplicate-request finding, got %#v", findings)
	}
	if !strings.Contains(findings[0].Detail, "called 5 times") {
		t.Fatalf("finding should describe the 5-request burst, got %q", findings[0].Detail)
	}
}

func TestAnalyzeIgnoresSpreadOutRequests(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var entries []traffic.Request
	for i := 0; i < 10; i++ {
		entries = append(entries, traffic.Request{Time: base.Add(time.Duration(i) * 2 * time.Second), Method: "GET", Path: "/api/users", Status: 200})
	}
	if findings := Analyze(entries); len(findings) != 0 {
		t.Fatalf("requests 2s apart should not trip a 3-in-3s rule, got %#v", findings)
	}
}
