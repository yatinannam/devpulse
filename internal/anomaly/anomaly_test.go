package anomaly

import (
	"reflect"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/baseline"
)

func TestCompareFindsServiceAndLatencyChanges(t *testing.T) {
	base := baseline.Snapshot{
		Version:  1,
		Services: []baseline.Service{{Port: 3000, Kind: "Node / Next.js", HTTP: true}},
		Requests: baseline.Traffic{
			Total:     100,
			Errors:    2,
			Endpoints: []baseline.Endpoint{{Method: "GET", Path: "/api/users", Count: 100, Errors: 2, Average: 100_000_000}},
		},
	}
	current := baseline.Snapshot{
		Version:  1,
		Services: []baseline.Service{{Port: 8080, Kind: "Go", HTTP: true}},
		Requests: baseline.Traffic{
			Total:     100,
			Errors:    20,
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

func findingsOfKind(findings []Finding, kind string) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}

func TestCompareTreatsEmptyBaselineAsZeroErrorRate(t *testing.T) {
	// A baseline captured before any traffic has no error rate to double,
	// so it must count as 0% instead of disabling the rule.
	base := baseline.Snapshot{Version: 1}
	cases := []struct {
		name    string
		current baseline.Traffic
		want    []Finding
	}{
		{name: "half of the requests fail", current: baseline.Traffic{Total: 100, Errors: 50}, want: []Finding{
			{Severity: SeverityError, Kind: "error-rate", Subject: "traffic", Message: "error rate increased from 0.0% to 50.0%"},
		}},
		{name: "below minimum error count", current: baseline.Traffic{Total: 2, Errors: 2}},
		{name: "below error rate threshold", current: baseline.Traffic{Total: 100, Errors: 9}},
		{name: "no current traffic", current: baseline.Traffic{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findingsOfKind(Compare(base, baseline.Snapshot{Version: 1, Requests: tc.current}), "error-rate")
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestCompareLatencyMessageKeepsSubMillisecondPrecision(t *testing.T) {
	cases := []struct {
		previous, current time.Duration
		want              string
	}{
		{900 * time.Microsecond, 2700 * time.Microsecond, "average latency increased from 900µs to 2.7ms"},
		{100 * time.Millisecond, 300 * time.Millisecond, "average latency increased from 100ms to 300ms"},
		{12340 * time.Microsecond, 1234567 * time.Microsecond, "average latency increased from 12.3ms to 1.235s"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			base := baseline.Snapshot{Requests: baseline.Traffic{Endpoints: []baseline.Endpoint{{Method: "GET", Path: "/", Count: 5, Average: int64(tc.previous)}}}}
			current := baseline.Snapshot{Requests: baseline.Traffic{Endpoints: []baseline.Endpoint{{Method: "GET", Path: "/", Count: 5, Average: int64(tc.current)}}}}
			got := findingsOfKind(Compare(base, current), "latency-regression")
			if len(got) != 1 || got[0].Message != tc.want {
				t.Fatalf("got %+v, want message %q", got, tc.want)
			}
		})
	}
}

func TestCompareSortsPortSubjectsNumerically(t *testing.T) {
	current := baseline.Snapshot{Services: []baseline.Service{{Port: 10000, Kind: "Go"}, {Port: 3000, Kind: "Go"}, {Port: 443, Kind: "Go"}}}
	for i := 0; i < 20; i++ {
		var got []string
		for _, f := range Compare(baseline.Snapshot{}, current) {
			got = append(got, f.Subject)
		}
		if want := []string{":443", ":3000", ":10000"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: got %q, want %q", i, got, want)
		}
	}
}

func TestCompareReportsServiceChangedAsInfo(t *testing.T) {
	base := baseline.Snapshot{Services: []baseline.Service{
		{Port: 3000, Kind: "Node / Next.js", Process: "node"},
		{Port: 4000, Kind: "Go", Process: "api"},
		{Port: 5000, Kind: "Go", Process: "api"},
	}}
	current := baseline.Snapshot{Services: []baseline.Service{
		{Port: 3000, Kind: "Go", Process: "api"},
		{Port: 4000, Kind: "Go", Process: "worker"},
		{Port: 5000, Kind: "Go"}, // process unknown: not a change
	}}
	want := []Finding{
		{Severity: SeverityInfo, Kind: "service-changed", Subject: ":3000", Message: "service changed from Node / Next.js (node) to Go (api)"},
		{Severity: SeverityInfo, Kind: "service-changed", Subject: ":4000", Message: "service changed from Go (api) to Go (worker)"},
	}
	if got := Compare(base, current); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestCompareNamesUnidentifiedServicesInAddedAndRemovedMessages(t *testing.T) {
	// A non-HTTP listener has an empty Kind, which used to render as the
	// double-spaced "service  started".
	base := baseline.Snapshot{Version: baseline.Version, Services: []baseline.Service{{Port: 5432}}}
	current := baseline.Snapshot{Version: baseline.Version, Services: []baseline.Service{{Port: 6379, Process: "redis-server"}}}
	messages := map[string]string{}
	for _, f := range Compare(base, current) {
		messages[f.Kind] = f.Message
	}
	if got, want := messages["service-added"], "redis-server started listening"; got != want {
		t.Errorf("service-added message = %q, want %q", got, want)
	}
	if got, want := messages["service-removed"], "TCP service is no longer listening"; got != want {
		t.Errorf("service-removed message = %q, want %q", got, want)
	}
}
