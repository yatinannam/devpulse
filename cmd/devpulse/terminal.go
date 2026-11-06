package main

// Helpers shared by the commands' text (non --json) output. Layout
// primitives live in internal/ui; this file holds the DevPulse-specific
// pieces several commands reuse.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yatinannam/devpulse/internal/config"
	"github.com/yatinannam/devpulse/internal/discovery"
	"github.com/yatinannam/devpulse/internal/status"
	"github.com/yatinannam/devpulse/internal/traffic"
	"github.com/yatinannam/devpulse/internal/ui"
)

// sessionContext is the title context for a command that reads a session,
// such as "demo.json, 12 requests". Extra parts follow, comma-separated.
func sessionContext(path string, requests int, extra ...string) ui.Line {
	parts := append([]string{filepath.Base(path), ui.Count(requests, "request", "requests")}, extra...)
	return ui.Plain(strings.Join(parts, ", "))
}

// proxyURL is where the configured traffic proxy accepts requests.
func proxyURL() string {
	listen := config.Default().Listen
	if c, err := config.Load(); err == nil && c.Listen != "" {
		listen = c.Listen
	}
	return proxyURLFor(listen)
}

// proxyURLFor turns a listen address into a URL a browser or curl can use:
// an empty or wildcard host becomes 127.0.0.1.
func proxyURLFor(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "http://" + listen
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

// writeNoTraffic is the empty state for commands that need captured
// requests: it says how to capture some.
func writeNoTraffic(w io.Writer, width int) {
	ui.Wrapped(w, width, 0, ui.Plain("No captured traffic yet. Run devpulse traffic, then send requests to "+proxyURL()+"."))
}

// latencyFact is a dim label followed by a colored latency: "p95 1.3s".
func latencyFact(label string, d time.Duration) ui.Line {
	return ui.Join(ui.Styled(label+" ", ui.Muted), ui.Latency(d))
}

// trafficFacts summarizes captured traffic for the status and watch footer.
func trafficFacts(s status.Summary) []ui.Line {
	words := " requests"
	if s.Total == 1 {
		words = " request"
	}
	errorText := fmt.Sprintf("%s (%.1f%%)", ui.Count(s.Errors, "error", "errors"), 100*float64(s.Errors)/float64(max(1, s.Total)))
	errorStyle := ui.Style(ui.Muted)
	if s.Errors > 0 {
		errorStyle = ui.Error
	}
	facts := []ui.Line{
		ui.Join(ui.Styled(strconv.Itoa(s.Total), ui.Bold), ui.Plain(words)),
		ui.Styled(errorText, errorStyle),
		latencyFact("avg", s.Average),
		latencyFact("p50", s.P50),
		latencyFact("p95", s.P95),
		latencyFact("p99", s.P99),
	}
	if s.Slow > 0 {
		facts = append(facts, ui.Styled(fmt.Sprintf("%d slow", s.Slow), ui.Warn))
	}
	return facts
}

// renderStatusBody writes what status and watch share: the services, then
// the captured traffic or what to do when there is none.
func renderStatusBody(w io.Writer, width int, services []discovery.Service, session traffic.Session, loadErr error, path string) {
	var requests []traffic.Request
	if loadErr == nil {
		requests = session.Requests
	}
	status.PrintServices(w, status.GroupByService(services, requests), width)
	fmt.Fprintln(w)
	switch {
	case loadErr != nil && !errors.Is(loadErr, fs.ErrNotExist):
		ui.Marked(w, width, ui.LevelWarn, ui.Plain(fmt.Sprintf("Could not read %s: %v", filepath.Base(path), loadErr)))
	case len(requests) == 0:
		writeNoTraffic(w, width)
	default:
		ui.Facts(w, width, "Traffic", trafficFacts(status.Build(requests)))
	}
}

// ratePulse remembers how many requests each watch refresh added, for the
// pulse sparkline in the watch title line.
type ratePulse struct {
	limit   int
	samples []float64
	total   int
	started bool
}

// observe records the session's request count at one refresh.
func (p *ratePulse) observe(total int) {
	if p.started {
		added := total - p.total
		if added < 0 {
			// The session shrank, so a new capture replaced it.
			added = total
		}
		p.samples = append(p.samples, float64(added))
		if len(p.samples) > p.limit {
			p.samples = p.samples[len(p.samples)-p.limit:]
		}
	}
	p.total, p.started = total, true
}

// context is the watch title context: the pulse, the latest rate and the
// refresh time.
func (p *ratePulse) context(now time.Time) ui.Line {
	updated := "updated " + now.Format("15:04:05")
	if len(p.samples) == 0 {
		return ui.Plain(updated)
	}
	latest := int(p.samples[len(p.samples)-1])
	return ui.Join(
		ui.Styled(ui.Sparkline(p.samples, p.limit), ui.Accent),
		ui.Plain(fmt.Sprintf("  %d new, %s", latest, updated)),
	)
}

// renderWatchFrame writes one watch refresh.
func renderWatchFrame(w io.Writer, width int, now time.Time, interval time.Duration, pulse *ratePulse, services []discovery.Service, session traffic.Session, loadErr error, path string) {
	ui.Title(w, width, "watch", pulse.context(now))
	renderStatusBody(w, width, services, session, loadErr, path)
	fmt.Fprintln(w)
	ui.Writeln(w, ui.Styled(fmt.Sprintf("Refreshing every %s. Press Ctrl+C to stop.", interval), ui.Muted))
}
