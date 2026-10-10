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
	if findings[0].Severity != SeverityError {
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

func TestCompareRequiresMaterialErrorRateRegression(t *testing.T) {
	base := baseline.Snapshot{Requests: baseline.Traffic{Total: 100, Errors: 2}}
	cases := []struct {
		name    string
		current baseline.Traffic
		want    bool
	}{
		{name: "below minimum error count", current: baseline.Traffic{Total: 100, Errors: 2}, want: false},
		{name: "below error rate threshold", current: baseline.Traffic{Total: 100, Errors: 9}, want: false},
		{name: "regression crosses thresholds", current: baseline.Traffic{Total: 100, Errors: 10}, want: true},
		{name: "no current traffic", current: baseline.Traffic{}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			current := baseline.Snapshot{Requests: tc.current}
			found := false
			for _, finding := range Compare(base, current) {
				if finding.Kind == "error-rate" {
					found = true
					break
				}
			}
			if found != tc.want {
				t.Fatalf("error-rate finding=%v, want %v", found, tc.want)
			}
		})
	}
}
