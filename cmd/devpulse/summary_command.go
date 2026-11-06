package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/yatinannam/devpulse/internal/output"
	"github.com/yatinannam/devpulse/internal/summary"
	"github.com/yatinannam/devpulse/internal/traffic"
	"github.com/yatinannam/devpulse/internal/ui"
)

type summaryPayload struct {
	Session string            `json:"session"`
	Summary summary.Report    `json:"summary"`
	Slowest []traffic.Request `json:"slowest"`
}

func summaryCommand(args []string) {
	fs := flag.NewFlagSet("summary", flag.ExitOnError)
	from := fs.String("from", sessionPath(), "traffic session to summarize")
	slowestCount := fs.Int("slowest", 5, "number of slowest requests to include (0 disables)")
	jsonOutput := fs.Bool("json", false, "print summary as JSON")
	mustParseArgs(fs, args)

	if *slowestCount < 0 {
		fmt.Fprintln(os.Stderr, "devpulse: --slowest must be zero or greater")
		os.Exit(2)
	}

	s, err := traffic.LoadSession(*from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	report := summary.Build(s.Requests)
	slowest := summary.Slowest(s.Requests, *slowestCount)

	if *jsonOutput {
		if err := output.WriteJSON(os.Stdout, summaryPayload{
			Session: *from,
			Summary: report,
			Slowest: slowest,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
			os.Exit(1)
		}
		return
	}

	renderSummary(os.Stdout, ui.TermWidth(), *from, s.Requests, report, slowest)
}

// renderSummary writes aligned metrics, the latency pulse in request order,
// then the slowest requests.
func renderSummary(w io.Writer, width int, path string, requests []traffic.Request, report summary.Report, slowest []traffic.Request) {
	ui.Title(w, width, "summary", sessionContext(path, report.Total))
	if report.Total == 0 {
		writeNoTraffic(w, width)
		return
	}
	errorStyle := ui.Style(ui.Normal)
	if report.Errors > 0 {
		errorStyle = ui.Error
	}
	const caption = "  latency in request order"
	const keyWidth = len("Requests")
	latencies := make([]float64, len(requests))
	for i, r := range requests {
		latencies[i] = float64(r.Latency)
	}
	pulseWidth := min(60, width-keyWidth-2-len(caption))
	ui.KVList(w, width, 0, []ui.KV{
		{Key: "Requests", Value: ui.Styled(strconv.Itoa(report.Total), ui.Bold)},
		{Key: "Errors", Value: ui.Styled(fmt.Sprintf("%d (%.1f%%)", report.Errors, report.ErrorRate*100), errorStyle)},
		{Key: "Average", Value: ui.Latency(report.Average)},
		{Key: "p50", Value: ui.Latency(report.P50)},
		{Key: "p95", Value: ui.Latency(report.P95)},
		{Key: "p99", Value: ui.Latency(report.P99)},
		{Key: "Pulse", Value: ui.Join(ui.Styled(ui.Sparkline(latencies, pulseWidth), ui.Accent), ui.Styled(caption, ui.Muted))},
	})
	if len(slowest) == 0 {
		return
	}
	fmt.Fprintln(w)
	ui.Heading(w, "Slowest requests", "")
	requestTable(slowest, 2).Render(w, width)
}
