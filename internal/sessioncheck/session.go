package sessioncheck

import "github.com/yatinannam/devpulse/internal/traffic"

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

func Validate(s traffic.Session) Report {
	r := Report{Valid: true, RequestCount: len(s.Requests), Issues: []Issue{}}
	if s.Version != 1 {
		r.Valid = false
		r.Issues = append(r.Issues, Issue{Field: "version", Code: "unsupported_version", Message: "session version must be 1"})
	}
	return r
}
