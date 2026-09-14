package anomaly

import (
	"testing"

	"github.com/yatinannam/devpulse/internal/baseline"
)

func TestCompareFindsServiceAndLatencyChanges(t *testing.T) {
	base := baseline.Snapshot{
		Version: 1,
		Services: []baseline.Service{{Port: 3000, Kind: "Node / Next.js", HTTP: true}},
		Requests: baseline.Traffic{
			Total: 100,
			Errors: 2,
			Endpoints: []baseline.Endpoint{{Method: "GET", Path: "/api/users", Count: 100, Errors: 2, Average: 100_000_000}},
		},
	}
	current := baseline.Snapshot{
		Version: 1,
		Services: []baseline.Service{{Port: 8080, Kind: "Go", HTTP: true}},
		Requests: baseline.Traffic{
			Total: 100,
			Errors: 20,
			Endpoints: []baseline.Endpoint{{Method: "GET", Path: "/api/users", Count: 100, Errors: 20, Average: 300_000_000}},
		},
	}

	findings := Compare(base, current)
	if len(findings) < 4 {
		t.Fatalf("expected service, error-rate, and latency findings; got %+v", findings)
	}
	if findings[0].Severity != string(SeverityError) {
		t.Fatalf("expected highest severity first, got %+v", findings[0])
	}
}

func TestCompareIgnoresSmallLatencyChanges(t *testing.T) {
	base := baseline.Snapshot{Version: 1, Requests: baseline.Traffic{Total: 100, Errors: 1, Endpoints: []baseline.Endpoint{{Method: "GET", Path: "/", Count: 100, Average: 100_000_000}}}}
	current := baseline.Snapshot{Version: 1, Requests: baseline.Traffic{Total: 100, Errors: 2, Endpoints: []baseline.Endpoint{{Method: "GET", Path: "/", Count: 100, Average: 150_000_000}}}}
	if got := Compare(base, current); len(got) != 0 {
		t.Fatalf("unexpected findings: %+v", got)
	}
}
