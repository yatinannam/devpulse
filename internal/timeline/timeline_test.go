package timeline

import (
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
    if got := Build(entries, 0); got != nil {
        t.Fatalf("got %#v for zero width, want nil", got)
    }
}
