package report

import (
	"sort"
	"time"

	"github.com/yatinannam/devpulse/internal/doctor"
	"github.com/yatinannam/devpulse/internal/summary"
	"github.com/yatinannam/devpulse/internal/traffic"
)

type Options struct {
	Title string
	SessionPath string
	Top int
	GeneratedAt time.Time
}

type StatusCount struct { Status int; Count int }

type Endpoint struct {
	Method string
	Path string
	Requests int
	Errors int
	ErrorRate float64
	Average time.Duration
}

type Report struct {
	Title string
	SessionPath string
	SessionCreatedAt time.Time
	GeneratedAt time.Time
	Summary summary.Report
	StatusCodes []StatusCount
	Endpoints []Endpoint
	Slowest []traffic.Request
	Findings []doctor.Finding
}

// Build creates a report from a saved traffic session.
func Build(session traffic.Session, opts Options) Report {
	title := opts.Title
	if title == "" { title = "DevPulse Traffic Report" }
	generated := opts.GeneratedAt
	if generated.IsZero() { generated = time.Now() }
	return Report{
		Title: title, SessionPath: opts.SessionPath, SessionCreatedAt: session.CreatedAt,
		GeneratedAt: generated, Summary: summary.Build(session.Requests), Endpoints: buildEndpoints(session.Requests),
	}
}

type endpointKey struct { method, path string }

func buildEndpoints(entries []traffic.Request) []Endpoint {
	groups := make(map[endpointKey]*Endpoint)
	totals := make(map[endpointKey]time.Duration)
	for _, entry := range entries {
		key := endpointKey{entry.Method, entry.Path}
		endpoint := groups[key]
		if endpoint == nil {
			endpoint = &Endpoint{Method: entry.Method, Path: entry.Path}
			groups[key] = endpoint
		}
		endpoint.Requests++
		if entry.Status >= 400 { endpoint.Errors++ }
		totals[key] += entry.Latency
	}
	out := make([]Endpoint, 0, len(groups))
	for key, endpoint := range groups {
		endpoint.Average = totals[key] / time.Duration(endpoint.Requests)
		endpoint.ErrorRate = float64(endpoint.Errors) / float64(endpoint.Requests)
		out = append(out, *endpoint)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Requests != out[j].Requests { return out[i].Requests > out[j].Requests }
		if out[i].Errors != out[j].Errors { return out[i].Errors > out[j].Errors }
		if out[i].Path != out[j].Path { return out[i].Path < out[j].Path }
		return out[i].Method < out[j].Method
	})
	return out
}
