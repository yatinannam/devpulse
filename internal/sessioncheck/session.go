package sessioncheck

import (
	"strings"

	"github.com/yatinannam/devpulse/internal/traffic"
)

type Issue struct {
	Request int    `json:"request,omitempty"`
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Report struct {
	Valid        bool    `json:"valid"`
	RequestCount int     `json:"request_count"`
	Issues       []Issue `json:"issues"`
}

// Validate checks session metadata and captured-request invariants without mutating the session.
func Validate(s traffic.Session) Report {
	r := Report{Valid: true, RequestCount: len(s.Requests), Issues: []Issue{}}
	add := func(issue Issue) {
		r.Valid = false
		r.Issues = append(r.Issues, issue)
	}
	if s.Version != 1 {
		add(Issue{Field: "version", Code: "unsupported_version", Message: "session version must be 1"})
	}
	if s.CreatedAt.IsZero() {
		add(Issue{Field: "createdAt", Code: "missing_created_at", Message: "session creation time is required"})
	}
	for i, req := range s.Requests {
		n := i + 1
		if req.Time.IsZero() {
			add(Issue{Request: n, Field: "time", Code: "missing_timestamp", Message: "request timestamp is required"})
		}
		if strings.TrimSpace(req.Method) == "" {
			add(Issue{Request: n, Field: "method", Code: "missing_method", Message: "request method must not be empty"})
		}
		if path := strings.TrimSpace(req.Path); path == "" || (path != "*" && !strings.HasPrefix(path, "/")) {
			add(Issue{Request: n, Field: "path", Code: "invalid_path", Message: "request path must begin with '/' or equal '*'"})
		}
		if req.Status < 100 || req.Status > 599 {
			add(Issue{Request: n, Field: "status", Code: "invalid_status", Message: "HTTP status must be between 100 and 599"})
		}
		if req.Latency < 0 {
			add(Issue{Request: n, Field: "latency", Code: "negative_latency", Message: "request latency must not be negative"})
		}
		if req.TargetPort < 1 || req.TargetPort > 65535 {
			add(Issue{Request: n, Field: "targetPort", Code: "invalid_target_port", Message: "target port must be between 1 and 65535"})
		}
	}
	return r
}
