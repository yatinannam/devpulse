package status

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/yatinannam/devpulse/internal/discovery"
	"github.com/yatinannam/devpulse/internal/metrics"
	"github.com/yatinannam/devpulse/internal/traffic"
	"github.com/yatinannam/devpulse/internal/ui"
)

type Summary struct {
	Total, Errors, Slow int
	Average             time.Duration
	P50                 time.Duration
	P95                 time.Duration
	P99                 time.Duration
	Findings            int
}

func Build(entries []traffic.Request) Summary {
	var s Summary
	s.Total = len(entries)
	if len(entries) == 0 {
		return s
	}
	var total time.Duration
	latencies := make([]time.Duration, 0, len(entries))
	for _, e := range entries {
		total += e.Latency
		latencies = append(latencies, e.Latency)
		if e.Status >= 400 {
			s.Errors++
		}
		if e.Latency >= 500*time.Millisecond {
			s.Slow++
		}
	}
	s.Average = total / time.Duration(len(entries))
	s.P50, s.P95, s.P99 = metrics.Percentiles(latencies)
	return s
}

type Endpoint struct {
	Method, Path  string
	Count, Errors int
	Average       time.Duration
}

func NormalizeEndpointPath(raw string) string {
	path := raw
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	if path == "" {
		return "/"
	}
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "" {
			continue
		}
		if isUUID(part) || isNumeric(part) {
			parts[i] = ":id"
		}
	}
	return strings.Join(parts, "/")
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
			continue
		}
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func Endpoints(entries []traffic.Request) []Endpoint {
	m := map[string]*Endpoint{}
	for _, e := range entries {
		path := NormalizeEndpointPath(e.Path)
		key := e.Method + " " + path
		x := m[key]
		if x == nil {
			x = &Endpoint{Method: e.Method, Path: path}
			m[key] = x
		}
		x.Count++
		if e.Status >= 400 {
			x.Errors++
		}
		x.Average += e.Latency
	}
	out := make([]Endpoint, 0, len(m))
	for _, x := range m {
		x.Average /= time.Duration(x.Count)
		out = append(out, *x)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Errors != out[j].Errors {
			return out[i].Errors > out[j].Errors
		}
		if out[i].Average != out[j].Average {
			return out[i].Average > out[j].Average
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		// Map iteration order is random; break ties so the order (and
		// baseline.json) is the same on every run.
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

type ServiceTraffic struct {
	Service             discovery.Service
	Requests            []Endpoint
	Total, Errors, Slow int
	Average             time.Duration
}

func GroupByService(services []discovery.Service, entries []traffic.Request) []ServiceTraffic {
	groups := map[int][]traffic.Request{}
	for _, e := range entries {
		groups[e.TargetPort] = append(groups[e.TargetPort], e)
	}
	out := make([]ServiceTraffic, 0, len(services))
	// Requests are keyed by port, so a port listed twice (several PIDs on
	// one socket) would count every request once per copy. Keep the first.
	seen := map[int]bool{}
	for _, s := range services {
		if seen[s.Port] {
			continue
		}
		seen[s.Port] = true
		es := groups[s.Port]
		summary := Build(es)
		out = append(out, ServiceTraffic{Service: s, Requests: Endpoints(es), Total: summary.Total, Errors: summary.Errors, Slow: summary.Slow, Average: summary.Average})
	}
	return out
}

func Health(g ServiceTraffic) string {
	if !g.Service.HTTP {
		return "running"
	}
	if g.Errors > 0 {
		return "error"
	}
	if g.Slow > 0 {
		return "warning"
	}
	return "healthy"
}

// PrintServices writes the discovered services sorted by port: each HTTP
// service with its health, traffic and busiest endpoints, then every other
// TCP listener in a compact grid. No line is wider than width.
func PrintServices(w io.Writer, groups []ServiceTraffic, width int) {
	// Endpoints arrive worst first (errors, then latency), so the cap keeps
	// the rows that explain a service's health.
	const endpointLimit = 5
	sorted := append([]ServiceTraffic(nil), groups...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Service.Port < sorted[j].Service.Port })

	var httpGroups []ServiceTraffic
	var others []ui.Line
	portWidth, kindWidth, ownerWidth, methodWidth := 0, 0, 0, 0
	for _, g := range sorted {
		port := fmt.Sprintf(":%d", g.Service.Port)
		if !g.Service.HTTP {
			item := ui.Plain(port)
			if g.Service.Process != "" {
				item = ui.Join(item, ui.Styled(" "+g.Service.Process, ui.Muted))
			}
			others = append(others, item)
			continue
		}
		if g.Service.Kind == "" {
			g.Service.Kind = "HTTP service"
		}
		httpGroups = append(httpGroups, g)
		portWidth = max(portWidth, len(port))
		kindWidth = max(kindWidth, len([]rune(g.Service.Kind)))
		ownerWidth = max(ownerWidth, len([]rune(strings.TrimSpace(g.Service.Process+" "+g.Service.PID))))
		for i, e := range g.Requests {
			if i < endpointLimit {
				methodWidth = max(methodWidth, len(e.Method))
			}
		}
	}

	ui.Heading(w, "HTTP services", "")
	if len(httpGroups) == 0 {
		ui.Wrapped(w, width, 2, ui.Plain("No HTTP services found. Start your app, then run devpulse status again."))
	} else {
		// The first column holds service names and endpoint paths. Paths
		// are capped near the service rows' width, so one long path does
		// not push every number to the far edge of a wide terminal.
		serviceWidth := 1 + 1 + portWidth + 2 + kindWidth + 2 + ownerWidth
		table := ui.NewTable(
			ui.Column{Flex: true, Max: max(serviceWidth, 40)},
			ui.Column{Align: ui.AlignRight},
			ui.Column{Align: ui.AlignRight},
			ui.Column{Align: ui.AlignRight},
			ui.Column{},
		)
		table.Indent = 2
		// Endpoint rows pad their numbers by the width of " req", " err"
		// and " avg", so they line up under the service row's numbers.
		const unit = "    "
		for _, g := range httpGroups {
			owner := strings.TrimSpace(g.Service.Process + " " + g.Service.PID)
			left := ui.Join(healthMarker(Health(g)),
				ui.Plain(fmt.Sprintf(" %-*s  %-*s  ", portWidth, fmt.Sprintf(":%d", g.Service.Port), kindWidth, g.Service.Kind)),
				ui.Styled(owner, ui.Muted))
			if g.Total == 0 {
				table.Row(left, ui.Styled("0 req", ui.Muted))
				continue
			}
			errors := ui.Styled("0 err", ui.Muted)
			if g.Errors > 0 {
				errors = ui.Styled(fmt.Sprintf("%d err", g.Errors), ui.Error)
			}
			var slow ui.Line
			if g.Slow > 0 {
				slow = ui.Styled(fmt.Sprintf("%d slow", g.Slow), ui.Warn)
			}
			table.Row(left, ui.Plain(fmt.Sprintf("%d req", g.Total)), errors, ui.Join(ui.Latency(g.Average), ui.Plain(" avg")), slow)

			for i, e := range g.Requests {
				if i == endpointLimit {
					table.Row(ui.Join(ui.Plain("    "), ui.Styled(fmt.Sprintf("+ %d more endpoints", len(g.Requests)-endpointLimit), ui.Muted)))
					break
				}
				path := ui.Join(ui.Plain("    "), ui.Styled(fmt.Sprintf("%-*s", methodWidth, e.Method), ui.Muted), ui.Plain(" "+e.Path))
				errors := ui.Styled("0", ui.Muted)
				if e.Errors > 0 {
					errors = ui.Styled(fmt.Sprint(e.Errors), ui.Error)
				}
				var tag ui.Line
				if e.Average >= ui.Slow {
					tag = ui.Styled("slow", ui.LatencyStyle(e.Average))
				}
				table.Row(path, ui.Plain(fmt.Sprintf("%d%s", e.Count, unit)), ui.Join(errors, ui.Plain(unit)), ui.Join(ui.Latency(e.Average), ui.Plain(unit)), tag)
			}
		}
		table.Render(w, width)
	}

	if len(others) > 0 {
		fmt.Fprintln(w)
		ui.Heading(w, "Other listeners", fmt.Sprintf("(%d)", len(others)))
		ui.Grid(w, width, 2, others)
	}
}

// healthMarker returns the glyph for a Health value: ● healthy, ▲ warning,
// ✖ error, ○ a running non-HTTP listener.
func healthMarker(h string) ui.Line {
	switch h {
	case "healthy":
		return ui.LevelOK.Glyph()
	case "warning":
		return ui.LevelWarn.Glyph()
	case "error":
		return ui.LevelError.Glyph()
	default:
		return ui.LevelIdle.Glyph()
	}
}
