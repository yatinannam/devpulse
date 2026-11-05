package timeline

import (
    "encoding/json"
    "testing"
    "time"

    "github.com/yatinannam/devpulse/internal/traffic"
)

func TestBuildOrdersBucketsAndCountsErrors(t *testing.T) {
    base := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
    entries := []traffic.Request{
        {Time: base.Add(70 * time.Second), Status: 200, Latency: 20 * time.Millisecond},
        {Time: base.Add(10 * time.Second), Status: 500, Latency: 100 * time.Millisecond},
        {Time: base.Add(130 * time.Second), Status: 404, Latency: 40 * time.Millisecond},
    }

    got := Build(entries, time.Minute)
    if len(got) != 3 {
        t.Fatalf("got %d buckets, want 3", len(got))
    }
    if !got[0].Start.Before(got[1].Start) || !got[1].Start.Before(got[2].Start) {
        t.Fatalf("buckets are not chronological: %#v", got)
    }
    if got[0].Requests != 1 || got[0].Errors != 1 {
        t.Fatalf("first bucket = %#v, want one request and one error", got[0])
    }
}

func TestBuildRejectsInvalidWidth(t *testing.T) {
    entries := []traffic.Request{{Time: time.Now(), Status: 200}}
    if got := Build(entries, 0); len(got) != 0 {
        t.Fatalf("got %#v for zero width, want no buckets", got)
    }
}

func TestBuildEmptyResultsMarshalAsEmptyArray(t *testing.T) {
	entries := []traffic.Request{{Time: time.Now(), Status: 200}}
	cases := map[string][]Bucket{
		"no entries":     Build(nil, time.Minute),
		"zero width":     Build(entries, 0),
		"negative width": Build(entries, -time.Second),
	}
	for name, got := range cases {
		data, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "[]" {
			t.Errorf("%s: got %s, want []", name, data)
		}
	}
}

func TestBuildMergesEqualInstantsFromDifferentLocations(t *testing.T) {
	// Sessions decoded from JSON give each non-whole-hour offset its own
	// *time.Location, so identical instants must still share one bucket.
	ist1 := time.FixedZone("", 5*3600+30*60)
	ist2 := time.FixedZone("", 5*3600+30*60)
	base := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	entries := []traffic.Request{
		{Time: base.Add(5 * time.Second).In(ist1), Status: 200, Latency: 10 * time.Millisecond},
		{Time: base.Add(15 * time.Second).In(ist2), Status: 200, Latency: 30 * time.Millisecond},
		{Time: base.Add(25 * time.Second), Status: 200, Latency: 20 * time.Millisecond},
	}
	got := Build(entries, time.Minute)
	if len(got) != 1 {
		t.Fatalf("got %d buckets, want 1: %#v", len(got), got)
	}
	if got[0].Requests != 3 {
		t.Fatalf("bucket has %d requests, want 3", got[0].Requests)
	}
}
