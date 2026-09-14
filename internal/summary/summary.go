package summary

import (
    "sort"
    "time"

    "github.com/yatinannam/devpulse/internal/metrics"
    "github.com/yatinannam/devpulse/internal/traffic"
)

type Report struct {
    Total      int
    Errors     int
    ErrorRate  float64
    Average    time.Duration
    P50        time.Duration
    P95        time.Duration
    P99        time.Duration
}

func Build(entries []traffic.Request) Report {
    var r Report
    if len(entries) == 0 { return r }
    latencies := make([]time.Duration, 0, len(entries))
    var total time.Duration
    for _, e := range entries {
        r.Total++
        if e.Status >= 400 { r.Errors++ }
        total += e.Latency
        latencies = append(latencies, e.Latency)
    }
    r.Average = total / time.Duration(r.Total)
    r.ErrorRate = float64(r.Errors) / float64(r.Total)
    r.P50, r.P95, r.P99 = metrics.Percentiles(latencies)
    return r
}

func Slowest(entries []traffic.Request, limit int) []traffic.Request {
    if limit <= 0 || len(entries) == 0 { return nil }
    out := append([]traffic.Request(nil), entries...)
    sort.SliceStable(out, func(i, j int) bool { return out[i].Latency > out[j].Latency })
    if limit > len(out) { limit = len(out) }
    return out[:limit]
}
