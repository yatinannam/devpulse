package diff

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yatinannam/devpulse/internal/baseline"
)

type Kind string

const (
	ServiceAdded    Kind = "service-added"
	ServiceRemoved  Kind = "service-removed"
	TrafficChanged  Kind = "traffic-changed"
	EndpointChanged Kind = "endpoint-changed"
	EndpointAdded   Kind = "endpoint-added"
	EndpointRemoved Kind = "endpoint-removed"
	ServiceChanged  Kind = "service-changed"
)

type Change struct {
	Kind    Kind   `json:"kind"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}

func Compare(base, current baseline.Snapshot) []Change {
	// Never nil: JSON consumers are promised `[]`, not null.
	out := []Change{}
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
			out = append(out, Change{ServiceAdded, fmt.Sprintf(":%d", port), fmt.Sprintf("service %s detected", label(s))})
		}
	}
	for port, s := range baseServices {
		if _, ok := currentServices[port]; !ok {
			out = append(out, Change{ServiceRemoved, fmt.Sprintf(":%d", port), fmt.Sprintf("service %s removed", label(s))})
		}
	}
	for port, s := range currentServices {
		if old, ok := baseServices[port]; ok && serviceChanged(old, s) {
			out = append(out, Change{ServiceChanged, fmt.Sprintf(":%d", port), fmt.Sprintf("service changed from %s to %s", describe(old), describe(s))})
		}
	}
	if base.Requests.Total != current.Requests.Total || base.Requests.Errors != current.Requests.Errors || roundAverage(base.Requests.Average) != roundAverage(current.Requests.Average) {
		out = append(out, Change{TrafficChanged, "traffic", fmt.Sprintf("requests %d→%d, errors %d→%d, average %s→%s", base.Requests.Total, current.Requests.Total, base.Requests.Errors, current.Requests.Errors, formatAverage(base.Requests.Average), formatAverage(current.Requests.Average))})
	}
	baseEndpoints := map[string]baseline.Endpoint{}
	currentEndpoints := map[string]baseline.Endpoint{}
	for _, e := range base.Requests.Endpoints {
		baseEndpoints[e.Method+" "+e.Path] = e
	}
	for _, e := range current.Requests.Endpoints {
		currentEndpoints[e.Method+" "+e.Path] = e
	}
	for key, e := range currentEndpoints {
		old, ok := baseEndpoints[key]
		if !ok {
			out = append(out, Change{EndpointAdded, key, fmt.Sprintf("new endpoint; count %d, errors %d, average %s", e.Count, e.Errors, formatAverage(e.Average))})
			continue
		}
		if old.Count != e.Count || old.Errors != e.Errors || roundAverage(old.Average) != roundAverage(e.Average) {
			out = append(out, Change{EndpointChanged, key, fmt.Sprintf("count %d→%d, errors %d→%d, average %s→%s", old.Count, e.Count, old.Errors, e.Errors, formatAverage(old.Average), formatAverage(e.Average))})
		}
	}
	for key, old := range baseEndpoints {
		if _, ok := currentEndpoints[key]; !ok {
			out = append(out, Change{EndpointRemoved, key, fmt.Sprintf("endpoint removed; last seen count %d, errors %d, average %s", old.Count, old.Errors, formatAverage(old.Average))})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return subjectLess(out[i].Subject, out[j].Subject)
	})
	return out
}

// roundAverage converts a nanosecond average to the millisecond resolution
// diff compares at, so 20.0001ms and 20.0002ms are not reported as a change.
func roundAverage(ns int64) time.Duration {
	return time.Duration(ns).Round(time.Millisecond)
}

// formatAverage prints an average at the resolution it is compared at.
func formatAverage(ns int64) string {
	d := roundAverage(ns)
	if d == 0 {
		return "0ms"
	}
	return d.String()
}

// serviceChanged reports whether a different program now listens on a port.
// Process lookup can fail without privileges, so process names are compared
// only when both snapshots recorded one.
func serviceChanged(old, current baseline.Service) bool {
	if old.Kind != current.Kind {
		return true
	}
	return old.Process != "" && current.Process != "" && old.Process != current.Process
}

// describe names a service by kind and, when known, its process.
func describe(s baseline.Service) string {
	if s.Kind != "" && s.Process != "" {
		return fmt.Sprintf("%s (%s)", s.Kind, s.Process)
	}
	return label(s)
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

func label(s baseline.Service) string {
	if s.Kind != "" {
		return s.Kind
	}
	if s.Process != "" {
		return s.Process
	}
	return "TCP service"
}
