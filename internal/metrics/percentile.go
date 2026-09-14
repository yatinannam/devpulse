package metrics

import (
	"sort"
	"time"
)

// Percentiles returns p50, p95, and p99 for latency samples.
// It returns zero values for an empty sample set.
func Percentiles(values []time.Duration) (p50, p95, p99 time.Duration) {
	if len(values) == 0 {
		return 0, 0, 0
	}
	values = append([]time.Duration(nil), values...)
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	pick := func(percent float64) time.Duration {
		idx := int(percent*float64(len(values)-1) + 0.5)
		return values[idx]
	}
	return pick(0.50), pick(0.95), pick(0.99)
}
