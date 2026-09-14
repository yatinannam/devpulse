package timeline

import (
	"sort"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

type Bucket struct {
	Start   time.Time
	Requests int
	Errors   int
	Average time.Duration
}

// Build groups captured requests into fixed-size chronological buckets.
func Build(entries []traffic.Request, width time.Duration) []Bucket {
	if len(entries) == 0 || width <= 0 {
		return nil
	}
	buckets := map[time.Time]*Bucket{}
	for _, entry := range entries {
		start := entry.Time.Truncate(width)
		b := buckets[start]
		if b == nil {
			b = &Bucket{Start: start}
			buckets[start] = b
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
