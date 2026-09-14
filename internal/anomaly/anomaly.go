package anomaly

import (
	"fmt"
	"sort"

	"github.com/yatinannam/devpulse/internal/baseline"
)

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type Finding struct {
	Severity string
	Kind     string
	Subject  string
	Message  string
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
			findings = append(findings, Finding{Severity: string(SeverityInfo), Kind: "service-added", Subject: fmt.Sprintf(":%d", port), Message: fmt.Sprintf("service %s started", s.Kind)})
		}
	}
	for port, s := range baseServices {
		if _, ok := currentServices[port]; !ok {
			findings = append(findings, Finding{Severity: string(SeverityWarning), Kind: "service-removed", Subject: fmt.Sprintf(":%d", port), Message: fmt.Sprintf("service %s is no longer listening", s.Kind)})
		}
	}

	if base.Requests.Total > 0 {
		baseErrorRate := float64(base.Requests.Errors) / float64(base.Requests.Total)
		currentErrorRate := float64(current.Requests.Errors) / float64(max(1, current.Requests.Total))
		if currentErrorRate >= 0.10 && currentErrorRate >= baseErrorRate*2 && current.Requests.Errors >= 3 {
			findings = append(findings, Finding{Severity: string(SeverityError), Kind: "error-rate", Subject: "traffic", Message: fmt.Sprintf("error rate increased from %.1f%% to %.1f%%", baseErrorRate*100, currentErrorRate*100)})
		}
	}

	baseEndpoints := endpointMap(base.Requests.Endpoints)
	for _, current := range current.Requests.Endpoints {
		previous, ok := baseEndpoints[current.Method+" "+current.Path]
		if !ok || previous.Average <= 0 {
			continue
		}
		if current.Average >= previous.Average*2 && current.Count >= 3 {
			findings = append(findings, Finding{Severity: string(SeverityWarning), Kind: "latency-regression", Subject: current.Method + " " + current.Path, Message: fmt.Sprintf("average latency increased from %dms to %dms", previous.Average/1e6, current.Average/1e6)})
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return severityRank(findings[i].Severity) > severityRank(findings[j].Severity)
		}
		return findings[i].Subject < findings[j].Subject
	})
	return findings
}

func endpointMap(endpoints []baseline.Endpoint) map[string]baseline.Endpoint {
	out := make(map[string]baseline.Endpoint, len(endpoints))
	for _, e := range endpoints {
		out[e.Method+" "+e.Path] = e
	}
	return out
}

func severityRank(s string) int {
	switch s {
	case string(SeverityError):
		return 3
	case string(SeverityWarning):
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
