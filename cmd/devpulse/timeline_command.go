package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/yatinannam/devpulse/internal/output"
	"github.com/yatinannam/devpulse/internal/timeline"
	"github.com/yatinannam/devpulse/internal/traffic"
	"github.com/yatinannam/devpulse/internal/ui"
)

type timelinePayload struct {
	Session string            `json:"session"`
	Bucket  time.Duration     `json:"bucket_width"`
	Buckets []timeline.Bucket `json:"buckets"`
}

func timelineCommand(args []string) {
	fs := flag.NewFlagSet("timeline", flag.ExitOnError)
	from := fs.String("from", sessionPath(), "traffic session to inspect")
	width := fs.Duration("bucket", time.Minute, "bucket width, for example 30s or 5m")
	jsonOutput := fs.Bool("json", false, "print timeline as JSON")
	mustParseArgs(fs, args)

	if *width <= 0 {
		fmt.Fprintln(os.Stderr, "devpulse: --bucket must be greater than zero")
		os.Exit(2)
	}
	s, err := traffic.LoadSession(*from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
		os.Exit(1)
	}
	buckets := timeline.Build(s.Requests, *width)

	if *jsonOutput {
		if err := output.WriteJSON(os.Stdout, timelinePayload{
			Session: *from,
			Bucket:  *width,
			Buckets: buckets,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "devpulse: %v\n", err)
			os.Exit(1)
		}
		return
	}

	renderTimeline(os.Stdout, ui.TermWidth(), *from, len(s.Requests), *width, buckets)
}

// bucketLabel formats a bucket width without zero units: "1m", not "1m0s".
func bucketLabel(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

// renderTimeline writes one row per bucket with a pulse bar scaled to the
// busiest bucket; the bar's error share is drawn in red. Starts are local
// time, with the date only when the buckets span more than one day.
func renderTimeline(w io.Writer, width int, path string, total int, bucketWidth time.Duration, buckets []timeline.Bucket) {
	ui.Title(w, width, "timeline", sessionContext(path, total, bucketLabel(bucketWidth)+" buckets"))
	if total == 0 {
		writeNoTraffic(w, width)
		return
	}
	if len(buckets) == 0 {
		ui.Wrapped(w, width, 0, ui.Plain("None of the captured requests has a timestamp, so there is nothing to place on a timeline."))
		return
	}

	layout := "15:04:05"
	firstDay := buckets[0].Start.Local().Format("2006-01-02")
	for _, b := range buckets {
		if b.Start.Local().Format("2006-01-02") != firstDay {
			layout = "2006-01-02 15:04:05"
			break
		}
	}
	busiest, countWidth, errorWidth, avgWidth := 0, len("Requests"), len("Errors"), len("Avg")
	for _, b := range buckets {
		busiest = max(busiest, b.Requests)
		countWidth = max(countWidth, len(strconv.Itoa(b.Requests)))
		errorWidth = max(errorWidth, len(strconv.Itoa(b.Errors)))
		avgWidth = max(avgWidth, len(ui.Duration(b.Average)))
	}
	// The bar gets what the other columns and their gaps leave, up to 40.
	barWidth := max(len("Pulse"), min(40, width-len(layout)-countWidth-errorWidth-avgWidth-4*2))

	table := ui.NewTable(
		ui.Column{Header: "Start"},
		ui.Column{Header: "Requests", Align: ui.AlignRight},
		ui.Column{Header: "Errors", Align: ui.AlignRight},
		ui.Column{Header: "Avg", Align: ui.AlignRight},
		ui.Column{Header: "Pulse"},
	)
	for _, b := range buckets {
		cells := ui.BarLen(float64(b.Requests), float64(busiest), barWidth)
		errorCells := 0
		if b.Errors > 0 && b.Requests > 0 {
			errorCells = min(cells, max(1, int(math.Round(float64(cells*b.Errors)/float64(b.Requests)))))
		}
		bar := ui.Join(
			ui.Styled(strings.Repeat("█", cells-errorCells), ui.Accent),
			ui.Styled(strings.Repeat("█", errorCells), ui.Error),
		)
		errors := ui.Styled("0", ui.Muted)
		if b.Errors > 0 {
			errors = ui.Styled(strconv.Itoa(b.Errors), ui.Error)
		}
		table.Row(ui.Plain(b.Start.Local().Format(layout)), ui.Plain(strconv.Itoa(b.Requests)), errors, ui.Latency(b.Average), bar)
	}
	table.Render(w, width)
}
