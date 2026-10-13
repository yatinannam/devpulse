package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/yatinannam/devpulse/internal/output"
	"github.com/yatinannam/devpulse/internal/summary"
	"github.com/yatinannam/devpulse/internal/traffic"
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
	_ = fs.Parse(args)

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

	fmt.Println("DEVPULSE SUMMARY")
	fmt.Println("------------------------------------------------")
	fmt.Printf("Session: %s\n", *from)
	fmt.Printf("Requests: %d  Errors: %d  Error rate: %.1f%%\n", report.Total, report.Errors, report.ErrorRate*100)
	fmt.Printf("Average: %s  P50: %s  P95: %s  P99: %s\n",
		report.Average.Round(time.Millisecond),
		report.P50.Round(time.Millisecond),
		report.P95.Round(time.Millisecond),
		report.P99.Round(time.Millisecond))
	fmt.Println()
	fmt.Println("SLOWEST REQUESTS")
	if len(slowest) == 0 {
		fmt.Println("No requests captured.")
		return
	}
	for i, request := range slowest {
		fmt.Printf("%d. %-6s %-32s %3d  %s\n", i+1, request.Method, request.Path, request.Status, request.Latency.Round(time.Millisecond))
	}
}
