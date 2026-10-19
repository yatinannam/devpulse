package doctor

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

func TestFindingsExposeStableJSONSchema(t *testing.T) {
	got := AnalyzeWithProfile([]traffic.Request{{
		Time: time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC),
		Method: "GET", Path: "/api", Status: 503, Latency: time.Second,
	}}, ProfileDefault)
	if len(got) == 0 {
		t.Fatal("expected diagnostic findings")
	}
	data, err := json.Marshal(got[0])
	if err != nil {
		t.Fatalf("marshal finding: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("decode finding JSON: %v", err)
	}
	for _, key := range []string{"level", "title", "detail", "advice"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("finding JSON missing %q: %s", key, data)
		}
	}
}

func TestAnalyzeWithProfileEmptySessionReturnsNoFindings(t *testing.T) {
	for _, profile := range []Profile{ProfileDefault, ProfileStrict, ProfilePermissive} {
		if got := AnalyzeWithProfile(nil, profile); len(got) != 0 {
			t.Errorf("profile %q returned findings for empty session: %#v", profile, got)
		}
	}
}
