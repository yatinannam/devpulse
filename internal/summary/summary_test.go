package summary

import (
    "encoding/json"
    "testing"
    "time"

    "github.com/yatinannam/devpulse/internal/traffic"
)

func TestBuildReportsErrorRateAndPercentiles(t *testing.T) {
    entries := []traffic.Request{
        {Status: 200, Latency: 10 * time.Millisecond},
        {Status: 500, Latency: 20 * time.Millisecond},
        {Status: 200, Latency: 30 * time.Millisecond},
        {Status: 404, Latency: 40 * time.Millisecond},
    }
    got := Build(entries)
    if got.Total != 4 || got.Errors != 2 {
        t.Fatalf("got counts %+v", got)
    }
    if got.ErrorRate != 0.5 {
        t.Fatalf("got error rate %v, want 0.5", got.ErrorRate)
    }
    if got.P50 == 0 || got.P95 == 0 || got.P99 == 0 {
        t.Fatalf("expected populated percentiles: %+v", got)
    }
}

func TestSlowestDoesNotMutateInput(t *testing.T) {
    entries := []traffic.Request{{Latency: 10}, {Latency: 30}, {Latency: 20}}
    got := Slowest(entries, 2)
    if len(got) != 2 || got[0].Latency != 30 || got[1].Latency != 20 {
        t.Fatalf("unexpected slowest entries: %#v", got)
    }
    if entries[0].Latency != 10 {
        t.Fatalf("input slice was reordered")
    }
}

func TestReportJSONUsesStableFieldNames(t *testing.T) {
	report := Build([]traffic.Request{{Status: 500, Latency: 25 * time.Millisecond}})
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	for _, key := range []string{"total", "errors", "error_rate", "average", "p50", "p95", "p99"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("missing JSON field %q in %s", key, data)
		}
	}
}
