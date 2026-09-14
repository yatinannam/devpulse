package diff

import (
    "fmt"
    "sort"
    "time"

    "github.com/yatinannam/devpulse/internal/baseline"
)

type Kind string

const (
    ServiceAdded   Kind = "service-added"
    ServiceRemoved Kind = "service-removed"
    TrafficChanged Kind = "traffic-changed"
    EndpointChanged Kind = "endpoint-changed"
)

type Change struct {
    Kind    Kind   `json:"kind"`
    Subject string `json:"subject"`
    Message string `json:"message"`
}

func Compare(base, current baseline.Snapshot) []Change {
    var out []Change
    baseServices := map[int]baseline.Service{}
    currentServices := map[int]baseline.Service{}
    for _, s := range base.Services { baseServices[s.Port] = s }
    for _, s := range current.Services { currentServices[s.Port] = s }
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
    if base.Requests.Total != current.Requests.Total || base.Requests.Errors != current.Requests.Errors || time.Duration(base.Requests.Average) != time.Duration(current.Requests.Average) {
        out = append(out, Change{TrafficChanged, "traffic", fmt.Sprintf("requests %d→%d, errors %d→%d, average %s→%s", base.Requests.Total, current.Requests.Total, base.Requests.Errors, current.Requests.Errors, time.Duration(base.Requests.Average).Round(time.Millisecond), time.Duration(current.Requests.Average).Round(time.Millisecond))})
    }
    baseEndpoints := map[string]baseline.Endpoint{}
    currentEndpoints := map[string]baseline.Endpoint{}
    for _, e := range base.Requests.Endpoints { baseEndpoints[e.Method+" "+e.Path] = e }
    for _, e := range current.Requests.Endpoints { currentEndpoints[e.Method+" "+e.Path] = e }
    for key, e := range currentEndpoints {
        old, ok := baseEndpoints[key]
        if !ok || old.Count != e.Count || old.Errors != e.Errors || old.Average != e.Average {
            out = append(out, Change{EndpointChanged, key, fmt.Sprintf("count %d→%d, errors %d→%d, average %s→%s", old.Count, e.Count, old.Errors, e.Errors, time.Duration(old.Average).Round(time.Millisecond), time.Duration(e.Average).Round(time.Millisecond))})
        }
    }
    sort.Slice(out, func(i, j int) bool {
        if out[i].Kind != out[j].Kind { return out[i].Kind < out[j].Kind }
        return out[i].Subject < out[j].Subject
    })
    return out
}

func label(s baseline.Service) string {
    if s.Kind != "" { return s.Kind }
    if s.Process != "" { return s.Process }
    return "TCP service"
}
