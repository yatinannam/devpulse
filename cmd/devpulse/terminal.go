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
	"github.com/yatinannam/devpulse/internal/diff"
	"github.com/yatinannam/devpulse/internal/discovery"
	"github.com/yatinannam/devpulse/internal/project"
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

// requestTable lists requests in the given order as Time, Method, Path,
// Status and Latency. The path absorbs the remaining width and truncates;
// status and latency are colored.
func requestTable(requests []traffic.Request, indent int) *ui.Table {
	table := ui.NewTable(
		ui.Column{Header: "Time"},
		ui.Column{Header: "Method"},
		ui.Column{Header: "Path", Flex: true},
		ui.Column{Header: "Status", Align: ui.AlignRight},
		ui.Column{Header: "Latency", Align: ui.AlignRight},
	)
	table.Indent = indent
	for _, r := range requests {
		stamp := ui.Styled("--:--:--", ui.Muted)
		if !r.Time.IsZero() {
			stamp = ui.Plain(r.Time.Local().Format("15:04:05"))
		}
		table.Row(stamp, ui.Plain(r.Method), ui.Plain(r.Path), ui.Status(r.Status), ui.Latency(r.Latency))
	}
	return table
}

// renderRecent writes the recent command's text view. recent is in capture
// order; it is listed newest first.
func renderRecent(w io.Writer, width int, path string, total int, recent []traffic.Request) {
	ui.Title(w, width, "recent", sessionContext(path, total))
	if len(recent) == 0 {
		writeNoTraffic(w, width)
		return
	}
	newestFirst := make([]traffic.Request, len(recent))
	for i, r := range recent {
		newestFirst[len(recent)-1-i] = r
	}
	requestTable(newestFirst, 0).Render(w, width)
	fmt.Fprintln(w)
	ui.Writeln(w, ui.Styled(fmt.Sprintf("Showing %d of %s", len(recent), ui.Count(total, "request", "requests")), ui.Muted))
}

// humanizeKind turns a stable kebab-case kind into a sentence-case label:
// "latency-regression" becomes "Latency regression".
func humanizeKind(kind string) string {
	label := strings.ReplaceAll(kind, "-", " ")
	if label == "" {
		return ""
	}
	return strings.ToUpper(label[:1]) + label[1:]
}

// diffLevel picks the glyph for a baseline change: something new is ●,
// something gone or replaced is ▲, and changed numbers are ○.
func diffLevel(kind diff.Kind) ui.Level {
	switch kind {
	case diff.ServiceAdded, diff.EndpointAdded:
		return ui.LevelOK
	case diff.ServiceRemoved, diff.ServiceChanged, diff.EndpointRemoved:
		return ui.LevelWarn
	default:
		return ui.LevelIdle
	}
}

// changeRow is one line of a diff or anomaly list.
type changeRow struct {
	level                  ui.Level
	kind, subject, message string
}

// renderChanges writes one line per change: glyph and kind, subject, then
// the message, which wraps rather than losing words.
func renderChanges(w io.Writer, width int, rows []changeRow) {
	table := ui.NewTable(
		ui.Column{},
		ui.Column{Flex: true, Max: 28, Min: 8},
		ui.Column{Flex: true, Wrap: true, Min: 20},
	)
	for _, r := range rows {
		table.Row(ui.Join(r.level.Glyph(), ui.Plain(" "+humanizeKind(r.kind))), ui.Plain(r.subject), ui.Plain(r.message))
	}
	table.Render(w, width)
}

// countsLine joins non-zero severity counts: "1 error, 2 warnings, 1 info".
func countsLine(errors, warnings, infos int) ui.Line {
	var parts []ui.Line
	if errors > 0 {
		parts = append(parts, ui.Styled(ui.Count(errors, "error", "errors"), ui.Error))
	}
	if warnings > 0 {
		parts = append(parts, ui.Styled(ui.Count(warnings, "warning", "warnings"), ui.Warn))
	}
	if infos > 0 {
		parts = append(parts, ui.Styled(ui.Count(infos, "info", "info"), ui.Muted))
	}
	var out ui.Line
	for i, p := range parts {
		if i > 0 {
			out = ui.Join(out, ui.Plain(", "))
		}
		out = ui.Join(out, p)
	}
	return out
}

// comparisonContext is the title context for commands that compare a
// session with a baseline.
func comparisonContext(sessionPath, baselinePath string) ui.Line {
	return ui.Plain(filepath.Base(sessionPath) + " vs " + filepath.Base(baselinePath))
}

// renderDiff writes the diff command's text view.
func renderDiff(w io.Writer, width int, sessionPath, baselinePath string, changes []diff.Change) {
	ui.Title(w, width, "diff", comparisonContext(sessionPath, baselinePath))
	if len(changes) == 0 {
		ui.Marked(w, width, ui.LevelOK, ui.Plain("No changes since the baseline."))
		return
	}
	rows := make([]changeRow, len(changes))
	for i, c := range changes {
		rows[i] = changeRow{diffLevel(c.Kind), string(c.Kind), c.Subject, c.Message}
	}
	renderChanges(w, width, rows)
	fmt.Fprintln(w)
	ui.Writeln(w, ui.Styled(ui.Count(len(changes), "change", "changes"), ui.Muted))
}

// helpGroups lists every command in the README's groups.
var helpGroups = []struct {
	title    string
	commands [][2]string
}{
	{"Discovery and capture", [][2]string{
		{"ports", "List local listening ports and processes"},
		{"traffic", "Capture HTTP traffic through the proxy"},
		{"status", "Show services and captured traffic"},
		{"watch", "Continuously refresh service and traffic health"},
	}},
	{"Analysis", [][2]string{
		{"recent", "Show the most recent captured requests"},
		{"doctor", "Analyze a captured session"},
		{"summary", "Summarize latency, errors, and slowest requests"},
		{"timeline", "Group captured traffic into chronological buckets"},
		{"validate", "Validate a captured traffic session"},
	}},
	{"Baselines and change detection", [][2]string{
		{"baseline", "Capture the current environment as a baseline"},
		{"diff", "Compare the current environment with the baseline"},
		{"anomalies", "Detect regressions against a saved baseline"},
	}},
	{"Reporting and export", [][2]string{
		{"report", "Generate a Markdown or JSON traffic report"},
		{"export", "Export captured requests as filtered CSV"},
	}},
	{"Setup", [][2]string{
		{"init", "Detect the current project and configure DevPulse"},
		{"config", "View or change persistent defaults"},
		{"version", "Print the current build version"},
	}},
}

// renderHelp writes the command overview, grouped as the README groups it.
func renderHelp(w io.Writer, width int) {
	ui.Title(w, width, "", ui.Plain("local development observability"))
	ui.KVList(w, width, 0, []ui.KV{{Key: "Usage", Value: ui.Plain("devpulse <command> [flags]")}})
	nameWidth := 0
	for _, g := range helpGroups {
		for _, c := range g.commands {
			nameWidth = max(nameWidth, len(c[0]))
		}
	}
	for _, g := range helpGroups {
		fmt.Fprintln(w)
		ui.Heading(w, g.title, "")
		table := ui.NewTable(ui.Column{Min: nameWidth}, ui.Column{Flex: true, Wrap: true, Min: 20})
		table.Indent = 2
		for _, c := range g.commands {
			table.Row(ui.Plain(fmt.Sprintf("%-*s", nameWidth, c[0])), ui.Plain(c[1]))
		}
		table.Render(w, width)
	}
	fmt.Fprintln(w)
	ui.Writeln(w, ui.Join(ui.Styled("Run ", ui.Muted), ui.Plain("devpulse <command> --help"), ui.Styled(" for command-specific flags.", ui.Muted)))
}

// renderInit writes the result of init. An empty target means no local
// HTTP service matched the project.
func renderInit(w io.Writer, width int, info project.Info, target, savedPath string) {
	ui.Title(w, width, "init", nil)
	label := info.Name
	if info.Kind != "" {
		label += " (" + info.Kind + ")"
	}
	if target == "" {
		ui.Marked(w, width, ui.LevelWarn, ui.Plain("No local HTTP service matched "+label+"."))
		ui.Wrapped(w, width, 2, ui.Plain("Start the application, then run devpulse init again."))
		return
	}
	ui.Marked(w, width, ui.LevelOK, ui.Plain("Configured "+label+" with target "+target))
	const saved = "  Saved to "
	ui.Writeln(w, ui.Styled(saved+ui.TruncatePath(savedPath, width-len(saved)), ui.Muted))
	fmt.Fprintln(w)
	ui.Writeln(w, ui.Join(ui.Styled("Next: ", ui.Muted), ui.Plain("devpulse traffic")))
}

// renderConfig writes the active configuration as aligned key-value pairs
// and where it comes from. exists reports whether the source file is on
// disk; saved, whether this run just wrote it.
func renderConfig(w io.Writer, width int, c config.Config, source string, exists, saved bool) {
	var context ui.Line
	if saved {
		context = ui.Plain("saved")
	}
	ui.Title(w, width, "config", context)
	// Values start after the widest key, "Watch interval", and two spaces.
	room := width - len("Watch interval") - 2
	note := ""
	if !exists {
		note = " (not created yet)"
	}
	sourceValue := ui.Join(ui.Plain(ui.TruncatePath(source, room-len(note))), ui.Styled(note, ui.Muted))
	ui.KVList(w, width, 0, []ui.KV{
		{Key: "Listen", Value: ui.Plain(c.Listen)},
		{Key: "Target", Value: ui.Plain(c.Target)},
		{Key: "Watch interval", Value: ui.Plain(c.WatchInterval)},
		{Key: "Source", Value: sourceValue},
	})
}

// renderTrafficStart writes the traffic command's header: where the proxy
// listens and forwards, and where the session will be saved.
func renderTrafficStart(w io.Writer, width int, listen, target, session string) {
	ui.Title(w, width, "traffic", nil)
	room := width - len("Session") - 2
	ui.KVList(w, width, 0, []ui.KV{
		{Key: "Proxy", Value: ui.Plain(proxyURLFor(listen) + " → " + target)},
		{Key: "Session", Value: ui.Plain(ui.TruncatePath(session, room))},
	})
	fmt.Fprintln(w)
	ui.Wrapped(w, width, 0, ui.Styled("Waiting for HTTP traffic. Press Ctrl+C to save and stop.", ui.Muted))
	fmt.Fprintln(w)
}

// trafficLine is one live request while traffic records: time, method,
// path, colored status and latency, and the upstream it went to. Lines are
// printed as requests arrive, so columns have fixed widths; the path takes
// what is left, up to 40 columns, and the upstream is dropped first when
// the terminal is narrow.
func trafficLine(width int, r traffic.Request) ui.Line {
	head := ui.Join(ui.Styled(r.Time.Local().Format("15:04:05"), ui.Muted), ui.Plain(fmt.Sprintf("  %-7s  ", r.Method)))
	status, latency := ui.Status(r.Status), ui.Latency(r.Latency)
	numbers := ui.Join(
		ui.Plain(strings.Repeat(" ", 2+max(0, 3-status.Width()))), status,
		ui.Plain(strings.Repeat(" ", 2+max(0, 6-latency.Width()))), latency,
	)
	upstream := ui.Styled(fmt.Sprintf("  → %s:%d", r.TargetHost, r.TargetPort), ui.Muted)
	room := width - head.Width() - numbers.Width() - upstream.Width()
	if room < 12 {
		upstream = nil
		room = width - head.Width() - numbers.Width()
	}
	room = min(40, room)
	path := ui.Plain(r.Path).Truncate(room)
	path = ui.Join(path, ui.Plain(strings.Repeat(" ", room-path.Width())))
	return ui.Join(head, path, numbers, upstream)
}

// renderTrafficSaved reports the saved session and any requests the
// recorder dropped to stay within --max-requests.
func renderTrafficSaved(w io.Writer, width, saved int, path string, dropped int) {
	ui.Marked(w, width, ui.LevelOK, ui.Plain(fmt.Sprintf("Saved %s to %s", ui.Count(saved, "request", "requests"), path)))
	if dropped > 0 {
		ui.Marked(w, width, ui.LevelWarn, ui.Plain(fmt.Sprintf("%s dropped; raise --max-requests to keep more.", ui.Count(dropped, "older request was", "older requests were"))))
	}
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
