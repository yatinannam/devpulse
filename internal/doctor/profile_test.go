package doctor

import (
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

func TestThresholdsForProfiles(t *testing.T) {
	tests := []struct {
		profile Profile
		slow time.Duration
		status int
		minDuplicates int
	}{
		{ProfileDefault, 500 * time.Millisecond, 400, 3},
		{ProfileStrict, 250 * time.Millisecond, 400, 2},
		{ProfilePermissive, time.Second, 500, 4},
	}
	for _, test := range tests {
		got, err := ThresholdsFor(test.profile)
		if err != nil { t.Fatalf("ThresholdsFor(%q): %v", test.profile, err) }
		if got.SlowRequest != test.slow || got.ErrorStatus != test.status || got.MinDuplicateRequests != test.minDuplicates {
			t.Errorf("ThresholdsFor(%q) = %+v", test.profile, got)
		}
	}
	if _, err := ThresholdsFor("unknown"); err == nil {
		t.Fatal("unknown profile should return an error")
	}
}

func TestAnalyzeWithProfileChangesSlowThreshold(t *testing.T) {
	now := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	entries := []traffic.Request{{Time: now, Method: "GET", Path: "/health", Status: 200, Latency: 300 * time.Millisecond}}
	strict := AnalyzeWithProfile(entries, ProfileStrict)
	permissive := AnalyzeWithProfile(entries, ProfilePermissive)
	if len(strict) != 1 || strict[0].Title != "Slow requests" {
		t.Fatalf("strict findings = %#v, want slow-request finding", strict)
	}
	if len(permissive) != 0 {
		t.Fatalf("permissive findings = %#v, want none", permissive)
	}
}

func TestAnalyzeWithProfileSortsDuplicateFindingsDeterministically(t *testing.T) {
	now := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	var entries []traffic.Request
	for _, path := range []string{"/z", "/a"} {
		for i := 0; i < 3; i++ {
			entries = append(entries, traffic.Request{Time: now.Add(time.Duration(i) * time.Second), Method: "GET", Path: path, Status: 200})
		}
	}
	got := AnalyzeWithProfile(entries, ProfileDefault)
	if len(got) != 2 || got[0].Detail == "" || got[1].Detail == "" {
		t.Fatalf("unexpected findings: %#v", got)
	}
	if got[0].Detail > got[1].Detail {
		t.Fatalf("findings should be deterministic: %#v", got)
	}
}
