package anomaly

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yatinannam/devpulse/internal/baseline"
)

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type Finding struct {
	Severity Severity `json:"severity"`
	Kind     string   `json:"kind"`
	Subject  string   `json:"subject"`
	Message  string   `json:"message"`
}

func Compare(base, current baseline.Snapshot) []Finding {
	findings := make([]Finding, 0)
	baseServices := map[int]baseline.Service{}
	currentServices := map[int]baseline.Service{}
	for _, s := range base.Services {
		baseServices[s.Port] = s
	}
	for _, s := range current.Services {
		currentServices[s.Port] = s
	}
	for port, s := range currentServices {
		if _, ok := baseServices[port]; !ok {
			findings = append(findings, Finding{Severity: SeverityInfo, Kind: "service-added", Subject: fmt.Sprintf(":%d", port), Message: fmt.Sprintf("%s started listening", describe(s))})
		}
	}
	for port, s := range baseServices {
		if _, ok := currentServices[port]; !ok {
			findings = append(findings, Finding{Severity: SeverityWarning, Kind: "service-removed", Subject: fmt.Sprintf(":%d", port), Message: fmt.Sprintf("%s is no longer listening", describe(s))})
		}
	}
	for port, s := range currentServices {
		if old, ok := baseServices[port]; ok && serviceChanged(old, s) {
			findings = append(findings, Finding{Severity: SeverityInfo, Kind: "service-changed", Subject: fmt.Sprintf(":%d", port), Message: fmt.Sprintf("service changed from %s to %s", describe(old), describe(s))})
		}
	}

	// A baseline captured before any traffic counts as a 0% error rate, so
	// the rule still fires on a failing current run.
	baseErrorRate := 0.0
	if base.Requests.Total > 0 {
		baseErrorRate = float64(base.Requests.Errors) / float64(base.Requests.Total)
	}
	currentErrorRate := float64(current.Requests.Errors) / float64(max(1, current.Requests.Total))
	if currentErrorRate >= 0.10 && currentErrorRate >= baseErrorRate*2 && current.Requests.Errors >= 3 {
		findings = append(findings, Finding{Severity: SeverityError, Kind: "error-rate", Subject: "traffic", Message: fmt.Sprintf("error rate increased from %.1f%% to %.1f%%", baseErrorRate*100, currentErrorRate*100)})
	}

	baseEndpoints := endpointMap(base.Requests.Endpoints)
	for _, current := range current.Requests.Endpoints {
		previous, ok := baseEndpoints[current.Method+" "+current.Path]
		if !ok || previous.Average <= 0 {
			continue
		}
		if current.Average >= previous.Average*2 && current.Count >= 3 {
			findings = append(findings, Finding{Severity: SeverityWarning, Kind: "latency-regression", Subject: current.Method + " " + current.Path, Message: fmt.Sprintf("average latency increased from %s to %s", formatLatency(time.Duration(previous.Average)), formatLatency(time.Duration(current.Average)))})
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return severityRank(findings[i].Severity) > severityRank(findings[j].Severity)
		}
		if findings[i].Subject != findings[j].Subject {
			return subjectLess(findings[i].Subject, findings[j].Subject)
		}
		return findings[i].Kind < findings[j].Kind
	})
	return findings
}

// formatLatency rounds d to about three significant digits, so 0.9ms prints
// as 900µs instead of truncating to 0ms.
func formatLatency(d time.Duration) string {
	switch {
	case d >= 100*time.Millisecond:
		return d.Round(time.Millisecond).String()
	case d >= 10*time.Millisecond:
		return d.Round(100 * time.Microsecond).String()
	case d >= time.Millisecond:
		return d.Round(10 * time.Microsecond).String()
	default:
		return d.Round(time.Microsecond).String()
	}
}

// serviceChanged reports whether a different program now listens on a port,
// using the same rule as diff's service-changed. Process lookup can fail
// without privileges, so process names are compared only when both
// snapshots recorded one.
func serviceChanged(old, current baseline.Service) bool {
	if old.Kind != current.Kind {
		return true
	}
	return old.Process != "" && current.Process != "" && old.Process != current.Process
}

// describe names a service by kind and, when known, its process, matching
// the wording of diff's service-changed message.
func describe(s baseline.Service) string {
	switch {
	case s.Kind != "" && s.Process != "":
		return fmt.Sprintf("%s (%s)", s.Kind, s.Process)
	case s.Kind != "":
		return s.Kind
	case s.Process != "":
		return s.Process
	default:
		return "TCP service"
	}
}

// subjectLess orders port subjects such as ":3000" numerically, so :3000
// sorts before :10000, and every other subject lexicographically.
func subjectLess(a, b string) bool {
	pa, aok := portOf(a)
	pb, bok := portOf(b)
	if aok && bok {
		return pa < pb
	}
	return a < b
}

func portOf(subject string) (int, bool) {
	if !strings.HasPrefix(subject, ":") {
		return 0, false
	}
	port, err := strconv.Atoi(subject[1:])
	return port, err == nil
}

func endpointMap(endpoints []baseline.Endpoint) map[string]baseline.Endpoint {
	out := make(map[string]baseline.Endpoint, len(endpoints))
	for _, e := range endpoints {
		out[e.Method+" "+e.Path] = e
	}
	return out
}

func severityRank(s Severity) int {
	switch s {
	case SeverityError:
		return 3
	case SeverityWarning:
		return 2
	default:
		return 1
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
