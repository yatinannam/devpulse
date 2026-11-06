package timeline

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
	_ "time/tzdata" // real DST zones on every CI host

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

func TestBuildSkipsZeroTimeEntries(t *testing.T) {
	base := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	entries := []traffic.Request{
		{Status: 500, Latency: time.Second},
		{Time: base.Add(5 * time.Second), Status: 200, Latency: 10 * time.Millisecond},
		{Time: time.Time{}, Status: 200},
	}
	got := Build(entries, time.Minute)
	if len(got) != 1 {
		t.Fatalf("got %d buckets, want 1: %#v", len(got), got)
	}
	if got[0].Start.Year() != 2026 || got[0].Requests != 1 || got[0].Errors != 0 || got[0].Average != 10*time.Millisecond {
		t.Fatalf("zero-time entries leaked into the timeline: %#v", got[0])
	}

	data, err := json.Marshal(Build([]traffic.Request{{Status: 200}}, time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "[]" {
		t.Fatalf("only zero-time entries: got %s, want []", data)
	}
}

// withLocal runs fn with time.Local set to zone and restores it afterwards.
func withLocal(t *testing.T, zone *time.Location, fn func()) {
	t.Helper()
	saved := time.Local
	time.Local = zone
	defer func() { time.Local = saved }()
	fn()
}

func TestBuildAlignsBucketsToLocalWallClock(t *testing.T) {
	cases := []struct {
		name  string
		zone  *time.Location
		width time.Duration
		times []string // local wall-clock times on 2026-09-14
		want  []string // local bucket starts
	}{
		{"IST hourly", time.FixedZone("IST", 5*3600+30*60), time.Hour, []string{"10:47", "10:05", "11:15"}, []string{"10:00", "11:00"}},
		{"Newfoundland hourly", time.FixedZone("NST", -(3*3600 + 30*60)), time.Hour, []string{"09:59", "10:00", "10:30"}, []string{"09:00", "10:00"}},
		{"Nepal quarter hours", time.FixedZone("NPT", 5*3600+45*60), 15 * time.Minute, []string{"10:14", "10:15", "10:29"}, []string{"10:00", "10:15"}},
		{"UTC hourly", time.UTC, time.Hour, []string{"10:47", "11:15"}, []string{"10:00", "11:00"}},
		{"IST daily", time.FixedZone("IST", 5*3600+30*60), 24 * time.Hour, []string{"00:10", "23:50"}, []string{"00:00"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withLocal(t, tc.zone, func() {
				var entries []traffic.Request
				for _, clock := range tc.times {
					local, err := time.ParseInLocation("2006-01-02 15:04", "2026-09-14 "+clock, tc.zone)
					if err != nil {
						t.Fatal(err)
					}
					// Sessions store instants; decode them in UTC to show that
					// alignment follows time.Local, not the entry's zone.
					entries = append(entries, traffic.Request{Time: local.UTC(), Status: 200})
				}
				var got []string
				for _, bucket := range Build(entries, tc.width) {
					got = append(got, bucket.Start.In(time.Local).Format("15:04"))
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("bucket starts = %q, want %q", got, tc.want)
				}
			})
		})
	}
}

// A DST change inside a bucket must not move the bucket start: every request
// of a local day shares one bucket that starts at local midnight, and buckets
// never overlap. Real zones come from the embedded tzdata, so the test does
// not depend on the host's zoneinfo.
func TestBuildKeepsBucketsAlignedAcrossDSTChanges(t *testing.T) {
	newYork := loadZone(t, "America/New_York")
	lordHowe := loadZone(t, "Australia/Lord_Howe") // shifts by 30 minutes
	cases := []struct {
		name  string
		zone  *time.Location
		width time.Duration
		times []string // local wall-clock times with their UTC offset
		want  []string // bucket starts, local
	}{
		{"spring forward daily", newYork, 24 * time.Hour,
			[]string{"2026-03-08 00:30 -0500", "2026-03-08 12:00 -0400", "2026-03-08 23:59 -0400"},
			[]string{"2026-03-08 00:00 -0500"}},
		{"fall back daily", newYork, 24 * time.Hour,
			[]string{"2026-11-01 00:30 -0400", "2026-11-01 12:00 -0500"},
			[]string{"2026-11-01 00:00 -0400"}},
		{"spring forward 2h", newYork, 2 * time.Hour,
			[]string{"2026-03-08 01:30 -0500", "2026-03-08 03:15 -0400"},
			[]string{"2026-03-08 00:00 -0500", "2026-03-08 03:00 -0400"}},
		// The repeated 01:00 hour stays two separate one-hour buckets.
		{"fall back hourly", newYork, time.Hour,
			[]string{"2026-11-01 01:30 -0400", "2026-11-01 01:30 -0500"},
			[]string{"2026-11-01 01:00 -0400", "2026-11-01 01:00 -0500"}},
		{"half-hour shift hourly", lordHowe, time.Hour,
			[]string{"2026-10-04 01:45 +1030", "2026-10-04 02:45 +1100"},
			[]string{"2026-10-04 01:00 +1030", "2026-10-04 02:30 +1100"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withLocal(t, tc.zone, func() {
				var entries []traffic.Request
				for _, clock := range tc.times {
					local, err := time.ParseInLocation("2006-01-02 15:04 -0700", clock, tc.zone)
					if err != nil {
						t.Fatal(err)
					}
					entries = append(entries, traffic.Request{Time: local.UTC(), Status: 200})
				}
				var got []string
				for _, bucket := range Build(entries, tc.width) {
					got = append(got, bucket.Start.In(time.Local).Format("2006-01-02 15:04 -0700"))
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("bucket starts = %q, want %q", got, tc.want)
				}
			})
		})
	}
}

func loadZone(t *testing.T, name string) *time.Location {
	t.Helper()
	zone, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return zone
}
