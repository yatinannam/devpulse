package doctor

import (
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

func TestStrictProfileFlagsRepeatedRequestsEarlier(t *testing.T) {
	now := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	entries := []traffic.Request{
		{Time: now, Method: "GET", Path: "/poll", Status: 200},
		{Time: now.Add(time.Second), Method: "GET", Path: "/poll", Status: 200},
	}
	if got := AnalyzeWithProfile(entries, ProfileStrict); len(got) != 1 || got[0].Title != "Duplicate requests" {
		t.Fatalf("strict profile findings = %#v", got)
	}
	if got := AnalyzeWithProfile(entries, ProfileDefault); len(got) != 0 {
		t.Fatalf("default profile should not flag two requests: %#v", got)
	}
}

func TestPermissiveProfileIgnoresClientErrors(t *testing.T) {
	now := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	entries := []traffic.Request{{Time: now, Method: "GET", Path: "/missing", Status: 404}}
	if got := AnalyzeWithProfile(entries, ProfilePermissive); len(got) != 0 {
		t.Fatalf("permissive profile findings = %#v, want none for 404", got)
	}
	if got := AnalyzeWithProfile(entries, ProfileDefault); len(got) != 1 || got[0].Title != "HTTP errors" {
		t.Fatalf("default profile should report 404: %#v", got)
	}
}
