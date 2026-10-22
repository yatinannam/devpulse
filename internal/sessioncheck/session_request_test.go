package sessioncheck

import (
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

func validRequest() traffic.Request {
	return traffic.Request{
		Time: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Method: "GET", Path: "/health", Status: 200,
		Latency: 12 * time.Millisecond, Bytes: 24,
		TargetHost: "localhost", TargetPort: 8080,
	}
}

func validSession(requests ...traffic.Request) traffic.Session {
	return traffic.Session{Version: 1, CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Requests: requests}
}

func TestValidateRequestShape(t *testing.T) {
	got := Validate(validSession(validRequest()))
	if !got.Valid || got.RequestCount != 1 || len(got.Issues) != 0 {
		t.Fatalf("unexpected report: %+v", got)
	}
	req := validRequest()
	req.Time = time.Time{}
	req.Method = " "
	req.Path = "api/users"
	got = Validate(validSession(req))
	codes := map[string]bool{}
	for _, issue := range got.Issues { codes[issue.Code] = true }
	for _, code := range []string{"missing_timestamp", "missing_method", "invalid_path"} {
		if !codes[code] { t.Errorf("missing issue %q in %+v", code, got.Issues) }
	}
}

func TestValidateAllowsAsteriskRequestTarget(t *testing.T) {
	req := validRequest()
	req.Method = "OPTIONS"
	req.Path = "*"
	if got := Validate(validSession(req)); !got.Valid {
		t.Fatalf("asterisk-form request should be valid: %+v", got.Issues)
	}
}
