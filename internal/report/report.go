package report

import (
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
		GeneratedAt: generated, Summary: summary.Build(session.Requests),
	}
}
