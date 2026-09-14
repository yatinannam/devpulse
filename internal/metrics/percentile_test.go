package metrics

import (
	"testing"
	"time"
)

func TestPercentilesEmpty(t *testing.T) {
	p50, p95, p99 := Percentiles(nil)
	if p50 != 0 || p95 != 0 || p99 != 0 {
		t.Fatalf("unexpected empty percentiles: %v %v %v", p50, p95, p99)
	}
}

func TestPercentilesUsesSortedSamples(t *testing.T) {
	values := []time.Duration{500 * time.Millisecond, 100 * time.Millisecond, 900 * time.Millisecond, 200 * time.Millisecond, 300 * time.Millisecond}
	p50, p95, p99 := Percentiles(values)
	if p50 != 300*time.Millisecond || p95 != 900*time.Millisecond || p99 != 900*time.Millisecond {
		t.Fatalf("unexpected percentiles: %v %v %v", p50, p95, p99)
	}
}
