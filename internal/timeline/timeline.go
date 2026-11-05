package timeline

import (
	"sort"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

type Bucket struct {
	Start    time.Time     `json:"start"`
	Requests int           `json:"requests"`
	Errors   int           `json:"errors"`
	Average  time.Duration `json:"average_ns"`
}

// Build groups captured requests into fixed-size chronological buckets.
func Build(entries []traffic.Request, width time.Duration) []Bucket {
	if len(entries) == 0 || width <= 0 {
		// Never nil: JSON consumers are promised `[]`, not null.
		return []Bucket{}
	}
	// Key by Unix nanoseconds, not time.Time: map equality on time.Time also
	// compares the *Location pointer, so the same instant decoded with two
	// distinct fixed zones (e.g. +05:30) would otherwise split into two buckets.
	buckets := map[int64]*Bucket{}
	for _, entry := range entries {
		// A zero time means the request was never timestamped; bucketing it
		// would add a year-0001 bucket ahead of the real traffic.
		if entry.Time.IsZero() {
			continue
		}
		start := bucketStart(entry.Time, width)
		key := start.UnixNano()
		b := buckets[key]
		if b == nil {
			b = &Bucket{Start: start}
			buckets[key] = b
		}
		b.Requests++
		if entry.Status >= 400 {
			b.Errors++
		}
		b.Average += entry.Latency
	}
	out := make([]Bucket, 0, len(buckets))
	for _, b := range buckets {
		b.Average /= time.Duration(b.Requests)
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}

// bucketStart aligns t to the local wall clock. time.Truncate works on
// absolute time, which aligns to UTC, so for a +05:30 user hourly buckets
// would start at :30 local. Shifting by the local offset of t's instant
// before truncating, and back afterwards, starts them on the local hour.
func bucketStart(t time.Time, width time.Duration) time.Time {
	_, offset := t.In(time.Local).Zone()
	shift := time.Duration(offset) * time.Second
	return t.Add(shift).Truncate(width).Add(-shift)
}
